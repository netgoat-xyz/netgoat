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

	if err := seedDefaults(db); err != nil {
		return err
	}

	return nil
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
		rules := []struct {
			Name       string
			Expression string
			Priority   int
		}{
			{"Block Admin", `Path startsWith "/admin"`, 10},
			{"Block SQL Injection", `Path matches "(?i)(union|select|insert|delete|update|drop).*"`, 20},
			{"Block XSS", `Path matches "(?i)<script>"`, 20},
			{"Block Path Traversal", `Path matches "\\.\\./"`, 20},
		}

		for _, rule := range rules {
			_, err = db.Exec(`INSERT INTO waf_rules (name, expression, action, priority) VALUES (?, ?, ?, ?)`,
				rule.Name, rule.Expression, "BLOCK", rule.Priority)
			if err != nil {
				log.Error().Err(err).Str("rule", rule.Name).Msg("Failed to insert WAF rule")
			} else {
				log.Info().Str("rule", rule.Name).Msg("Inserted default WAF rule")
			}
		}
	}
	return nil
}

// GetTargetByDomain returns the target URL and certificate for a domain
func GetTargetByDomain(db *sql.DB, domain string) (string, string, string, error) {
	var targetURL, certPem, keyPem string
	err := db.QueryRow(`
		SELECT target_url, COALESCE(certificate_pem, ''), COALESCE(private_key_pem, '') FROM routes 
		WHERE route_type = 'domain' AND domain = ? AND active = 1
		LIMIT 1`, domain).Scan(&targetURL, &certPem, &keyPem)

	if err != nil {
		if err != sql.ErrNoRows {
			log.Error().Err(err).Str("domain", domain).Msg("Error querying target by domain")
		}
		return "", "", "", err
	}
	log.Debug().Str("domain", domain).Str("target_url", targetURL).Msg("Found domain route")
	return targetURL, certPem, keyPem, nil
}

// GetTargetByPath returns the target URL for a path (path-based routing)
func GetTargetByPath(db *sql.DB, path string) (string, error) {
	var targetURL string
	err := db.QueryRow(`
		SELECT target_url FROM routes 
		WHERE route_type = 'path' AND ? LIKE path_prefix || '%' AND active = 1
		ORDER BY LENGTH(path_prefix) DESC 
		LIMIT 1`, path).Scan(&targetURL)

	if err != nil {
		if err != sql.ErrNoRows {
			log.Error().Err(err).Str("path", path).Msg("Error querying target by path")
		}
		return "", err
	}
	log.Debug().Str("path", path).Str("target_url", targetURL).Msg("Found path route")
	return targetURL, nil
}

// GetTarget tries domain-based first, then falls back to path-based
func GetTarget(db *sql.DB, domain, path string) (string, error) {
	// Try domain-based routing first
	if domain != "" {
		domain := strings.Split(domain, ":")[0] // Remove port
		targetURL, _, _, err := GetTargetByDomain(db, domain)
		if err == nil && targetURL != "" {
			log.Debug().Str("domain", domain).Msg("Using domain-based route")
			return targetURL, nil
		}
	}

	// Fall back to path-based routing
	if path != "" {
		targetURL, err := GetTargetByPath(db, path)
		if err == nil && targetURL != "" {
			log.Debug().Str("path", path).Msg("Using path-based route")
			return targetURL, nil
		}
	}

	log.Warn().Str("domain", domain).Str("path", path).Msg("No route found for domain or path")
	return "", sql.ErrNoRows
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
