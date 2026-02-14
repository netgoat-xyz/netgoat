package auth

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create users table
	_, err = db.Exec(`CREATE TABLE users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		email TEXT,
		zero_trust_enabled INTEGER DEFAULT 0
	)`)
	if err != nil {
		t.Fatalf("Failed to create users table: %v", err)
	}

	// Create sessions table
	_, err = db.Exec(`CREATE TABLE user_sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		token TEXT NOT NULL UNIQUE,
		expires_at DATETIME NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		t.Fatalf("Failed to create user_sessions table: %v", err)
	}

	return db
}

func insertTestUser(t *testing.T, db *sql.DB, username, password string, zeroTrust int) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	_, err = db.Exec("INSERT INTO users (username, password_hash, zero_trust_enabled) VALUES (?, ?, ?)",
		username, string(hash), zeroTrust)
	if err != nil {
		t.Fatalf("Failed to insert test user: %v", err)
	}
}

func TestCheckWithNoAuth(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	req := httptest.NewRequest("GET", "/", nil)
	result := Check(req, db)

	if result.Authenticated {
		t.Error("Should not be authenticated with no auth")
	}
	if result.Username != "" {
		t.Errorf("Username should be empty, got %s", result.Username)
	}
	if result.UserID != 0 {
		t.Errorf("UserID should be 0, got %d", result.UserID)
	}
}

func TestCheckWithBasicAuth(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	insertTestUser(t, db, "testuser", "testpass", 0)

	tests := []struct {
		name          string
		username      string
		password      string
		wantAuth      bool
		wantUsername  string
		wantZeroTrust bool
	}{
		{
			name:          "valid credentials",
			username:      "testuser",
			password:      "testpass",
			wantAuth:      true,
			wantUsername:  "testuser",
			wantZeroTrust: false,
		},
		{
			name:          "wrong password",
			username:      "testuser",
			password:      "wrongpass",
			wantAuth:      false,
			wantUsername:  "",
			wantZeroTrust: false,
		},
		{
			name:          "nonexistent user",
			username:      "nobody",
			password:      "pass",
			wantAuth:      false,
			wantUsername:  "",
			wantZeroTrust: false,
		},
		{
			name:          "empty password",
			username:      "testuser",
			password:      "",
			wantAuth:      false,
			wantUsername:  "",
			wantZeroTrust: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.SetBasicAuth(tt.username, tt.password)

			result := Check(req, db)

			if result.Authenticated != tt.wantAuth {
				t.Errorf("Authenticated = %v, want %v", result.Authenticated, tt.wantAuth)
			}
			if result.Username != tt.wantUsername {
				t.Errorf("Username = %s, want %s", result.Username, tt.wantUsername)
			}
			if result.ZeroTrustReq != tt.wantZeroTrust {
				t.Errorf("ZeroTrustReq = %v, want %v", result.ZeroTrustReq, tt.wantZeroTrust)
			}
			if tt.wantAuth && result.SessionToken == "" {
				t.Error("SessionToken should be set for authenticated user")
			}
			if !tt.wantAuth && result.SessionToken != "" {
				t.Error("SessionToken should be empty for unauthenticated user")
			}
		})
	}
}

func TestCheckWithZeroTrust(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	insertTestUser(t, db, "ztuser", "ztpass", 1)

	req := httptest.NewRequest("GET", "/", nil)
	req.SetBasicAuth("ztuser", "ztpass")

	result := Check(req, db)

	if !result.Authenticated {
		t.Error("Should be authenticated")
	}
	if !result.ZeroTrustReq {
		t.Error("ZeroTrustReq should be true")
	}
}

func TestCheckWithInvalidBasicAuth(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tests := []struct {
		name   string
		header string
	}{
		{
			name:   "missing space",
			header: "Basicdm9nZXI6c3VwZXJzZWNyZXQ=",
		},
		{
			name:   "wrong scheme",
			header: "Bearer token123",
		},
		{
			name:   "invalid base64",
			header: "Basic !!!invalid!!!",
		},
		{
			name:   "missing colon",
			header: "Basic " + "dXNlcm5hbWVvbmx5", // base64 of "usernameonly"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("Authorization", tt.header)

			result := Check(req, db)

			if result.Authenticated {
				t.Error("Should not be authenticated with invalid auth header")
			}
		})
	}
}

func TestCheckWithCookie(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	insertTestUser(t, db, "cookieuser", "cookiepass", 0)

	// Get user ID
	var userID int
	err := db.QueryRow("SELECT id FROM users WHERE username = ?", "cookieuser").Scan(&userID)
	if err != nil {
		t.Fatalf("Failed to get user ID: %v", err)
	}

	// Create a session
	token := "valid_session_token_123"
	_, err = db.Exec(`INSERT INTO user_sessions (user_id, token, expires_at) VALUES (?, ?, datetime('now', '+24 hours'))`,
		userID, token)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  "auth_token",
		Value: token,
	})

	result := Check(req, db)

	if !result.Authenticated {
		t.Error("Should be authenticated with valid cookie")
	}
	if result.Username != "cookieuser" {
		t.Errorf("Username = %s, want cookieuser", result.Username)
	}
	if result.SessionToken != token {
		t.Errorf("SessionToken = %s, want %s", result.SessionToken, token)
	}
}

func TestCheckWithExpiredSession(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	insertTestUser(t, db, "expireduser", "pass", 0)

	var userID int
	db.QueryRow("SELECT id FROM users WHERE username = ?", "expireduser").Scan(&userID)

	// Create expired session
	token := "expired_token"
	_, err := db.Exec(`INSERT INTO user_sessions (user_id, token, expires_at) VALUES (?, ?, datetime('now', '-1 hour'))`,
		userID, token)
	if err != nil {
		t.Fatalf("Failed to create expired session: %v", err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  "auth_token",
		Value: token,
	})

	result := Check(req, db)

	if result.Authenticated {
		t.Error("Should not be authenticated with expired session")
	}
}

func TestCheckWithInvalidCookie(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  "auth_token",
		Value: "nonexistent_token",
	})

	result := Check(req, db)

	if result.Authenticated {
		t.Error("Should not be authenticated with invalid cookie")
	}
}

func TestCheckBasicAuthOverCookie(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	insertTestUser(t, db, "basicuser", "basicpass", 0)

	req := httptest.NewRequest("GET", "/", nil)
	req.SetBasicAuth("basicuser", "basicpass")
	req.AddCookie(&http.Cookie{
		Name:  "auth_token",
		Value: "some_token",
	})

	result := Check(req, db)

	// Basic auth should take precedence over cookie
	if !result.Authenticated {
		t.Error("Should be authenticated with valid basic auth")
	}
	if result.Username != "basicuser" {
		t.Errorf("Username = %s, want basicuser", result.Username)
	}
}

func TestHandleLoginGET(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	req := httptest.NewRequest("GET", "/login", nil)
	w := httptest.NewRecorder()

	HandleLogin(w, req, db)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if !strings.Contains(body, "<!doctype html>") {
		t.Error("Response should contain HTML")
	}
	if !strings.Contains(body, "Zero-Trust Gateway") {
		t.Error("Response should contain login page title")
	}
}

func TestHandleLoginPOSTSuccess(t *testing.T) {
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
		t.Errorf("Status = %d, want %d", w.Code, http.StatusFound)
	}

	// Check for redirect
	location := w.Header().Get("Location")
	if location != "/" {
		t.Errorf("Location = %s, want /", location)
	}

	// Check for cookie
	cookies := w.Result().Cookies()
	found := false
	for _, cookie := range cookies {
		if cookie.Name == "auth_token" {
			found = true
			if cookie.Value == "" {
				t.Error("Cookie value should not be empty")
			}
			if cookie.Path != "/" {
				t.Errorf("Cookie path = %s, want /", cookie.Path)
			}
		}
	}
	if !found {
		t.Error("auth_token cookie should be set")
	}
}

func TestHandleLoginPOSTFailure(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	insertTestUser(t, db, "user", "correctpass", 0)

	tests := []struct {
		name     string
		username string
		password string
	}{
		{
			name:     "wrong password",
			username: "user",
			password: "wrongpass",
		},
		{
			name:     "nonexistent user",
			username: "nobody",
			password: "pass",
		},
		{
			name:     "empty username",
			username: "",
			password: "pass",
		},
		{
			name:     "empty password",
			username: "user",
			password: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form := url.Values{}
			form.Set("username", tt.username)
			form.Set("password", tt.password)

			req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()

			HandleLogin(w, req, db)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("Status = %d, want %d", w.Code, http.StatusUnauthorized)
			}

			body := w.Body.String()
			if !strings.Contains(body, "Invalid credentials") {
				t.Error("Response should contain error message")
			}
		})
	}
}

func TestAuthenticateUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	insertTestUser(t, db, "testuser", "password123", 1)

	result := &AuthResult{}
	authenticated := authenticateUser(db, "testuser", "password123", result)

	if !authenticated {
		t.Error("Should authenticate with correct credentials")
	}
	if result.Username != "testuser" {
		t.Errorf("Username = %s, want testuser", result.Username)
	}
	if result.UserID == 0 {
		t.Error("UserID should be set")
	}
	if !result.ZeroTrustReq {
		t.Error("ZeroTrustReq should be true")
	}
}

func TestValidateSession(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	insertTestUser(t, db, "sessionuser", "pass", 1)

	var userID int
	db.QueryRow("SELECT id FROM users WHERE username = ?", "sessionuser").Scan(&userID)

	token := "valid_token_abc"
	_, err := db.Exec(`INSERT INTO user_sessions (user_id, token, expires_at) VALUES (?, ?, datetime('now', '+1 hour'))`,
		userID, token)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	result := &AuthResult{}
	valid := validateSession(db, token, result)

	if !valid {
		t.Error("Should validate correct session")
	}
	if result.Username != "sessionuser" {
		t.Errorf("Username = %s, want sessionuser", result.Username)
	}
	if !result.ZeroTrustReq {
		t.Error("ZeroTrustReq should be true")
	}
	if result.SessionToken != token {
		t.Errorf("SessionToken = %s, want %s", result.SessionToken, token)
	}
}

func TestCreateSession(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	insertTestUser(t, db, "newuser", "pass", 0)

	token := createSession(db, "newuser")

	if token == "" {
		t.Error("Token should not be empty")
	}

	// Verify session was created in database
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM user_sessions WHERE token = ?", token).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query sessions: %v", err)
	}
	if count != 1 {
		t.Errorf("Session count = %d, want 1", count)
	}

	// Verify expiration is set
	var expiresAt string
	err = db.QueryRow("SELECT expires_at FROM user_sessions WHERE token = ?", token).Scan(&expiresAt)
	if err != nil {
		t.Fatalf("Failed to get expiration: %v", err)
	}
	if expiresAt == "" {
		t.Error("Expiration should be set")
	}
}

func TestAuthResultFields(t *testing.T) {
	result := &AuthResult{
		Authenticated: true,
		Username:      "testuser",
		UserID:        42,
		ZeroTrustReq:  true,
		SessionToken:  "token123",
	}

	if !result.Authenticated {
		t.Error("Authenticated should be true")
	}
	if result.Username != "testuser" {
		t.Errorf("Username = %s, want testuser", result.Username)
	}
	if result.UserID != 42 {
		t.Errorf("UserID = %d, want 42", result.UserID)
	}
	if !result.ZeroTrustReq {
		t.Error("ZeroTrustReq should be true")
	}
	if result.SessionToken != "token123" {
		t.Errorf("SessionToken = %s, want token123", result.SessionToken)
	}
}

func TestConcurrentAuthentication(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	insertTestUser(t, db, "concurrent", "pass", 0)

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			req := httptest.NewRequest("GET", "/", nil)
			req.SetBasicAuth("concurrent", "pass")
			result := Check(req, db)
			if !result.Authenticated {
				t.Error("Should be authenticated")
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}