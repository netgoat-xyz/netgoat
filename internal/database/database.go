package database

import (
	"database/sql"

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
		path_prefix TEXT NOT NULL UNIQUE,
		target_url TEXT NOT NULL
	);`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS waf_rules (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		expression TEXT NOT NULL,
		action TEXT NOT NULL DEFAULT 'BLOCK',
		priority INTEGER DEFAULT 0
	);`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL
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
		_, err = db.Exec(`INSERT INTO routes (path_prefix, target_url) VALUES (?, ?)`, "/", "http://example.com")
		if err != nil {
			return err
		}
		log.Info().Str("route", "/").Str("target", "http://example.com").Msg("Inserted default route")
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
			{"Block SQL Injection", `Path matches "(?i)(union|select|insert|delete|update|drop).*"` , 20},
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

func GetTarget(db *sql.DB, path string) string {
	var target string
	err := db.QueryRow(`
		SELECT target_url FROM routes 
		WHERE ? LIKE path_prefix || '%' 
		ORDER BY length(path_prefix) DESC 
		LIMIT 1`, path).Scan(&target)

	if err != nil {
		if err != sql.ErrNoRows {
			log.Error().Err(err).Str("path", path).Msg("Error querying target")
		}
		return ""
	}
	return target
}
