package main

import (
	"database/sql"
	"encoding/base64"
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
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
)

// Config holds the application configuration
type Config struct {
	DebugLogs bool `yaml:"debug_logs"`
	Honeypot  bool `yaml:"honeypot"`
	Auth      struct {
		Enabled       bool   `yaml:"enabled"`
		SessionSecret string `yaml:"session_secret"`
	} `yaml:"auth"`
	SSL struct {
		Enabled  bool   `yaml:"enabled"`
		CertFile string `yaml:"cert_file"`
		KeyFile  string `yaml:"key_file"`
		Port     string `yaml:"port"`
	} `yaml:"ssl"`
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
			log.Info().Bool("debug_logs", config.DebugLogs).Bool("honeypot", config.Honeypot).Bool("auth_enabled", config.Auth.Enabled).Msg("Loaded configuration")
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

	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		handleLogin(w, r, db)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Auth Check (ZeroTrust)
		if config.Auth.Enabled {
			if !checkAuth(w, r) {
				// If it's an API call, return 401, else redirect to login
				if strings.Contains(r.Header.Get("Accept"), "application/json") {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
				} else {
					http.Redirect(w, r, "/login", http.StatusFound)
				}
				return
			}
		}

		// Honeypot Check
		if config.Honeypot {
			if checkHoneypot(w, r) {
				log.Warn().Str("ip", r.RemoteAddr).Str("path", r.URL.Path).Msg("Honeypot triggered")
				return
			}
		}

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

		// WebSocket Support Check
		if r.Header.Get("Upgrade") == "websocket" {
			log.Info().Str("client", r.RemoteAddr).Msg("WebSocket upgrade detected")
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

	if config.SSL.Enabled {
		port := config.SSL.Port
		if port == "" {
			port = ":8443"
		}
		log.Info().Str("port", port).Msg("Reverse proxy listening (HTTPS)")
		if err := http.ListenAndServeTLS(port, config.SSL.CertFile, config.SSL.KeyFile, nil); err != nil {
			log.Fatal().Err(err).Msg("Server failed")
		}
	} else {
		port := ":8080"
		log.Info().Str("port", port).Msg("Reverse proxy listening (HTTP)")
		if err := http.ListenAndServe(port, nil); err != nil {
			log.Fatal().Err(err).Msg("Server failed")
		}
	}
}

func checkAuth(w http.ResponseWriter, r *http.Request) bool {
	// Simple Basic Auth for "ZeroTrust" demonstration
	// In a real scenario, this would check a session cookie or JWT
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		// Check for cookie
		cookie, err := r.Cookie("auth_token")
		if err == nil && cookie.Value == "valid_session" {
			return true
		}
		return false
	}

	// Basic Auth parsing
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Basic" {
		return false
	}

	payload, _ := base64.StdEncoding.DecodeString(parts[1])
	pair := strings.SplitN(string(payload), ":", 2)
	if len(pair) != 2 {
		return false
	}

	// Verify against DB (implemented in handleLogin, but here we need to verify every request if using Basic Auth)
	// For simplicity in this "ZeroTrust" example, we'll rely on the session cookie set by /login
	// or we could verify Basic Auth against DB here.
	// Let's stick to the cookie for browser flow.
	return false
}

func handleLogin(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method == "GET" {
		w.Write([]byte(`
			<html><body>
			<h1>ZeroTrust Login</h1>
			<form method="POST">
				User: <input type="text" name="username"><br>
				Pass: <input type="password" name="password"><br>
				<input type="submit" value="Login">
			</form>
			</body></html>
		`))
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	var hash string
	err := db.QueryRow("SELECT password_hash FROM users WHERE username = ?", username).Scan(&hash)
	if err != nil {
		log.Warn().Str("user", username).Msg("Login failed: user not found")
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		log.Warn().Str("user", username).Msg("Login failed: bad password")
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:  "auth_token",
		Value: "valid_session", // In real app, use a signed JWT or random token
		Path:  "/",
	})

	http.Redirect(w, r, "/", http.StatusFound)
}

func checkHoneypot(w http.ResponseWriter, r *http.Request) bool {
	path := r.URL.Path
	if path == "/.env" {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("DB_PASSWORD=supersecret\nAWS_ACCESS_KEY=AKIAIOSFODNN7EXAMPLE\n"))
		return true
	}
	if strings.Contains(path, "/.git/") || path == "/.git" {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("[core]\n\trepositoryformatversion = 0\n\tfilemode = true\n\tbare = false\n\tlogallrefupdates = true\n"))
		return true
	}
	return false
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

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL
	);`)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create users table")
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

	// Insert default admin user if empty
	err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err == nil && count == 0 {
		// Default password is "admin"
		hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		_, err = db.Exec(`INSERT INTO users (username, password_hash) VALUES (?, ?)`, "admin", string(hash))
		if err != nil {
			log.Error().Err(err).Msg("Failed to insert default user")
		} else {
			log.Info().Msg("Inserted default user: admin / admin")
		}
	}

	// Insert default WAF rules if empty
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
