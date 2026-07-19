package auth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestHandleLoginCreatesValidSessionCookie(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	insertTestUser(t, db, "loginuser", "loginpass", 0)

	form := url.Values{}
	form.Set("username", "loginuser")
	form.Set("password", "loginpass")

	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	HandleLogin(w, req, db)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}

	var token string
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name != "auth_token" {
			continue
		}
		token = cookie.Value
		if cookie.Path != "/" {
			t.Errorf("cookie path = %s, want /", cookie.Path)
		}
		if !cookie.HttpOnly {
			t.Error("cookie should be HttpOnly")
		}
		if cookie.SameSite != http.SameSiteStrictMode {
			t.Errorf("cookie SameSite = %v, want %v", cookie.SameSite, http.SameSiteStrictMode)
		}
		if cookie.MaxAge != 24*60*60 {
			t.Errorf("cookie MaxAge = %d, want one day", cookie.MaxAge)
		}
	}
	if token == "" {
		t.Fatal("auth_token cookie should be set")
	}

	result := &AuthResult{}
	if !validateSession(db, token, result) {
		t.Fatal("auth_token cookie should reference a stored session")
	}
	if result.Username != "loginuser" {
		t.Errorf("session username = %s, want loginuser", result.Username)
	}
}

func TestHandleLoginFormUsesPost(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	req := httptest.NewRequest("GET", "/login", nil)
	w := httptest.NewRecorder()

	HandleLogin(w, req, db)

	body := w.Body.String()
	if !strings.Contains(body, `method="POST"`) {
		t.Fatal("login form should submit with POST")
	}
	if !strings.Contains(body, `action="/login"`) {
		t.Fatal("login form should post to /login")
	}
}

func TestRequireZeroTrustChallenge(t *testing.T) {
	tests := []struct {
		name          string
		result        *AuthResult
		globalEnabled bool
		verified      bool
		want          bool
	}{
		{"nil auth result", nil, true, false, false},
		{"unauthenticated user", &AuthResult{Authenticated: false, ZeroTrustReq: true}, true, false, false},
		{"global zero trust disabled", &AuthResult{Authenticated: true, ZeroTrustReq: true}, false, false, false},
		{"user does not require zero trust", &AuthResult{Authenticated: true, ZeroTrustReq: false}, true, false, false},
		{"already verified", &AuthResult{Authenticated: true, ZeroTrustReq: true}, true, true, false},
		{"challenge required", &AuthResult{Authenticated: true, ZeroTrustReq: true}, true, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RequireZeroTrustChallenge(tt.result, tt.globalEnabled, tt.verified)
			if got != tt.want {
				t.Errorf("RequireZeroTrustChallenge() = %v, want %v", got, tt.want)
			}
		})
	}
}
