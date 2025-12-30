package honeypot

import (
	"net/http"
	"strings"
)

func Check(w http.ResponseWriter, r *http.Request) bool {
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
