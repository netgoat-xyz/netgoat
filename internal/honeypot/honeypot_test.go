package honeypot

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCheckDotEnv(t *testing.T) {
	req := httptest.NewRequest("GET", "/.env", nil)
	w := httptest.NewRecorder()

	triggered := Check(w, req)

	if !triggered {
		t.Error("Should trigger on /.env request")
	}

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "text/plain" {
		t.Errorf("Content-Type = %s, want text/plain", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "DB_PASSWORD") {
		t.Error("Response should contain DB_PASSWORD")
	}
	if !strings.Contains(body, "AWS_ACCESS_KEY") {
		t.Error("Response should contain AWS_ACCESS_KEY")
	}
}

func TestCheckGitDirectory(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{
			name: "exact .git",
			path: "/.git",
		},
		{
			name: ".git with trailing slash",
			path: "/.git/",
		},
		{
			name: ".git/config",
			path: "/.git/config",
		},
		{
			name: ".git/HEAD",
			path: "/.git/HEAD",
		},
		{
			name: "nested .git path",
			path: "/some/path/.git/objects",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			triggered := Check(w, req)

			if !triggered {
				t.Errorf("Should trigger on %s request", tt.path)
			}

			if w.Code != http.StatusOK {
				t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
			}

			contentType := w.Header().Get("Content-Type")
			if contentType != "text/plain" {
				t.Errorf("Content-Type = %s, want text/plain", contentType)
			}

			body := w.Body.String()
			if !strings.Contains(body, "[core]") {
				t.Error("Response should contain [core]")
			}
			if !strings.Contains(body, "repositoryformatversion") {
				t.Error("Response should contain repositoryformatversion")
			}
		})
	}
}

func TestCheckNormalPaths(t *testing.T) {
	normalPaths := []string{
		"/",
		"/index.html",
		"/api/users",
		"/admin/dashboard",
		"/assets/style.css",
		"/images/logo.png",
		"/.well-known/security.txt",
		"/robots.txt",
		"/favicon.ico",
	}

	for _, path := range normalPaths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			w := httptest.NewRecorder()

			triggered := Check(w, req)

			if triggered {
				t.Errorf("Should not trigger on normal path %s", path)
			}

			// Should not write anything for normal paths
			if w.Body.Len() > 0 {
				t.Errorf("Should not write response for normal path %s", path)
			}
		})
	}
}

func TestCheckCaseSensitivity(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		shouldTrigger bool
	}{
		{
			name:        "lowercase .env",
			path:        "/.env",
			shouldTrigger: true,
		},
		{
			name:        "uppercase .ENV",
			path:        "/.ENV",
			shouldTrigger: false,
		},
		{
			name:        "mixed case .Env",
			path:        "/.Env",
			shouldTrigger: false,
		},
		{
			name:        "lowercase .git",
			path:        "/.git",
			shouldTrigger: true,
		},
		{
			name:        "uppercase .GIT",
			path:        "/.GIT",
			shouldTrigger: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			triggered := Check(w, req)

			if triggered != tt.shouldTrigger {
				t.Errorf("Check(%s) = %v, want %v", tt.path, triggered, tt.shouldTrigger)
			}
		})
	}
}

func TestCheckWithQueryString(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		shouldTrigger bool
	}{
		{
			name:        ".env with query",
			path:        "/.env?debug=true",
			shouldTrigger: true,
		},
		{
			name:        ".git with query",
			path:        "/.git?test=1",
			shouldTrigger: true,
		},
		{
			name:        "normal path with .env in query",
			path:        "/page?file=.env",
			shouldTrigger: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			triggered := Check(w, req)

			if triggered != tt.shouldTrigger {
				t.Errorf("Check(%s) = %v, want %v", tt.path, triggered, tt.shouldTrigger)
			}
		})
	}
}

func TestCheckDifferentHTTPMethods(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "HEAD", "OPTIONS"}

	for _, method := range methods {
		t.Run(method+" /.env", func(t *testing.T) {
			req := httptest.NewRequest(method, "/.env", nil)
			w := httptest.NewRecorder()

			triggered := Check(w, req)

			if !triggered {
				t.Errorf("Should trigger on %s /.env", method)
			}
		})

		t.Run(method+" /.git", func(t *testing.T) {
			req := httptest.NewRequest(method, "/.git", nil)
			w := httptest.NewRecorder()

			triggered := Check(w, req)

			if !triggered {
				t.Errorf("Should trigger on %s /.git", method)
			}
		})
	}
}

func TestCheckResponseContent(t *testing.T) {
	// Test .env response
	req := httptest.NewRequest("GET", "/.env", nil)
	w := httptest.NewRecorder()
	Check(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "supersecret") {
		t.Error(".env response should contain fake secrets")
	}
	if !strings.Contains(body, "AKIAIOSFODNN7EXAMPLE") {
		t.Error(".env response should contain fake AWS key")
	}

	// Test .git response
	req = httptest.NewRequest("GET", "/.git", nil)
	w = httptest.NewRecorder()
	Check(w, req)

	body = w.Body.String()
	if !strings.Contains(body, "filemode") {
		t.Error(".git response should contain git config content")
	}
	if !strings.Contains(body, "bare = false") {
		t.Error(".git response should contain bare config")
	}
}

func TestCheckDoesNotModifyRequest(t *testing.T) {
	originalPath := "/.env"
	req := httptest.NewRequest("GET", originalPath, nil)
	w := httptest.NewRecorder()

	Check(w, req)

	if req.URL.Path != originalPath {
		t.Error("Check should not modify request path")
	}
}

func TestCheckMultipleCalls(t *testing.T) {
	req := httptest.NewRequest("GET", "/.env", nil)

	// Call multiple times
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		triggered := Check(w, req)

		if !triggered {
			t.Errorf("Call %d: Should trigger on /.env", i+1)
		}

		body := w.Body.String()
		if !strings.Contains(body, "DB_PASSWORD") {
			t.Errorf("Call %d: Response should contain DB_PASSWORD", i+1)
		}
	}
}

func TestCheckEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		shouldTrigger bool
	}{
		{
			name:        "double slash .env",
			path:        "//.env",
			shouldTrigger: false,
		},
		{
			name:        ".env in subdirectory",
			path:        "/subdir/.env",
			shouldTrigger: false, // Only exact /.env triggers
		},
		{
			name:        ".gitignore (not .git)",
			path:        "/.gitignore",
			shouldTrigger: false,
		},
		{
			name:        ".github directory",
			path:        "/.github/",
			shouldTrigger: false,
		},
		{
			name:        "env without dot",
			path:        "/env",
			shouldTrigger: false,
		},
		{
			name:        "git without dot",
			path:        "/git",
			shouldTrigger: false,
		},
		{
			name:        ".git in middle of path",
			path:        "/path/.git/subdir",
			shouldTrigger: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			triggered := Check(w, req)

			if triggered != tt.shouldTrigger {
				t.Errorf("Check(%s) = %v, want %v", tt.path, triggered, tt.shouldTrigger)
			}
		})
	}
}

func TestCheckReturnValue(t *testing.T) {
	// Test that return value matches write behavior
	tests := []struct {
		path          string
		shouldTrigger bool
	}{
		{"/.env", true},
		{"/.git", true},
		{"/.git/config", true},
		{"/normal", false},
		{"/", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			triggered := Check(w, req)

			hasBody := w.Body.Len() > 0
			if triggered != hasBody {
				t.Errorf("Return value (%v) doesn't match write behavior (body len=%d)", triggered, w.Body.Len())
			}

			if triggered != tt.shouldTrigger {
				t.Errorf("Check(%s) = %v, want %v", tt.path, triggered, tt.shouldTrigger)
			}
		})
	}
}