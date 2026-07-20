package database

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"unicode/utf8"
)

// RouteResolver resolves requests from an immutable, preloaded route snapshot.
// Reload builds a complete replacement before publishing it, so concurrent
// callers always observe either the old snapshot or the new one.
type RouteResolver struct {
	snapshot atomic.Pointer[routeSnapshot]
}

type routeSnapshot struct {
	exactDomains map[string]*cachedRoute
	patterns     []*cachedRoute
	paths        []*cachedRoute
}

type cachedRoute struct {
	id              int64
	routeType       string
	domain          string
	normalizedHost  string
	pathPrefix      string
	targetURL       string
	certificatePEM  string
	privateKeyPEM   string
	exactRouteKey   string
	patternRouteKey string
	pathRouteKey    string
	targets         []RouteTarget
	matcher         domainMatcher
}

type domainMatcher struct {
	wildcard string
	regex    *regexp.Regexp
}

// NewRouteResolver returns an empty resolver. Call Reload before serving
// requests; resolving against the empty snapshot returns sql.ErrNoRows.
func NewRouteResolver() *RouteResolver {
	resolver := &RouteResolver{}
	resolver.snapshot.Store(newRouteSnapshot())
	return resolver
}

// Reload reads all active routes and their ordered targets in two queries and
// atomically publishes the resulting immutable snapshot. If loading or
// validation fails, the last successfully loaded snapshot remains active.
func (r *RouteResolver) Reload(db *sql.DB) error {
	if r == nil {
		return fmt.Errorf("reload route resolver: nil resolver")
	}
	if db == nil {
		return fmt.Errorf("reload route resolver: nil database")
	}

	snapshot, err := loadRouteSnapshot(db)
	if err != nil {
		return err
	}
	r.snapshot.Store(snapshot)
	return nil
}

// Resolve returns the highest-priority route from the currently published
// snapshot. Returned target slices are independent copies and may be safely
// modified by the caller.
func (r *RouteResolver) Resolve(domain, path string) (*RouteMatch, error) {
	if r == nil {
		return nil, sql.ErrNoRows
	}

	snapshot := r.snapshot.Load()
	if snapshot == nil {
		return nil, sql.ErrNoRows
	}

	if domain != "" {
		domain = normalizeResolverDomain(domain)
		if route := snapshot.exactDomains[domain]; route != nil {
			if len(route.targets) > 0 {
				return route.domainMatch(route.exactRouteKey), nil
			}
		}

		for _, route := range snapshot.patterns {
			// The legacy resolver excludes an exact-equal pattern before
			// considering wildcard or regular-expression semantics.
			if strings.EqualFold(route.domain, domain) {
				continue
			}
			if !route.matcher.matches(domain) {
				continue
			}
			if len(route.targets) > 0 {
				return route.domainMatch(route.patternRouteKey), nil
			}
			// Resolution stops at the first matching pattern even when it has
			// no usable upstreams, matching GetRouteTargets' precedence.
			break
		}
	}

	if path != "" {
		for _, route := range snapshot.paths {
			if !strings.HasPrefix(path, route.pathPrefix) {
				continue
			}
			if len(route.targets) > 0 {
				return route.pathMatch(), nil
			}
			break
		}
	}

	return nil, sql.ErrNoRows
}

func loadRouteSnapshot(db *sql.DB) (*routeSnapshot, error) {
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("begin route snapshot: %w", err)
	}
	defer tx.Rollback()

	snapshot := newRouteSnapshot()
	routesByID := make(map[int64]*cachedRoute)
	routeOrder := make([]*cachedRoute, 0)

	rows, err := tx.Query(`
		SELECT id, route_type, COALESCE(domain, ''), COALESCE(path_prefix, ''),
		       target_url, COALESCE(certificate_pem, ''), COALESCE(private_key_pem, '')
		FROM routes
		WHERE active = 1 AND route_type IN ('domain', 'wildcard', 'regex', 'path')
		ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("load active routes: %w", err)
	}

	for rows.Next() {
		route := &cachedRoute{}
		if err := rows.Scan(
			&route.id,
			&route.routeType,
			&route.domain,
			&route.pathPrefix,
			&route.targetURL,
			&route.certificatePEM,
			&route.privateKeyPEM,
		); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan active route: %w", err)
		}

		route.routeType = strings.ToLower(strings.TrimSpace(route.routeType))
		if route.routeType != "path" {
			route.normalizedHost = normalizeResolverDomain(route.domain)
			route.exactRouteKey = "domain:" + route.normalizedHost
			route.patternRouteKey = "domain:" + route.domain
			matcher, err := compileDomainMatcher(route.routeType, route.domain)
			if err != nil {
				_ = rows.Close()
				return nil, fmt.Errorf("compile route %d domain %q: %w", route.id, route.domain, err)
			}
			route.matcher = matcher
		} else {
			route.pathRouteKey = "path:" + route.pathPrefix
		}

		routesByID[route.id] = route
		routeOrder = append(routeOrder, route)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterate active routes: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close active routes query: %w", err)
	}

	targetRows, err := tx.Query(`
		SELECT rt.route_id, rt.target_url, rt.health_check
		FROM route_targets AS rt
		JOIN routes AS r ON r.id = rt.route_id
		WHERE r.active = 1 AND r.route_type IN ('domain', 'wildcard', 'regex', 'path')
		ORDER BY rt.route_id ASC, rt.sort_order ASC, rt.id ASC`)
	if err != nil {
		return nil, fmt.Errorf("load active route targets: %w", err)
	}

	for targetRows.Next() {
		var (
			routeID     int64
			targetURL   string
			healthCheck string
		)
		if err := targetRows.Scan(&routeID, &targetURL, &healthCheck); err != nil {
			_ = targetRows.Close()
			return nil, fmt.Errorf("scan active route target: %w", err)
		}
		if healthCheck == "" {
			healthCheck = "http"
		}
		if route := routesByID[routeID]; route != nil {
			route.targets = append(route.targets, RouteTarget{URL: targetURL, HealthCheck: healthCheck})
		}
	}
	if err := targetRows.Err(); err != nil {
		_ = targetRows.Close()
		return nil, fmt.Errorf("iterate active route targets: %w", err)
	}
	if err := targetRows.Close(); err != nil {
		return nil, fmt.Errorf("close active route targets query: %w", err)
	}

	for _, route := range routeOrder {
		if len(route.targets) == 0 && route.targetURL != "" {
			route.targets = []RouteTarget{{URL: route.targetURL, HealthCheck: "http"}}
		}

		switch route.routeType {
		case "domain":
			if route.normalizedHost != "" {
				if _, exists := snapshot.exactDomains[route.normalizedHost]; !exists {
					snapshot.exactDomains[route.normalizedHost] = route
				}
			}
			if route.matcher.configured() {
				snapshot.patterns = append(snapshot.patterns, route)
			}
		case "wildcard", "regex":
			if route.domain != "" && route.matcher.configured() {
				snapshot.patterns = append(snapshot.patterns, route)
			}
		case "path":
			snapshot.paths = append(snapshot.paths, route)
		}
	}

	sort.SliceStable(snapshot.patterns, func(i, j int) bool {
		leftLength := utf8.RuneCountInString(snapshot.patterns[i].domain)
		rightLength := utf8.RuneCountInString(snapshot.patterns[j].domain)
		if leftLength != rightLength {
			return leftLength > rightLength
		}
		return snapshot.patterns[i].id < snapshot.patterns[j].id
	})
	sort.SliceStable(snapshot.paths, func(i, j int) bool {
		leftLength := utf8.RuneCountInString(snapshot.paths[i].pathPrefix)
		rightLength := utf8.RuneCountInString(snapshot.paths[j].pathPrefix)
		if leftLength != rightLength {
			return leftLength > rightLength
		}
		return snapshot.paths[i].id < snapshot.paths[j].id
	})

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit route snapshot: %w", err)
	}
	return snapshot, nil
}

func newRouteSnapshot() *routeSnapshot {
	return &routeSnapshot{exactDomains: make(map[string]*cachedRoute)}
}

// normalizeResolverDomain avoids constructing SplitHostPort errors for the
// overwhelmingly common already-unqualified hostname while retaining the
// established normalization behavior for ports and bracketed IPv6 hosts.
func normalizeResolverDomain(domain string) string {
	domain = strings.TrimSpace(domain)
	if strings.Contains(domain, ":") || strings.HasPrefix(domain, "[") {
		return normalizeDomain(domain)
	}
	return strings.ToLower(strings.TrimSuffix(domain, "."))
}

func compileDomainMatcher(routeType, domain string) (domainMatcher, error) {
	pattern := strings.ToLower(strings.TrimSpace(domain))

	switch {
	case routeType == "regex":
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return domainMatcher{}, err
		}
		return domainMatcher{regex: compiled}, nil
	case strings.HasPrefix(pattern, "regex:"):
		compiled, err := regexp.Compile(strings.TrimPrefix(pattern, "regex:"))
		if err != nil {
			return domainMatcher{}, err
		}
		return domainMatcher{regex: compiled}, nil
	case strings.HasPrefix(pattern, "~"):
		compiled, err := regexp.Compile(strings.TrimPrefix(pattern, "~"))
		if err != nil {
			return domainMatcher{}, err
		}
		return domainMatcher{regex: compiled}, nil
	case routeType == "wildcard" || strings.Contains(pattern, "*"):
		return domainMatcher{wildcard: pattern}, nil
	default:
		return domainMatcher{}, nil
	}
}

func (m domainMatcher) configured() bool {
	return m.regex != nil || m.wildcard != ""
}

func (m domainMatcher) matches(domain string) bool {
	if m.regex != nil {
		return m.regex.MatchString(domain)
	}
	return m.wildcard != "" && wildcardDomainMatch(m.wildcard, domain)
}

func (r *cachedRoute) domainMatch(routeKey string) *RouteMatch {
	return &RouteMatch{
		RouteKey:       routeKey,
		Targets:        cloneRouteTargets(r.targets),
		CertificatePEM: r.certificatePEM,
		PrivateKeyPEM:  r.privateKeyPEM,
	}
}

func (r *cachedRoute) pathMatch() *RouteMatch {
	return &RouteMatch{
		RouteKey: r.pathRouteKey,
		Targets:  cloneRouteTargets(r.targets),
	}
}

func cloneRouteTargets(targets []RouteTarget) []RouteTarget {
	cloned := make([]RouteTarget, len(targets))
	copy(cloned, targets)
	return cloned
}
