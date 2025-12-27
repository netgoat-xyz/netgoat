package main

import (
	"database/sql"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", "./database/proxy.db")
	if err != nil {
		log.Fatal(err)
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
			log.Printf("Invalid target URL in DB: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		log.Printf("Proxying request: %s %s -> %s", r.Method, r.URL.Path, targetStr)

		proxy := httputil.NewSingleHostReverseProxy(targetURL)

		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			req.Host = targetURL.Host
		}

		proxy.ServeHTTP(w, r)
	})

	port := ":8080"
	log.Printf("Reverse proxy listening on %s", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal(err)
	}
}

func initDB(db *sql.DB) {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS routes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		path_prefix TEXT NOT NULL UNIQUE,
		target_url TEXT NOT NULL
	);`)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM routes").Scan(&count)
	if err != nil {
		log.Fatalf("Failed to check routes count: %v", err)
	}

	if count == 0 {
		_, err = db.Exec(`INSERT INTO routes (path_prefix, target_url) VALUES (?, ?)`, "/", "http://example.com")
		if err != nil {
			log.Fatalf("Failed to insert default route: %v", err)
		}
		log.Println("Inserted default route: / -> http://example.com")
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
			log.Printf("Error querying target: %v", err)
		}
		return ""
	}
	return target
}
