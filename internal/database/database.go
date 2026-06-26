package database

import (
	"database/sql"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

func Init(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	if err := createTables(db); err != nil {
		return nil, err
	}

	return db, nil
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

	return migrateRouteTargets(db)
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
	if err == nil && count == 0 {
		hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		_, err = db.Exec(`INSERT INTO users (username, password_hash) VALUES (?, ?)`, "admin", string(hash))
		if err != nil {
			log.Error().Err(err).Msg("Failed to insert default user")
		} else {
			log.Info().Msg("Inserted default user: admin / admin")
		}
	}

	err = db.QueryRow("SELECT COUNT(*) FROM waf_rules").Scan(&count)
	if err == nil && count == 0 {
		// --- ENHANCED OWASP TOP 10 WAF RULES ---
		rules := []struct {
			Name       string
			Expression string
			Priority   int
		}{
			// General Admin Block
			{"Block Admin", `Path startsWith "/admin"`, 10},
			
			// OWASP A03:2021 - Injection (SQLi)
			{"Block SQL Injection (Path)", `Path matches ".*(?i)(union\\s+select|waitfor\\s+delay|1=1|--|;).*$"`, 20},
			{"Block SQL Injection (Query)", `RawQuery matches "(?i)(union\\s+select|waitfor\\s+delay|1=1|--|;)"`, 20},
			
			// OWASP A03:2021 - Injection (XSS)
			{"Block XSS (Path)", `Path matches "(?i)(<script>|javascript:|onerror=)"`, 20},
			{"Block XSS (Query)", `RawQuery matches "(?i)(<script>|javascript:|onerror=)"`, 20},
			
			// OWASP A01:2021 - Broken Access Control (Path Traversal)
			{"Block Path Traversal", `Path matches "(?:\\.\\./|\\.\\.\\\\)"`, 20},
			{"Block Path Traversal (Path Encoded)", `Path matches ".*(?i)(%2e%2e%2f|%2e%2e%5c).*$"`, 20},
			{"Block Path Traversal (Query)", `RawQuery matches "(?:\\.\\./|\\.\\.\\\\)"`, 20},
			{"Block Path Traversal (Query Encoded)", `RawQuery matches ".*(?i)(%2e%2e%2f|%2e%2e%5c).*$"`, 20},
			
			// OWASP A10:2021 - Server-Side Request Forgery (SSRF)
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
		WHERE route_type = 'domain' AND domain = ? AND active = 1
		LIMIT 1`, domain).Scan(&routeID, &targetURL, &certPem, &keyPem)
	return routeID, targetURL, certPem, keyPem, err
}

func findRouteByPath(db *sql.DB, path string) (int, string, error) {
	var routeID int
	var targetURL string
	err := db.QueryRow(`
		SELECT id, target_url FROM routes
		WHERE route_type = 'path' AND ? LIKE path_prefix || '%' AND active = 1
		ORDER BY LENGTH(path_prefix) DESC
		LIMIT 1`, path).Scan(&routeID, &targetURL)
	return routeID, targetURL, err
}

// GetRouteTargets resolves a route and returns all configured upstream targets.
func GetRouteTargets(db *sql.DB, domain, path string) (*RouteMatch, error) {
	if domain != "" {
		domain = strings.Split(domain, ":")[0]
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
	}

	if path != "" {
		routeID, fallbackURL, err := findRouteByPath(db, path)
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
					RouteKey: "path:" + path,
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

// SetRouteTargets replaces upstream targets for a route.
func SetRouteTargets(db *sql.DB, routeID int, targets []RouteTarget) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM route_targets WHERE route_id = ?`, routeID); err != nil {
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
		if _, err := tx.Exec(
			`INSERT INTO route_targets (route_id, target_url, health_check, sort_order) VALUES (?, ?, ?, ?)`,
			routeID, t.URL, check, i); err != nil {
			return err
		}
	}

	return tx.Commit()
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
