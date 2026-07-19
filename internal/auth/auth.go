package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"

	dbstore "netgoat.xyz/agent/internal/database"
)

type AuthResult struct {
	Authenticated bool
	Username      string
	UserID        int
	ZeroTrustReq  bool
	SessionToken  string // Set only when an existing cookie session is authenticated.
}

func Check(r *http.Request, db *sql.DB) *AuthResult {
	result := &AuthResult{Authenticated: false}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		cookie, err := r.Cookie("auth_token")
		if err == nil && cookie.Value != "" {
			if validateSession(db, cookie.Value, result) {
				result.Authenticated = true
				return result
			}
		}
		return result
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Basic" {
		return result
	}

	payload, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return result
	}
	pair := strings.SplitN(string(payload), ":", 2)
	if len(pair) != 2 {
		return result
	}

	username := pair[0]
	password := pair[1]

	if authenticateUser(db, username, password, result) {
		result.Authenticated = true
	}

	return result
}

func authenticateUser(db *sql.DB, username, password string, result *AuthResult) bool {
	var hash string
	var userID int
	var zeroTrustEnabled int

	err := db.QueryRow(
		"SELECT id, password_hash, zero_trust_enabled FROM users WHERE username = ?",
		username).Scan(&userID, &hash, &zeroTrustEnabled)

	if err != nil {
		log.Warn().Str("user", username).Msg("Login failed: user not found")
		return false
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		log.Warn().Str("user", username).Msg("Login failed: bad password")
		return false
	}

	result.Username = username
	result.UserID = userID
	result.ZeroTrustReq = zeroTrustEnabled == 1
	return true
}

func validateSession(db *sql.DB, sessionToken string, result *AuthResult) bool {
	var username string
	var userID int
	var zeroTrustEnabled int

	err := db.QueryRow(
		"SELECT id, username, zero_trust_enabled FROM users WHERE id = (SELECT user_id FROM user_sessions WHERE token = ? AND expires_at > datetime('now'))",
		sessionToken).Scan(&userID, &username, &zeroTrustEnabled)

	if err != nil {
		return false
	}

	result.Username = username
	result.UserID = userID
	result.ZeroTrustReq = zeroTrustEnabled == 1
	result.SessionToken = sessionToken
	return true
}

func createSession(db *sql.DB, username string) string {
	var userID int
	if err := db.QueryRow("SELECT id FROM users WHERE username = ?", username).Scan(&userID); err != nil {
		log.Warn().Err(err).Str("user", username).Msg("Failed to create session: user not found")
		return ""
	}
	if _, err := dbstore.PruneExpiredSessions(db); err != nil {
		// Cleanup is maintenance and should not turn valid credentials into an
		// authentication outage. The insert below still surfaces failures that
		// prevent creation of the requested session.
		log.Warn().Err(err).Msg("Failed to prune expired sessions")
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		log.Error().Err(err).Msg("Failed to create session token")
		return ""
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)

	if _, err := db.Exec(`
		INSERT INTO user_sessions (user_id, token, expires_at)
		VALUES (?, ?, datetime('now', '+24 hours'))`,
		userID, token); err != nil {
		log.Error().Err(err).Str("user", username).Msg("Failed to store session")
		return ""
	}

	return token
}

func HandleLogin(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method == "GET" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(` <!doctype html><html lang="en"><head><meta charset="UTF-8" /><meta name="viewport" content="width=device-width, initial-scale=1.0" /><title>Zero-Trust Gateway</title><script src="https://cdn.tailwindcss.com"></script></head><body class="flex min-h-screen items-center justify-center bg-zinc-950 font-sans text-zinc-200"><div class="w-full max-w-md rounded-xl border border-zinc-800 bg-zinc-900 p-8 shadow-2xl"><div class="mb-8 text-center"><div class="mb-4 inline-flex items-center"><img src="https://netgoat.sirv.com/Images/Public_Relations/netgoat_with_text.png" width="150" height="500" alt=""></div><h1 class="text-2xl font-bold tracking-tight text-white">Access Verification</h1><p class="mt-2 text-sm text-zinc-400">Continuous authentication active</p></div><form class="space-y-5" method="POST" action="/login"><div><label class="mb-1 block text-xs font-medium text-zinc-400 uppercase">Corporate ID</label><input placeholder="ducky" class="w-full rounded-lg border border-zinc-700 bg-zinc-950 px-4 py-3 transition-all outline-none placeholder:text-zinc-600 focus:border-transparent focus:ring-2 focus:ring-indigo-500" name="username"/></div><div><label class="mb-1 block text-xs font-medium text-zinc-400 uppercase">Access Token</label><input type="password" placeholder="••••••••" class="w-full rounded-lg border border-zinc-700 bg-zinc-950 px-4 py-3 transition-all outline-none placeholder:text-zinc-600 focus:border-transparent focus:ring-2 focus:ring-indigo-500" name="password" /></div><button type="submit" class="w-full rounded-lg bg-indigo-600 py-3 font-semibold text-white transition-all hover:bg-indigo-500 active:scale-[0.98]">Authorize Session</button></form><div class="mt-8 border-t border-zinc-800 pt-6 text-center"><p class="text-xs leading-relaxed text-zinc-500">By attempting access, you agree to the <br /><span class="text-indigo-400">Least Privilege Policy</span>. All actions logged.</p></div></div></body></html> `))
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid login request", http.StatusBadRequest)
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

	token := createSession(db, username)
	if token == "" {
		http.Error(w, "Unable to create session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    token,
		Path:     "/",
		MaxAge:   24 * 60 * 60,
		Expires:  time.Now().Add(24 * time.Hour).UTC(),
		Secure:   r.TLS != nil,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	http.Redirect(w, r, "/", http.StatusFound)
}

func RequireZeroTrustChallenge(result *AuthResult, globalEnabled bool, verified bool) bool {
	if result == nil || !result.Authenticated {
		return false
	}
	return globalEnabled && result.ZeroTrustReq && !verified
}
