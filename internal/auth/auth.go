package auth

import (
	"database/sql"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

func Check(r *http.Request) bool {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		cookie, err := r.Cookie("auth_token")
		if err == nil && cookie.Value == "valid_session" {
			return true
		}
		return false
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Basic" {
		return false
	}

	payload, _ := base64.StdEncoding.DecodeString(parts[1])
	pair := strings.SplitN(string(payload), ":", 2)
	if len(pair) != 2 {
		return false
	}

	return false
}

func HandleLogin(w http.ResponseWriter, r *http.Request, db *sql.DB) {
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

	http.SetCookie(w, &http.Cookie{
		Name:  "auth_token",
		Value: "valid_session",
		Path:  "/",
	})

	http.Redirect(w, r, "/", http.StatusFound)
}
