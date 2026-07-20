package database

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestRouteResolverMatchesDatabaseResolution(t *testing.T) {
	db := newResolverTestDB(t)

	insertResolverRoute(t, db, resolverRouteSpec{
		routeType: "wildcard",
		domain:    "*.example.test",
		targets:   []RouteTarget{{URL: "http://wildcard", HealthCheck: "tcp"}},
	})
	insertResolverRoute(t, db, resolverRouteSpec{
		routeType: "wildcard",
		domain:    "*.deep.example.test",
		targets:   []RouteTarget{{URL: "http://deep-wildcard", HealthCheck: "http"}},
	})
	insertResolverRoute(t, db, resolverRouteSpec{
		routeType: "regex",
		domain:    `^service-[0-9]+\.internal\.test$`,
		targets:   []RouteTarget{{URL: "http://regex", HealthCheck: "tcp"}},
		cert:      "regex-cert",
		key:       "regex-key",
	})
	insertResolverRoute(t, db, resolverRouteSpec{
		routeType: "domain",
		domain:    "api.example.test",
		targets: []RouteTarget{
			{URL: "http://exact-primary", HealthCheck: "http"},
			{URL: "http://exact-secondary", HealthCheck: "tcp"},
		},
		cert: "exact-cert",
		key:  "exact-key",
	})
	insertResolverRoute(t, db, resolverRouteSpec{
		routeType: "domain",
		domain:    "fallback.example.test",
		fallback:  "http://fallback",
	})
	insertResolverRoute(t, db, resolverRouteSpec{
		routeType:  "path",
		pathPrefix: "/api/",
		targets:    []RouteTarget{{URL: "http://api", HealthCheck: "http"}},
	})
	insertResolverRoute(t, db, resolverRouteSpec{
		routeType:  "path",
		pathPrefix: "/api/admin/",
		targets:    []RouteTarget{{URL: "http://admin", HealthCheck: "tcp"}},
	})

	resolver := NewRouteResolver()
	if err := resolver.Reload(db); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	tests := []struct {
		name   string
		domain string
		path   string
	}{
		{name: "exact wins over wildcard", domain: "API.EXAMPLE.TEST:443", path: "/api/admin/users"},
		{name: "longest wildcard", domain: "node.deep.example.test", path: "/"},
		{name: "general wildcard", domain: "www.example.test.", path: "/"},
		{name: "precompiled regex", domain: "service-42.internal.test", path: "/"},
		{name: "fallback target", domain: "fallback.example.test", path: "/"},
		{name: "longest path", path: "/api/admin/users"},
		{name: "general path", path: "/api/users"},
		{name: "not found", domain: "missing.test", path: "/missing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, wantErr := GetRouteTargets(db, tt.domain, tt.path)
			got, gotErr := resolver.Resolve(tt.domain, tt.path)

			if !sameResolutionError(gotErr, wantErr) {
				t.Fatalf("Resolve error = %v, GetRouteTargets error = %v", gotErr, wantErr)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("Resolve = %#v, GetRouteTargets = %#v", got, want)
			}
		})
	}
}

func TestRouteResolverReloadPublishesAdditionsAndRemovals(t *testing.T) {
	db := newResolverTestDB(t)
	oldRouteID := insertResolverRoute(t, db, resolverRouteSpec{
		routeType: "domain",
		domain:    "old.example.test",
		targets:   []RouteTarget{{URL: "http://old", HealthCheck: "http"}},
	})

	resolver := NewRouteResolver()
	if _, err := resolver.Resolve("old.example.test", "/"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Resolve before Reload error = %v, want sql.ErrNoRows", err)
	}
	if err := resolver.Reload(db); err != nil {
		t.Fatalf("initial Reload failed: %v", err)
	}
	assertResolvedTarget(t, resolver, "old.example.test", "/", "http://old")

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin route update: %v", err)
	}
	if _, err := tx.Exec(`DELETE FROM routes WHERE id = ?`, oldRouteID); err != nil {
		_ = tx.Rollback()
		t.Fatalf("delete old route: %v", err)
	}
	result, err := tx.Exec(`
		INSERT INTO routes (route_type, domain, path_prefix, target_url, active)
		VALUES ('domain', 'new.example.test', '', 'http://new-fallback', 1)`)
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("insert new route: %v", err)
	}
	newRouteID, err := result.LastInsertId()
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("get new route ID: %v", err)
	}
	if err := SetRouteTargetsTx(tx, int(newRouteID), []RouteTarget{{URL: "http://new", HealthCheck: "tcp"}}); err != nil {
		_ = tx.Rollback()
		t.Fatalf("set new route targets: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit route update: %v", err)
	}

	// The published snapshot remains stable until a successful reload.
	assertResolvedTarget(t, resolver, "old.example.test", "/", "http://old")
	if _, err := resolver.Resolve("new.example.test", "/"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("new route before Reload error = %v, want sql.ErrNoRows", err)
	}

	if err := resolver.Reload(db); err != nil {
		t.Fatalf("second Reload failed: %v", err)
	}
	if _, err := resolver.Resolve("old.example.test", "/"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("removed route error = %v, want sql.ErrNoRows", err)
	}
	assertResolvedTarget(t, resolver, "new.example.test", "/", "http://new")
}

func TestRouteResolverRejectsInvalidRegexAtomically(t *testing.T) {
	db := newResolverTestDB(t)
	goodRouteID := insertResolverRoute(t, db, resolverRouteSpec{
		routeType: "domain",
		domain:    "stable.example.test",
		targets:   []RouteTarget{{URL: "http://stable", HealthCheck: "http"}},
	})

	resolver := NewRouteResolver()
	if err := resolver.Reload(db); err != nil {
		t.Fatalf("initial Reload failed: %v", err)
	}

	if _, err := db.Exec(`DELETE FROM routes WHERE id = ?`, goodRouteID); err != nil {
		t.Fatalf("delete stable route from database: %v", err)
	}
	result, err := db.Exec(`
		INSERT INTO routes (route_type, domain, path_prefix, target_url, active)
		VALUES ('regex', '[unterminated', '', 'http://invalid', 1)`)
	if err != nil {
		t.Fatalf("insert invalid regex route: %v", err)
	}
	invalidRouteID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("get invalid route ID: %v", err)
	}

	err = resolver.Reload(db)
	if err == nil {
		t.Fatal("Reload accepted an invalid regular expression")
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("route %d", invalidRouteID)) ||
		!strings.Contains(err.Error(), "[unterminated") {
		t.Fatalf("Reload error = %q, want route ID and invalid pattern", err)
	}

	// The good route no longer exists in SQLite, so resolving it proves the
	// failed reload did not replace the last valid in-memory snapshot.
	assertResolvedTarget(t, resolver, "stable.example.test", "/", "http://stable")
	if _, err := resolver.Resolve("anything.example.test", "/"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("invalid route became visible: %v", err)
	}
	if err := resolver.Reload(nil); err == nil {
		t.Fatal("Reload(nil) succeeded")
	}
	assertResolvedTarget(t, resolver, "stable.example.test", "/", "http://stable")
}

func TestRouteResolverReturnsIndependentTargets(t *testing.T) {
	db := newResolverTestDB(t)
	insertResolverRoute(t, db, resolverRouteSpec{
		routeType: "domain",
		domain:    "copy.example.test",
		targets: []RouteTarget{
			{URL: "http://primary", HealthCheck: "http"},
			{URL: "http://secondary", HealthCheck: "tcp"},
		},
	})

	resolver := NewRouteResolver()
	if err := resolver.Reload(db); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	first, err := resolver.Resolve("copy.example.test", "/")
	if err != nil {
		t.Fatalf("first Resolve failed: %v", err)
	}
	first.Targets[0].URL = "http://mutated"
	first.Targets = append(first.Targets, RouteTarget{URL: "http://injected"})

	second, err := resolver.Resolve("copy.example.test", "/")
	if err != nil {
		t.Fatalf("second Resolve failed: %v", err)
	}
	want := []RouteTarget{
		{URL: "http://primary", HealthCheck: "http"},
		{URL: "http://secondary", HealthCheck: "tcp"},
	}
	if !reflect.DeepEqual(second.Targets, want) {
		t.Fatalf("second Resolve targets = %#v, want %#v", second.Targets, want)
	}
}

func TestRouteResolverConcurrentResolveAndReload(t *testing.T) {
	db := newResolverTestDB(t)
	routeID := insertResolverRoute(t, db, resolverRouteSpec{
		routeType: "domain",
		domain:    "concurrent.example.test",
		targets:   concurrentTargets("old"),
	})

	resolver := NewRouteResolver()
	if err := resolver.Reload(db); err != nil {
		t.Fatalf("initial Reload failed: %v", err)
	}

	const (
		readerCount = 8
		readerLoops = 2_000
		reloadLoops = 100
	)
	start := make(chan struct{})
	errorsFound := make(chan error, 1)
	var failed atomic.Bool
	var readers sync.WaitGroup

	for range readerCount {
		readers.Add(1)
		go func() {
			defer readers.Done()
			<-start
			for range readerLoops {
				if failed.Load() {
					return
				}
				match, err := resolver.Resolve("CONCURRENT.EXAMPLE.TEST:443", "/")
				if err != nil {
					recordResolverError(&failed, errorsFound, fmt.Errorf("Resolve failed: %w", err))
					return
				}
				if !isConcurrentTargetSet(match.Targets, "old") && !isConcurrentTargetSet(match.Targets, "new") {
					recordResolverError(&failed, errorsFound, fmt.Errorf("observed mixed snapshot: %#v", match.Targets))
					return
				}
			}
		}()
	}

	close(start)
	for i := range reloadLoops {
		version := "old"
		if i%2 == 0 {
			version = "new"
		}
		if err := SetRouteTargets(db, int(routeID), concurrentTargets(version)); err != nil {
			t.Fatalf("SetRouteTargets iteration %d: %v", i, err)
		}
		if err := resolver.Reload(db); err != nil {
			t.Fatalf("Reload iteration %d: %v", i, err)
		}
	}
	readers.Wait()
	close(errorsFound)
	if err := <-errorsFound; err != nil {
		t.Fatal(err)
	}
}

func TestRouteResolverResolveAllocations(t *testing.T) {
	resolver := NewRouteResolver()
	resolver.snapshot.Store(&routeSnapshot{
		exactDomains: map[string]*cachedRoute{
			"alloc.example.test": {
				targets: []RouteTarget{
					{URL: "http://primary", HealthCheck: "http"},
					{URL: "http://secondary", HealthCheck: "tcp"},
				},
			},
		},
	})

	var resolveErr error
	allocations := testing.AllocsPerRun(1_000, func() {
		var match *RouteMatch
		match, resolveErr = resolver.Resolve("alloc.example.test", "/")
		if match == nil || len(match.Targets) != 2 {
			panic("unexpected route resolution")
		}
	})
	if resolveErr != nil {
		t.Fatalf("Resolve failed: %v", resolveErr)
	}
	if allocations > 2 {
		t.Fatalf("Resolve allocations = %.2f, want at most 2", allocations)
	}
}

func BenchmarkRouteResolverResolve(b *testing.B) {
	b.Setenv(bootstrapUsernameEnv, "")
	b.Setenv(bootstrapPasswordEnv, "")
	db, err := Init(":memory:")
	if err != nil {
		b.Fatalf("Init failed: %v", err)
	}
	b.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`DELETE FROM routes`); err != nil {
		b.Fatalf("clear routes: %v", err)
	}

	exactID := insertBenchmarkRoute(b, db, "domain", "api.example.test", "http://exact")
	if err := SetRouteTargets(db, exactID, []RouteTarget{
		{URL: "http://exact-primary", HealthCheck: "http"},
		{URL: "http://exact-secondary", HealthCheck: "tcp"},
	}); err != nil {
		b.Fatalf("set exact targets: %v", err)
	}
	insertBenchmarkRoute(b, db, "regex", `^service-[0-9]+\.internal\.test$`, "http://regex")

	resolver := NewRouteResolver()
	if err := resolver.Reload(db); err != nil {
		b.Fatalf("Reload failed: %v", err)
	}

	benchmarks := []struct {
		name   string
		domain string
	}{
		{name: "exact", domain: "api.example.test"},
		{name: "precompiled_regex", domain: "service-42.internal.test"},
	}
	for _, benchmark := range benchmarks {
		b.Run(benchmark.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				match, err := resolver.Resolve(benchmark.domain, "/")
				if err != nil {
					b.Fatal(err)
				}
				benchmarkRouteMatch = match
			}
		})
	}
}

type resolverRouteSpec struct {
	routeType  string
	domain     string
	pathPrefix string
	fallback   string
	targets    []RouteTarget
	cert       string
	key        string
}

func newResolverTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db := newTestDB(t)
	if _, err := db.Exec(`DELETE FROM routes`); err != nil {
		t.Fatalf("clear seeded routes: %v", err)
	}
	return db
}

func insertResolverRoute(t *testing.T, db *sql.DB, route resolverRouteSpec) int64 {
	t.Helper()
	result, err := db.Exec(`
		INSERT INTO routes (
			route_type, domain, path_prefix, target_url,
			certificate_pem, private_key_pem, active
		) VALUES (?, ?, ?, ?, ?, ?, 1)`,
		route.routeType,
		route.domain,
		route.pathPrefix,
		route.fallback,
		route.cert,
		route.key,
	)
	if err != nil {
		t.Fatalf("insert %s route %q: %v", route.routeType, route.domain+route.pathPrefix, err)
	}
	routeID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("get route ID: %v", err)
	}
	if len(route.targets) > 0 {
		if err := SetRouteTargets(db, int(routeID), route.targets); err != nil {
			t.Fatalf("set targets for route %d: %v", routeID, err)
		}
	}
	return routeID
}

func sameResolutionError(left, right error) bool {
	return errors.Is(left, sql.ErrNoRows) == errors.Is(right, sql.ErrNoRows) &&
		(left == nil) == (right == nil)
}

func assertResolvedTarget(t *testing.T, resolver *RouteResolver, domain, path, want string) {
	t.Helper()
	match, err := resolver.Resolve(domain, path)
	if err != nil {
		t.Fatalf("Resolve(%q, %q) failed: %v", domain, path, err)
	}
	if len(match.Targets) != 1 || match.Targets[0].URL != want {
		t.Fatalf("Resolve(%q, %q) targets = %#v, want %q", domain, path, match.Targets, want)
	}
}

func concurrentTargets(version string) []RouteTarget {
	return []RouteTarget{
		{URL: "http://" + version + "-primary", HealthCheck: "http"},
		{URL: "http://" + version + "-secondary", HealthCheck: "tcp"},
	}
}

func isConcurrentTargetSet(targets []RouteTarget, version string) bool {
	return len(targets) == 2 &&
		targets[0] == (RouteTarget{URL: "http://" + version + "-primary", HealthCheck: "http"}) &&
		targets[1] == (RouteTarget{URL: "http://" + version + "-secondary", HealthCheck: "tcp"})
}

func recordResolverError(failed *atomic.Bool, destination chan<- error, err error) {
	if !failed.CompareAndSwap(false, true) {
		return
	}
	destination <- err
}

func insertBenchmarkRoute(b *testing.B, db *sql.DB, routeType, domain, target string) int {
	b.Helper()
	result, err := db.Exec(`
		INSERT INTO routes (route_type, domain, path_prefix, target_url, active)
		VALUES (?, ?, '', ?, 1)`, routeType, domain, target)
	if err != nil {
		b.Fatalf("insert benchmark route: %v", err)
	}
	routeID, err := result.LastInsertId()
	if err != nil {
		b.Fatalf("get benchmark route ID: %v", err)
	}
	if err := SetRouteTargets(db, int(routeID), []RouteTarget{{URL: target, HealthCheck: "http"}}); err != nil {
		b.Fatalf("set benchmark route target: %v", err)
	}
	return int(routeID)
}

var benchmarkRouteMatch *RouteMatch
