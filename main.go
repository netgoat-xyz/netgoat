package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

func main() {
	target := "http://example.com"
	if len(os.Args) > 1 {
		target = os.Args[1]
	}

	targetURL, err := url.Parse(target)
	if err != nil {
		log.Fatalf("Invalid target URL: %v", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = targetURL.Host
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Proxying request: %s %s", r.Method, r.URL.Path)
		proxy.ServeHTTP(w, r)
	})

	port := ":8080"
	log.Printf("Reverse proxy listening on %s, forwarding to %s", port, target)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal(err)
	}
}
