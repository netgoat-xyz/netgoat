package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"netgoat.xyz/agent/internal/challenge"
)

func TestWriteZeroTrustChallenge(t *testing.T) {
	store := challenge.NewStore()
	req := httptest.NewRequest(http.MethodGet, "http://example.com/private", nil)
	req.RemoteAddr = "203.0.113.44:12345"
	rr := httptest.NewRecorder()

	writeZeroTrustChallenge(rr, store, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("content-type = %q, want contains text/html", ct)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Verification Required") {
		t.Fatal("body should explain that verification is required")
	}
	if !strings.Contains(body, `name="challenge_id"`) {
		t.Fatal("body should include a challenge form")
	}
}
