package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"crypto/tls"

	"netgoat.xyz/agent/internal/challenge"
)

func TestWriteZeroTrustChallenge(t *testing.T) {
	store := challenge.NewStore(challenge.WithSecret([]byte("test-secret")), challenge.WithDifficulty(8, 8, 4))
	req := httptest.NewRequest(http.MethodGet, "http://example.com/private", nil)
	req.RemoteAddr = "203.0.113.44:12345"
	rr := httptest.NewRecorder()

	writeZeroTrustChallenge(rr, store, req, challenge.SessionBinding{SessionID: "tls-conn", Terminated: false, Subject: "user:1"})

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("content-type = %q, want contains text/html", ct)
	}
	body := rr.Body.String()
	if strings.Contains(body, `"nonce"`) || strings.Contains(body, "proof-of-work") {
		t.Fatal("unterminated TLS must not issue PoW")
	}
}

func TestWriteZeroTrustChallengeIssuesPoWWhenTerminated(t *testing.T) {
	store := challenge.NewStore(challenge.WithSecret([]byte("test-secret")), challenge.WithDifficulty(8, 8, 4))
	req := httptest.NewRequest(http.MethodGet, "https://example.com/private", nil)
	req.TLS = &tls.ConnectionState{HandshakeComplete: true}
	rr := httptest.NewRecorder()

	binding := challenge.SessionBinding{SessionID: "tls-conn", Terminated: true, Subject: "user:1"}
	writeZeroTrustChallenge(rr, store, req, binding)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "nonce") || !strings.Contains(body, "difficulty") {
		t.Fatalf("terminated TLS should render the JSON PoW puzzle, got %q", body)
	}
	if strings.Contains(body, "Click Verification") || strings.Contains(body, "Type the word") {
		t.Fatal("puzzle HTML must be gone")
	}
}
