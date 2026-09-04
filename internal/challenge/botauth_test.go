package challenge

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestUnpinnedEd25519DoesNotSkip(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var fetches atomic.Int32
	unpinned := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetches.Add(1)
		writeDirectory(t, w, public)
	}))
	t.Cleanup(unpinned.Close)

	pinned := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("pinned directory should not be fetched for an unpinned agent")
	}))
	t.Cleanup(pinned.Close)

	verifier, err := NewBotAuthVerifier(BotAuthConfig{
		Enabled:           true,
		PinnedDirectories: []string{pinned.URL},
		HTTPClient:        pinned.Client(),
		Now:               func() time.Time { return time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "https://app.example.test/resource", nil)
	req.Host = "app.example.test"
	now := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	attachWebBotAuth(t, req, private, jwkThumbprint(t, public), unpinned.URL, now)

	if verifier.Skip(req) {
		t.Fatal("unpinned Ed25519 signature must not skip")
	}
	if got := fetches.Load(); got != 0 {
		t.Fatalf("unpinned Signature-Agent was fetched %d times", got)
	}
}

func TestPinnedWebBotAuthSkips(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var fetches atomic.Int32
	directory := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetches.Add(1)
		writeDirectory(t, w, public)
	}))
	t.Cleanup(directory.Close)

	now := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	verifier, err := NewBotAuthVerifier(BotAuthConfig{
		Enabled:           true,
		PinnedDirectories: []string{directory.URL},
		HTTPClient:        directory.Client(),
		CacheTTL:          time.Minute,
		Now:               func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "https://app.example.test/resource", nil)
	req.Host = "app.example.test"
	attachWebBotAuth(t, req, private, jwkThumbprint(t, public), directory.URL, now)
	if !verifier.Skip(req) {
		t.Fatal("pinned directory + valid web-bot-auth signature should skip")
	}

	second := httptest.NewRequest(http.MethodGet, "https://app.example.test/other", nil)
	second.Host = "app.example.test"
	attachWebBotAuth(t, second, private, jwkThumbprint(t, public), directory.URL, now)
	if !verifier.Skip(second) {
		t.Fatal("cached pinned directory should still skip")
	}
	if got := fetches.Load(); got != 1 {
		t.Fatalf("directory fetches = %d, want 1 (cached)", got)
	}
}

func TestMissingSignatureDoesNotSkip(t *testing.T) {
	verifier, err := NewBotAuthVerifier(BotAuthConfig{Enabled: true, PinnedDirectories: []string{"https://example.invalid/.well-known/web-bot-auth"}})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "https://app.example.test/", nil)
	if verifier.Skip(req) {
		t.Fatal("missing signature must fail open to PoW, not skip")
	}
}

func TestEmptyPinListNeverSkips(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	directory := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("empty pin list must never fetch Signature-Agent URLs")
	}))
	t.Cleanup(directory.Close)

	verifier, err := NewBotAuthVerifier(BotAuthConfig{Enabled: true, PinnedDirectories: nil, HTTPClient: directory.Client()})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "https://app.example.test/", nil)
	req.Host = "app.example.test"
	attachWebBotAuth(t, req, private, jwkThumbprint(t, public), directory.URL, time.Now())
	if verifier.Skip(req) {
		t.Fatal("empty pin list must not skip")
	}
}

func writeDirectory(t *testing.T, w http.ResponseWriter, public ed25519.PublicKey) {
	t.Helper()
	x := base64.RawURLEncoding.EncodeToString(public)
	kid := jwkThumbprint(t, public)
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

func jwkThumbprint(t *testing.T, public ed25519.PublicKey) string {
	t.Helper()
	x := base64.RawURLEncoding.EncodeToString(public)
	thumbprint, err := ed25519JWKThumbprint(x)
	if err != nil {
		t.Fatal(err)
	}
	return thumbprint
}

func attachWebBotAuth(t *testing.T, req *http.Request, private ed25519.PrivateKey, keyID, agentURL string, now time.Time) {
	t.Helper()
	req.Header.Set("Signature-Agent", quoteSFString(agentURL))
	created := now.Unix()
	expires := now.Add(2 * time.Minute).Unix()
	inner := fmt.Sprintf(`("@authority" "signature-agent");created=%d;keyid=%s;alg="ed25519";expires=%d;tag="web-bot-auth"`,
		created, quoteSFString(keyID), expires)
	authority := requestAuthority(req)
	base := fmt.Sprintf("\"@authority\": %s\n\"signature-agent\": %s\n\"@signature-params\": %s",
		authority, req.Header.Get("Signature-Agent"), inner)
	signature := ed25519.Sign(private, []byte(base))
	req.Header.Set("Signature-Input", "sig="+inner)
	req.Header.Set("Signature", "sig=:"+base64.StdEncoding.EncodeToString(signature)+":")
}
