package main

import (
	"database/sql"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/expr-lang/expr"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// Config holds the application configuration
type Config struct {
	DebugLogs bool `yaml:"debug_logs"`
}

// WAFContext defines the variables available in the rule script
type WAFContext struct {
	IP      string
	Method  string
	Path    string
	Headers map[string][]string
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	// Load configuration
	var config Config
	configFile, err := ioutil.ReadFile("config.yml")
	if err == nil {
		if err := yaml.Unmarshal(configFile, &config); err != nil {
			log.Error().Err(err).Msg("Failed to parse config.yml")
		} else {
			log.Info().Bool("debug_logs", config.DebugLogs).Msg("Loaded configuration")
		}
	} else {
		log.Warn().Err(err).Msg("Could not read config.yml, using defaults")
	}

	if err := os.MkdirAll("./database", 0755); err != nil {
		log.Fatal().Err(err).Msg("Failed to create database directory")
	}

	db, err := sql.Open("sqlite3", "./database/proxy.db")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to open database")
	}
	defer db.Close()

	initDB(db)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// WAF Check
		blocked, ruleName := checkWAF(db, r, config)
		if blocked {
			log.Warn().Str("rule", ruleName).Str("ip", r.RemoteAddr).Str("path", r.URL.Path).Msg("Request blocked by WAF")
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		targetStr := getTarget(db, r.URL.Path)
		if targetStr == "" {
			http.Error(w, "No route found", http.StatusNotFound)
			return
		}

		targetURL, err := url.Parse(targetStr)
		if err != nil {
			log.Error().Err(err).Str("target", targetStr).Msg("Invalid target URL in DB")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		log.Info().Str("method", r.Method).Str("path", r.URL.Path).Str("target", targetStr).Msg("Proxying request")

		proxy := httputil.NewSingleHostReverseProxy(targetURL)

		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			req.Host = targetURL.Host
		}

		proxy.ServeHTTP(w, r)
	})

	port := ":8080"
	log.Info().Str("port", port).Msg("Reverse proxy listening")
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal().Err(err).Msg("Server failed")
	}
}

func initDB(db *sql.DB) {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS routes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		path_prefix TEXT NOT NULL UNIQUE,
		target_url TEXT NOT NULL
	);`)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create routes table")
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS waf_rules (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		expression TEXT NOT NULL,
		action TEXT NOT NULL DEFAULT 'BLOCK',
		priority INTEGER DEFAULT 0
	);`)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create waf_rules table")
	}

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM routes").Scan(&count)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to check routes count")
	}

	if count == 0 {
		_, err = db.Exec(`INSERT INTO routes (path_prefix, target_url) VALUES (?, ?)`, "/", "http://example.com")
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to insert default route")
		}
		log.Info().Str("route", "/").Str("target", "http://example.com").Msg("Inserted default route")
	}

	// Insert a sample WAF rule if empty
	err = db.QueryRow("SELECT COUNT(*) FROM waf_rules").Scan(&count)
	if err == nil && count == 0 {
		// Block requests to /admin from non-localhost (example)
		// Note: This is just a sample.
		_, err = db.Exec(`INSERT INTO waf_rules (name, expression, action, priority) VALUES (?, ?, ?, ?)`,
			"Block Admin",
			`Path startsWith "/admin"`,
			"BLOCK",
			10)
		if err != nil {
			log.Error().Err(err).Msg("Failed to insert default WAF rule")
		} else {
			log.Info().Msg("Inserted default WAF rule: Block Admin")
		}
	}
}

func checkWAF(db *sql.DB, r *http.Request, config Config) (bool, string) {
	rows, err := db.Query("SELECT name, expression, action FROM waf_rules ORDER BY priority DESC")
	if err != nil {
		log.Error().Err(err).Msg("Failed to query WAF rules")
		return false, ""
	}
	defer rows.Close()

	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip == "" {
		ip = r.RemoteAddr
	}

	env := WAFContext{
		IP:      ip,
		Method:  r.Method,
		Path:    r.URL.Path,
		Headers: r.Header,
	}

	for rows.Next() {
		var name, expression, action string
		if err := rows.Scan(&name, &expression, &action); err != nil {
			continue
		}

		program, err := expr.Compile(expression, expr.Env(WAFContext{}))
		if err != nil {
			log.Error().Err(err).Str("rule", name).Msg("Invalid WAF rule expression")
			continue
		}

		output, err := expr.Run(program, env)
		if err != nil {
			log.Error().Err(err).Str("rule", name).Msg("Error running WAF rule")
			continue
		}

		matched, ok := output.(bool)
		if config.DebugLogs {
			log.Debug().Str("rule", name).Str("expression", expression).Bool("matched", matched).Msg("WAF Rule Evaluation")
		}

		if ok && matched {
			if strings.ToUpper(action) == "BLOCK" {
				return true, name
			}
		}
	}
	return false, ""
}



func getTarget(db *sql.DB, path string) string {
	// Find the longest matching prefix
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
