package database

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

const (
	sqliteBusyTimeout = 5 * time.Second
	fileMaxOpenConns  = 8
	fileMaxIdleConns  = 4
)

func Init(path string) (*sql.DB, error) {
	dsn, inMemory, err := sqliteDSN(path)
	if err != nil {
		return nil, fmt.Errorf("configure sqlite database: %w", err)
	}

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	configureConnectionPool(db, inMemory)

	ctx, cancel := context.WithTimeout(context.Background(), sqliteBusyTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect sqlite database: %w", err)
	}

	if err := createTables(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize sqlite database: %w", err)
	}

	if err := createTables(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	if !isMemoryPath(path) {
		if err := validateIntegrity(db); err != nil {
			_ = db.Close()
			return nil, err
		}
	}

	return db, nil
}

func sqliteDSN(path string) (string, bool, error) {
	filename, rawQuery, hasQuery := strings.Cut(path, "?")
	params := make(url.Values)
	if hasQuery {
		var err error
		params, err = url.ParseQuery(rawQuery)
		if err != nil {
			return "", false, fmt.Errorf("parse sqlite DSN options: %w", err)
		}
	}

	inMemory := filename == ":memory:" || filename == "file::memory:" ||
		strings.EqualFold(params.Get("mode"), "memory")
	// go-sqlite3 accepts aliases for these options and gives some aliases
	// precedence. Remove them so callers cannot accidentally defeat the
	// connection-wide policy with a conflicting duplicate.
	params.Del("_timeout")
	params.Del("_fk")
	params.Set("_busy_timeout", strconv.FormatInt(sqliteBusyTimeout.Milliseconds(), 10))
	params.Set("_foreign_keys", "on")
	if !inMemory {
		params.Del("_journal")
		params.Del("_sync")
		params.Set("_journal_mode", "WAL")
		params.Set("_synchronous", "NORMAL")
	} else {
		params.Del("_journal")
		params.Del("_journal_mode")
		params.Del("_sync")
		params.Del("_synchronous")
	}

	return filename + "?" + params.Encode(), inMemory, nil
}

func configureConnectionPool(db *sql.DB, inMemory bool) {
	if inMemory {
		// Private in-memory databases are scoped to a single SQLite connection.
		// Keeping that connection idle also prevents the database from vanishing.
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
		return
	}

	// WAL permits concurrent readers but SQLite still serializes writers. A
	// modest bound retains read concurrency without creating excessive lock
	// contention or an unbounded number of file descriptors under load.
	db.SetMaxOpenConns(fileMaxOpenConns)
	db.SetMaxIdleConns(fileMaxIdleConns)
}

func createTables(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS routes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		route_type TEXT NOT NULL DEFAULT 'domain',
		domain TEXT,
		path_prefix TEXT,
		target_url TEXT NOT NULL,
		certificate_pem TEXT,
		private_key_pem TEXT,
		active INTEGER DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(route_type, domain, path_prefix)
	);`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS waf_rules (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		expression TEXT NOT NULL,
		action TEXT NOT NULL DEFAULT 'BLOCK',
		priority INTEGER DEFAULT 0,
		UNIQUE(name)
	);`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		email TEXT,
		zero_trust_enabled INTEGER DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS user_proxy_records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		domain TEXT NOT NULL,
		target_url TEXT NOT NULL,
		certificate_pem TEXT,
		private_key_pem TEXT,
		active INTEGER DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id),
		UNIQUE(user_id, domain)
	);`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS zero_trust_settings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		key TEXT NOT NULL UNIQUE,
		value TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS user_sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		token TEXT NOT NULL UNIQUE,
		expires_at DATETIME NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id)
	);`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_user_sessions_expires_at ON user_sessions(expires_at);`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS route_targets (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		route_id INTEGER NOT NULL,
		target_url TEXT NOT NULL,
		health_check TEXT NOT NULL DEFAULT 'http',
		sort_order INTEGER NOT NULL DEFAULT 0,
		FOREIGN KEY (route_id) REFERENCES routes(id) ON DELETE CASCADE,
		UNIQUE(route_id, target_url)
	);`)
	if err != nil {
		return err
	}

	if err := seedDefaults(db); err != nil {
		return err
	}

	if err := migrateRouteNulls(db); err != nil {
		return err
	}

	if err := migrateRouteTargets(db); err != nil {
		return err
	}
	_, err = PruneExpiredSessions(db)
	return err
}

func migrateRouteNulls(db *sql.DB) error {
	if _, err := db.Exec(`UPDATE routes SET domain = '' WHERE domain IS NULL`); err != nil {
		return err
	}
	_, err := db.Exec(`UPDATE routes SET path_prefix = '' WHERE path_prefix IS NULL`)
	return err
}

func migrateRouteTargets(db *sql.DB) error {
	_, err := db.Exec(`
		INSERT OR IGNORE INTO route_targets (route_id, target_url, health_check, sort_order)
		SELECT id, target_url, 'http', 0 FROM routes WHERE target_url != ''`)
	return err
}

func seedDefaults(db *sql.DB) error {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM routes").Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		_, err = db.Exec(`INSERT INTO routes (route_type, domain, target_url, active) VALUES (?, ?, ?, ?)`, "domain", "example.com", "http://example.com:8000", 1)
		if err != nil {
			return err
		}
		log.Info().Str("domain", "example.com").Str("target", "http://example.com:8000").Msg("Inserted default domain route")
	}

	err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		if err := seedBootstrapUser(db); err != nil {
			return err
		}
	}

	err = db.QueryRow("SELECT COUNT(*) FROM waf_rules").Scan(&count)
	if err == nil && count == 0 {
		rules := []struct {
			Name       string
			Expression string
			Priority   int
		}{
			{"Block Admin", `Path startsWith "/admin"`, 10},
			{"Block SQL Injection (Path)", `Path matches ".*(?i)(union\\s+select|waitfor\\s+delay|1=1|--|;).*$"`, 20},
			{"Block SQL Injection (Query)", `RawQuery matches "(?i)(union\\s+select|waitfor\\s+delay|1=1|--|;)"`, 20},
			{"Block XSS (Path)", `Path matches "(?i)(<script>|javascript:|onerror=)"`, 20},
			{"Block XSS (Query)", `RawQuery matches "(?i)(<script>|javascript:|onerror=)"`, 20},
			{"Block Path Traversal", `Path matches "(?:\\.\\./|\\.\\.\\\\)"`, 20},
			{"Block Path Traversal (Path Encoded)", `Path matches ".*(?i)(%2e%2e%2f|%2e%2e%5c).*$"`, 20},
			{"Block Path Traversal (Query)", `RawQuery matches "(?:\\.\\./|\\.\\.\\\\)"`, 20},
			{"Block Path Traversal (Query Encoded)", `RawQuery matches ".*(?i)(%2e%2e%2f|%2e%2e%5c).*$"`, 20},
			{"Block SSRF Metadata & Localhost", `RawQuery matches "(?i)(169\\.254\\.169\\.254|127\\.0\\.0\\.1|localhost)"`, 20},
		}

		for _, rule := range rules {
			_, err = db.Exec(`INSERT INTO waf_rules (name, expression, action, priority) VALUES (?, ?, ?, ?)`,
				rule.Name, rule.Expression, "BLOCK", rule.Priority)
			if err != nil {
				log.Error().Err(err).Str("rule", rule.Name).Msg("Failed to insert WAF rule")
			} else {
				log.Info().Str("rule", rule.Name).Msg("Inserted WAF rule")
			}
		}
	}
	return nil
}

const (
	bootstrapUsernameEnv = "NETGOAT_BOOTSTRAP_USERNAME"
	bootstrapPasswordEnv = "NETGOAT_BOOTSTRAP_PASSWORD"
	minBootstrapPassword = 12
)

func seedBootstrapUser(db *sql.DB) error {
	username := strings.TrimSpace(os.Getenv(bootstrapUsernameEnv))
	password := os.Getenv(bootstrapPasswordEnv)
	if username == "" && password == "" {
		log.Info().Str("username_env", bootstrapUsernameEnv).Str("password_env", bootstrapPasswordEnv).
			Msg("No users configured; bootstrap credentials were not provided")
		return nil
	}
	if username == "" || password == "" {
		return fmt.Errorf("both %s and %s must be set to bootstrap a user", bootstrapUsernameEnv, bootstrapPasswordEnv)
	}
	if len(password) < minBootstrapPassword {
		return fmt.Errorf("%s must contain at least %d characters", bootstrapPasswordEnv, minBootstrapPassword)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash bootstrap password: %w", err)
	}
	if _, err := db.Exec(`INSERT INTO users (username, password_hash) VALUES (?, ?)`, username, string(hash)); err != nil {
		return fmt.Errorf("insert bootstrap user: %w", err)
	}
	log.Info().Str("username", username).Msg("Inserted bootstrap user from environment")
	return nil
}

// PruneExpiredSessions removes authentication sessions that can no longer be
// used. It is safe to call at startup and before creating a new session.
func PruneExpiredSessions(db *sql.DB) (int64, error) {
	result, err := db.Exec(`DELETE FROM user_sessions WHERE expires_at <= CURRENT_TIMESTAMP`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// RouteTarget is a single upstream for a route.
type RouteTarget struct {
	URL         string
	HealthCheck string
}

// RouteMatch is the resolved route with all upstream targets.
type RouteMatch struct {
	RouteKey       string
	Targets        []RouteTarget
	CertificatePEM string
	PrivateKeyPEM  string
}

func loadRouteTargets(db *sql.DB, routeID int) ([]RouteTarget, error) {
	rows, err := db.Query(`
		SELECT target_url, health_check FROM route_targets
		WHERE route_id = ?
		ORDER BY sort_order ASC, id ASC`, routeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []RouteTarget
	for rows.Next() {
		var url, check string
		if err := rows.Scan(&url, &check); err != nil {
			return nil, err
		}
		if check == "" {
			check = "http"
		}
		targets = append(targets, RouteTarget{URL: url, HealthCheck: check})
	}
	return targets, rows.Err()
}

func findRouteByDomain(db *sql.DB, domain string) (int, string, string, string, error) {
	var routeID int
	var targetURL, certPem, keyPem string
	err := db.QueryRow(`
		SELECT id, target_url, COALESCE(certificate_pem, ''), COALESCE(private_key_pem, '')
		FROM routes
		WHERE route_type = 'domain' AND domain = ? COLLATE NOCASE AND active = 1
		LIMIT 1`, domain).Scan(&routeID, &targetURL, &certPem, &keyPem)
	return routeID, targetURL, certPem, keyPem, err
}

func findPatternRouteByDomain(db *sql.DB, domain string) (int, string, string, string, string, error) {
	rows, err := db.Query(`
		SELECT id, route_type, domain, target_url, COALESCE(certificate_pem, ''), COALESCE(private_key_pem, '')
		FROM routes
		WHERE active = 1 AND route_type IN ('domain', 'wildcard', 'regex')
		ORDER BY LENGTH(domain) DESC, id ASC`)
	if err != nil {
		return 0, "", "", "", "", err
	}
	defer rows.Close()

	domain = strings.ToLower(strings.TrimSpace(domain))
	for rows.Next() {
		var routeID int
		var routeType, pattern, targetURL, certPem, keyPem string
		if err := rows.Scan(&routeID, &routeType, &pattern, &targetURL, &certPem, &keyPem); err != nil {
			return 0, "", "", "", "", err
		}
		if pattern == "" || strings.EqualFold(pattern, domain) {
			continue
		}
		if domainPatternMatches(routeType, pattern, domain) {
			return routeID, targetURL, certPem, keyPem, pattern, nil
		}
	}
	if err := rows.Err(); err != nil {
		return 0, "", "", "", "", err
	}
	return 0, "", "", "", "", sql.ErrNoRows
}

func domainPatternMatches(routeType, pattern, domain string) bool {
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	routeType = strings.ToLower(strings.TrimSpace(routeType))

	switch {
	case routeType == "regex":
		return regexDomainMatch(pattern, domain)
	case strings.HasPrefix(pattern, "regex:"):
		return regexDomainMatch(strings.TrimPrefix(pattern, "regex:"), domain)
	case strings.HasPrefix(pattern, "~"):
		return regexDomainMatch(strings.TrimPrefix(pattern, "~"), domain)
	case routeType == "wildcard" || strings.Contains(pattern, "*"):
		return wildcardDomainMatch(pattern, domain)
	default:
		return false
	}
}

func regexDomainMatch(pattern, domain string) bool {
	re, err := regexp.Compile(pattern)
	return err == nil && re.MatchString(domain)
}

func wildcardDomainMatch(pattern, domain string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == domain
	}

	if parts[0] != "" && !strings.HasPrefix(domain, parts[0]) {
		return false
	}
	if last := parts[len(parts)-1]; last != "" && !strings.HasSuffix(domain, last) {
		return false
	}

	pos := len(parts[0])
	for _, part := range parts[1 : len(parts)-1] {
		if part == "" {
			continue
		}
		idx := strings.Index(domain[pos:], part)
		if idx < 0 {
			return false
		}
		pos += idx + len(part)
	}
	return true
}

func findRouteByPath(db *sql.DB, path string) (int, string, string, error) {
	var routeID int
	var targetURL, pathPrefix string
	err := db.QueryRow(`
		SELECT id, target_url, path_prefix FROM routes
		WHERE route_type = 'path' AND ? LIKE path_prefix || '%' AND active = 1
		ORDER BY LENGTH(path_prefix) DESC
		LIMIT 1`, path).Scan(&routeID, &targetURL, &pathPrefix)
	return routeID, targetURL, pathPrefix, err
}

// GetRouteTargets resolves a route and returns all configured upstream targets.
func GetRouteTargets(db *sql.DB, domain, path string) (*RouteMatch, error) {
	if domain != "" {
		domain = normalizeDomain(domain)
		routeID, fallbackURL, certPem, keyPem, err := findRouteByDomain(db, domain)
		if err == nil {
			targets, err := loadRouteTargets(db, routeID)
			if err != nil {
				return nil, err
			}
			if len(targets) == 0 && fallbackURL != "" {
				targets = []RouteTarget{{URL: fallbackURL, HealthCheck: "http"}}
			}
			if len(targets) > 0 {
				return &RouteMatch{
					RouteKey:       "domain:" + domain,
					Targets:        targets,
					CertificatePEM: certPem,
					PrivateKeyPEM:  keyPem,
				}, nil
			}
		} else if err != sql.ErrNoRows {
			log.Error().Err(err).Str("domain", domain).Msg("Error querying route by domain")
		}

		routeID, fallbackURL, certPem, keyPem, pattern, err := findPatternRouteByDomain(db, domain)
		if err == nil {
			targets, err := loadRouteTargets(db, routeID)
			if err != nil {
				return nil, err
			}
			if len(targets) == 0 && fallbackURL != "" {
				targets = []RouteTarget{{URL: fallbackURL, HealthCheck: "http"}}
			}
			if len(targets) > 0 {
				return &RouteMatch{
					RouteKey:       "domain:" + pattern,
					Targets:        targets,
					CertificatePEM: certPem,
					PrivateKeyPEM:  keyPem,
				}, nil
			}
		} else if err != sql.ErrNoRows {
			log.Error().Err(err).Str("domain", domain).Msg("Error querying pattern route by domain")
		}
	}

	if path != "" {
		routeID, fallbackURL, pathPrefix, err := findRouteByPath(db, path)
		if err == nil {
			targets, err := loadRouteTargets(db, routeID)
			if err != nil {
				return nil, err
			}
			if len(targets) == 0 && fallbackURL != "" {
				targets = []RouteTarget{{URL: fallbackURL, HealthCheck: "http"}}
			}
			if len(targets) > 0 {
				return &RouteMatch{
					RouteKey: "path:" + pathPrefix,
					Targets:  targets,
				}, nil
			}
		} else if err != sql.ErrNoRows {
			log.Error().Err(err).Str("path", path).Msg("Error querying route by path")
		}
	}

	log.Warn().Str("domain", domain).Str("path", path).Msg("No route found for domain or path")
	return nil, sql.ErrNoRows
}

func normalizeDomain(domain string) string {
	domain = strings.TrimSpace(domain)
	if host, _, err := net.SplitHostPort(domain); err == nil {
		domain = host
	} else {
		domain = strings.TrimPrefix(strings.TrimSuffix(domain, "]"), "[")
	}
	return strings.ToLower(strings.TrimSuffix(domain, "."))
}

// SetRouteTargets replaces upstream targets for a route.
func SetRouteTargets(db *sql.DB, routeID int, targets []RouteTarget) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := setRouteTargets(tx, routeID, targets); err != nil {
		return err
	}

	return tx.Commit()
}

// SetRouteTargetsTx replaces upstream targets inside an existing transaction.
func SetRouteTargetsTx(tx *sql.Tx, routeID int, targets []RouteTarget) error {
	return setRouteTargets(tx, routeID, targets)
}

type routeTargetExecer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func setRouteTargets(exec routeTargetExecer, routeID int, targets []RouteTarget) error {
	if _, err := exec.Exec(`DELETE FROM route_targets WHERE route_id = ?`, routeID); err != nil {
		return err
	}

	for i, t := range targets {
		if t.URL == "" {
			continue
		}
		check := strings.ToLower(strings.TrimSpace(t.HealthCheck))
		if check == "" {
			check = "http"
		}
		if _, err := exec.Exec(
			`INSERT INTO route_targets (route_id, target_url, health_check, sort_order) VALUES (?, ?, ?, ?)`,
			routeID, t.URL, check, i); err != nil {
			return err
		}
	}
	return nil
}

// ListAllRouteTargets returns every active upstream for health monitoring.
func ListAllRouteTargets(db *sql.DB) ([]RouteTarget, error) {
	rows, err := db.Query(`
		SELECT rt.target_url, rt.health_check
		FROM route_targets rt
		INNER JOIN routes r ON r.id = rt.route_id
		WHERE r.active = 1
		ORDER BY rt.id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seen := make(map[string]struct{})
	var targets []RouteTarget
	for rows.Next() {
		var url, check string
		if err := rows.Scan(&url, &check); err != nil {
			return nil, err
		}
		if _, ok := seen[url]; ok {
			continue
		}
		seen[url] = struct{}{}
		if check == "" {
			check = "http"
		}
		targets = append(targets, RouteTarget{URL: url, HealthCheck: check})
	}
	return targets, rows.Err()
}

// GetTargetByDomain returns the primary target URL and certificate for a domain.
func GetTargetByDomain(db *sql.DB, domain string) (string, string, string, error) {
	match, err := GetRouteTargets(db, domain, "")
	if err != nil {
		return "", "", "", err
	}
	if len(match.Targets) == 0 {
		return "", "", "", sql.ErrNoRows
	}
	log.Debug().Str("domain", domain).Str("target_url", match.Targets[0].URL).Msg("Found domain route")
	return match.Targets[0].URL, match.CertificatePEM, match.PrivateKeyPEM, nil
}

// GetTargetByPath returns the primary target URL for a path-based route.
func GetTargetByPath(db *sql.DB, path string) (string, error) {
	match, err := GetRouteTargets(db, "", path)
	if err != nil {
		return "", err
	}
	if len(match.Targets) == 0 {
		return "", sql.ErrNoRows
	}
	log.Debug().Str("path", path).Str("target_url", match.Targets[0].URL).Msg("Found path route")
	return match.Targets[0].URL, nil
}

// GetTarget returns the primary upstream URL for backward compatibility.
func GetTarget(db *sql.DB, domain, path string) (string, error) {
	match, err := GetRouteTargets(db, domain, path)
	if err != nil {
		return "", err
	}
	if len(match.Targets) == 0 {
		return "", sql.ErrNoRows
	}
	return match.Targets[0].URL, nil
}

// GetUserID returns the user ID for a given username
func GetUserID(db *sql.DB, username string) (int, error) {
	var id int
	err := db.QueryRow("SELECT id FROM users WHERE username = ?", username).Scan(&id)
	return id, err
}

// GetUserProxyRecords returns all proxy records (domains) for a user
func GetUserProxyRecords(db *sql.DB, userID int) ([]map[string]interface{}, error) {
	rows, err := db.Query(`
		SELECT id, domain, target_url, COALESCE(certificate_pem, ''), active FROM user_proxy_records
		WHERE user_id = ? AND active = 1
		ORDER BY domain ASC`, userID)
	if err != nil {
		log.Error().Err(err).Int("user_id", userID).Msg("Error querying user proxy records")
		return nil, err
	}
	defer rows.Close()

	var records []map[string]interface{}
	for rows.Next() {
		var id int
		var domain, targetURL, certPem string
		var active int
		if err := rows.Scan(&id, &domain, &targetURL, &certPem, &active); err != nil {
			log.Error().Err(err).Msg("Error scanning proxy record row")
			continue
		}
		records = append(records, map[string]interface{}{
			"id": id, "domain": domain, "target_url": targetURL, "has_cert": len(certPem) > 0, "active": active == 1,
		})
	}
	log.Debug().Int("user_id", userID).Int("count", len(records)).Msg("Retrieved user proxy records")
	return records, nil
}

// ListAllDomains returns all active domains in the system
func ListAllDomains(db *sql.DB) ([]map[string]interface{}, error) {
	rows, err := db.Query(`
		SELECT id, domain, target_url, active, created_at FROM routes
		WHERE active = 1
		ORDER BY domain ASC`)
	if err != nil {
		log.Error().Err(err).Msg("Error listing all domains")
		return nil, err
	}
	defer rows.Close()

	var domains []map[string]interface{}
	for rows.Next() {
		var id int
		var domain, targetURL string
		var active int
		var createdAt string
		if err := rows.Scan(&id, &domain, &targetURL, &active, &createdAt); err != nil {
			log.Error().Err(err).Msg("Error scanning domain row")
			continue
		}
		domains = append(domains, map[string]interface{}{
			"id": id, "domain": domain, "target_url": targetURL, "active": active == 1, "created_at": createdAt,
		})
	}
	log.Debug().Int("count", len(domains)).Msg("Listed all active domains")
	return domains, nil
}

// IsZeroTrustEnabled returns whether zero-trust is enabled globally
func IsZeroTrustEnabled(db *sql.DB) bool {
	var value string
	err := db.QueryRow("SELECT value FROM zero_trust_settings WHERE key = 'enabled'").Scan(&value)
	if err != nil {
		return true // Default to enabled
	}
	return value == "true"
}

// SetZeroTrustEnabled sets the global zero-trust setting
func SetZeroTrustEnabled(db *sql.DB, enabled bool) error {
	val := "false"
	if enabled {
		val = "true"
	}
	_, err := db.Exec(`
		INSERT INTO zero_trust_settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, "enabled", val)
	return err
}

// SetUserZeroTrustEnabled sets zero-trust for a specific user
func SetUserZeroTrustEnabled(db *sql.DB, userID int, enabled int) error {
	_, err := db.Exec("UPDATE users SET zero_trust_enabled = ? WHERE id = ?", enabled, userID)
	return err
}

// GetUserZeroTrustStatus returns whether zero-trust is enabled for a user
func GetUserZeroTrustStatus(db *sql.DB, userID int) (bool, error) {
	var enabled int
	err := db.QueryRow("SELECT zero_trust_enabled FROM users WHERE id = ?", userID).Scan(&enabled)
	return enabled == 1, err
}
