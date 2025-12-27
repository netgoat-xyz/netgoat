package main

import (
	"database/sql"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

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
		log.Fatal().Err(err).Msg("Failed to create table")
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
