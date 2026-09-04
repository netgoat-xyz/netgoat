package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"netgoat.xyz/agent/internal/auth"
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

func TestZeroTrustPinnedSkipAllowsRequest(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	directory := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeZeroTrustDirectory(t, w, public)
	}))
	t.Cleanup(directory.Close)

	now := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	verifier, err := challenge.NewBotAuthVerifier(challenge.BotAuthConfig{
		Enabled:           true,
		PinnedDirectories: []string{directory.URL},
		HTTPClient:        directory.Client(),
		CacheTTL:          time.Minute,
		Now:               func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	store := challenge.NewStore(
		challenge.WithSecret([]byte("test-secret")),
		challenge.WithBotAuth(verifier),
		challenge.WithNow(func() time.Time { return now }),
		challenge.WithDifficulty(8, 8, 4),
	)

	req := httptest.NewRequest(http.MethodGet, "https://app.example.test/private", nil)
	req.Host = "app.example.test"
	req.TLS = &tls.ConnectionState{HandshakeComplete: true}
	attachZeroTrustWebBotAuth(t, req, private, ed25519Thumbprint(t, public), directory.URL, now)

	binding := challenge.BindingFromRequest(req)
	binding.Subject = zeroTrustSubject(&auth.AuthResult{Authenticated: true, UserID: 1})
	result := &auth.AuthResult{Authenticated: true, UserID: 1, Username: "agent", ZeroTrustReq: true}

	if zeroTrustChallengeNeeded(store, req, result, true, binding) {
		t.Fatal("pinned web-bot-auth skip must not enter the zero-trust challenge write path")
	}
	if !store.IsVerified(binding) {
		t.Fatal("skip should mark the terminated session verified")
	}

	unsigned := httptest.NewRequest(http.MethodGet, "https://app.example.test/private", nil)
	unsigned.Host = "app.example.test"
	unsigned.TLS = req.TLS
	unpaid := challenge.SessionBinding{SessionID: "other-session", Terminated: true, Subject: "user:1"}
	if !zeroTrustChallengeNeeded(store, unsigned, result, true, unpaid) {
		t.Fatal("unsigned zero-trust traffic on another session must still be challenged")
	}
}

func writeZeroTrustDirectory(t *testing.T, w http.ResponseWriter, public ed25519.PublicKey) {
	t.Helper()
	x := base64.RawURLEncoding.EncodeToString(public)
	kid := ed25519Thumbprint(t, public)
	body, err := json.Marshal(map[string]any{
		"keys": []map[string]string{{
			"kty": "OKP",
			"crv": "Ed25519",
			"x":   x,
			"kid": kid,
			"alg": "EdDSA",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	w.Header().Set("Content-Type", "application/http-message-signatures-directory+json")
	_, _ = w.Write(body)
}

func ed25519Thumbprint(t *testing.T, public ed25519.PublicKey) string {
	t.Helper()
	x := base64.RawURLEncoding.EncodeToString(public)
	canonical := fmt.Sprintf(`{"crv":"Ed25519","kty":"OKP","x":%q}`, x)
	sum := sha256.Sum256([]byte(canonical))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func attachZeroTrustWebBotAuth(t *testing.T, req *http.Request, private ed25519.PrivateKey, keyID, agentURL string, now time.Time) {
	t.Helper()
	req.Header.Set("Signature-Agent", `"`+escapeSF(agentURL)+`"`)
	created := now.Unix()
	expires := now.Add(2 * time.Minute).Unix()
	inner := fmt.Sprintf(`("@authority" "signature-agent");created=%d;keyid="%s";alg="ed25519";expires=%d;tag="web-bot-auth"`,
		created, escapeSF(keyID), expires)
	authority := strings.ToLower(req.Host)
	base := fmt.Sprintf("\"@authority\": %s\n\"signature-agent\": %s\n\"@signature-params\": %s",
		authority, req.Header.Get("Signature-Agent"), inner)
	signature := ed25519.Sign(private, []byte(base))
	req.Header.Set("Signature-Input", "sig="+inner)
	req.Header.Set("Signature", "sig=:"+base64.StdEncoding.EncodeToString(signature)+":")
}

func escapeSF(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `"`, `\"`)
}
