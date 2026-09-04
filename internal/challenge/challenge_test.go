package challenge

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"netgoat.xyz/agent/internal/fingerprint"
)

var testSecret = []byte("challenge-test-secret-32-bytes!!")

func testStore(cfg storeConfig) *Store {
	if len(cfg.secret) == 0 {
		cfg.secret = testSecret
	}
	if cfg.baseDifficulty == 0 {
		cfg.baseDifficulty = 8
	}
	if cfg.maxDifficulty == 0 {
		cfg.maxDifficulty = 16
	}
	return newStore(cfg)
}

func terminated(sessionID string) SessionBinding {
	return SessionBinding{SessionID: sessionID, Terminated: true}
}

func solveChallenge(t *testing.T, ch *Challenge) string {
	t.Helper()
	if ch == nil {
		t.Fatal("challenge is nil")
	}
	mac, ok := decodeMAC(ch.MAC)
	if !ok {
		t.Fatalf("invalid mac %q", ch.MAC)
	}
	for counter := uint64(0); counter < 1<<24; counter++ {
		if leadingZeroBits(powDigest(mac, counter)) >= ch.Difficulty {
			return strconvFormat(counter)
		}
	}
	t.Fatal("failed to solve PoW")
	return ""
}

func strconvFormat(v uint64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

func TestNewStore(t *testing.T) {
	store := NewStore(WithSecret(testSecret), WithDifficulty(8, 16, 4))
	if store == nil {
		t.Fatal("NewStore returned nil")
	}
	if store.challenges == nil {
		t.Error("challenges map should be initialized")
	}
	if store.verified == nil {
		t.Error("verified map should be initialized")
	}
}

func TestGenerateID(t *testing.T) {
	id1 := GenerateID()
	id2 := GenerateID()
	if id1 == "" || id2 == "" {
		t.Error("GenerateID should not return empty string")
	}
	if id1 == id2 {
		t.Error("GenerateID should return unique IDs")
	}
	if len(id1) != 22 {
		t.Errorf("GenerateID length = %d, want 22", len(id1))
	}
}

func TestAgentUserAgentsDoNotRaiseDifficulty(t *testing.T) {
	store := testStore(storeConfig{})
	binding := terminated("session-a")
	base := store.DifficultyFor(binding)

	// DifficultyFor has no User-Agent argument. These strings used to add +30
	// suspicion and must not exist in the cost path.
	for _, ua := range []string{
		"python-requests/2.26.0",
		"Go-http-client/1.1",
		"Googlebot/2.1 (+http://www.google.com/bot.html)",
		"curl/8.0.0",
	} {
		if strings.Contains(strings.ToLower(ua), "python") || strings.Contains(strings.ToLower(ua), "go-http-client") || strings.Contains(strings.ToLower(ua), "bot") {
			if store.DifficultyFor(binding) != base {
				t.Fatalf("difficulty changed while inspecting UA %q", ua)
			}
		}
	}
	if base != store.baseDifficulty {
		t.Fatalf("calm difficulty = %d, want base %d", base, store.baseDifficulty)
	}
}

func TestStoreCreateRequiresTerminatedSession(t *testing.T) {
	store := testStore(storeConfig{})
	if ch := store.Create(SessionBinding{SessionID: "ghost", Terminated: false}); ch != nil {
		t.Fatal("pass-through / HTTP-only must not issue PoW")
	}
	if ch := store.Create(SessionBinding{Terminated: true}); ch != nil {
		t.Fatal("terminated session without SessionID must not issue PoW")
	}
	ch := store.Create(terminated("session-a"))
	if ch == nil {
		t.Fatal("terminated session should receive PoW")
	}
	if ch.Type != ChallengePoW {
		t.Errorf("type = %q, want %q", ch.Type, ChallengePoW)
	}
	if ch.Nonce == "" || ch.MAC == "" || ch.Difficulty <= 0 || ch.Expires == 0 {
		t.Fatalf("incomplete wire puzzle: %+v", ch)
	}
}

func TestStoreGet(t *testing.T) {
	store := testStore(storeConfig{})
	created := store.Create(terminated("session-a"))
	retrieved, ok := store.Get(created.ID)
	if !ok || retrieved == nil || retrieved.ID != created.ID {
		t.Fatal("Get should return the issued puzzle")
	}
	if _, ok := store.Get("nonexistentnonce123456"); ok {
		t.Fatal("Get should miss unknown ids")
	}
}

func TestStoreVerifyAndSessionIsolation(t *testing.T) {
	store := testStore(storeConfig{})
	sessionA := terminated("session-a")
	sessionB := terminated("session-b")
	ch := store.Create(sessionA)
	counter := solveChallenge(t, ch)

	if store.Verify(sessionB, ch.Nonce, counter) {
		t.Fatal("verify bound to session A must fail on session B")
	}
	if store.IsVerified(sessionB) {
		t.Fatal("session B must not inherit session A verification")
	}
	if !store.Verify(sessionA, ch.Nonce, counter) {
		t.Fatal("session A should verify its own puzzle")
	}
	if !store.IsVerified(sessionA) {
		t.Fatal("session A should be marked verified")
	}
	if store.IsVerified(sessionB) {
		t.Fatal("session B should still be unverified")
	}
}

func TestSameNonceCannotVerifyTwice(t *testing.T) {
	store := testStore(storeConfig{})
	binding := terminated("session-a")
	ch := store.Create(binding)
	counter := solveChallenge(t, ch)
	if !store.Verify(binding, ch.Nonce, counter) {
		t.Fatal("first verify should succeed")
	}
	if store.Verify(binding, ch.Nonce, counter) {
		t.Fatal("same nonce cannot verify twice")
	}
}

func TestExpiredExpiresFailsEvenIfCounterCorrect(t *testing.T) {
	now := time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC)
	store := testStore(storeConfig{now: func() time.Time { return now }})
	binding := terminated("session-a")
	ch := store.Create(binding)
	counter := solveChallenge(t, ch)
	now = time.Unix(ch.Expires, 0)
	if store.Verify(binding, ch.Nonce, counter) {
		t.Fatal("expired expires must fail even with a correct counter")
	}
}

func TestHMACWithoutExpiresFailsTheSuite(t *testing.T) {
	secret := testSecret
	expires := int64(1770000000)
	full := computeCommitment(secret, "session-a", "nonce-a", 16, expires)

	truncated := hmac.New(sha256.New, secret)
	writeLenPrefixed(truncated, "session-a")
	writeLenPrefixed(truncated, "nonce-a")
	var meta [4]byte
	binary.BigEndian.PutUint32(meta[:], 16)
	_, _ = truncated.Write(meta[:])
	if hmac.Equal(full, truncated.Sum(nil)) {
		t.Fatal("commitment HMAC must bind expires_unix; omitting it is a replay bug")
	}

	store := testStore(storeConfig{})
	binding := terminated("session-a")
	ch := store.Create(binding)
	mac, ok := decodeMAC(ch.MAC)
	if !ok {
		t.Fatal("issued mac should decode")
	}
	replay := computeCommitmentWithoutExpires(store.secret, binding.SessionID, ch.Nonce, ch.Difficulty)
	if hmac.Equal(mac, replay) {
		t.Fatal("issued mac included expires and must not match a truncated HMAC")
	}
}

func computeCommitmentWithoutExpires(secret []byte, sessionID, nonce string, difficulty int) []byte {
	mac := hmac.New(sha256.New, secret)
	writeLenPrefixed(mac, sessionID)
	writeLenPrefixed(mac, nonce)
	var meta [4]byte
	binary.BigEndian.PutUint32(meta[:], uint32(difficulty))
	_, _ = mac.Write(meta[:])
	return mac.Sum(nil)
}

func TestWrongCounterFails(t *testing.T) {
	store := testStore(storeConfig{})
	binding := terminated("session-a")
	ch := store.Create(binding)
	if store.Verify(binding, ch.Nonce, "nope") {
		t.Fatal("non-numeric counter must fail")
	}
	if store.Verify(binding, ch.Nonce, strings.Repeat("9", maxAnswerBytes+1)) {
		t.Fatal("oversized counter must fail")
	}
}

func TestLoadBumpIncreasesDifficulty(t *testing.T) {
	now := time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC)
	store := testStore(storeConfig{
		now:            func() time.Time { return now },
		baseDifficulty: 8,
		maxDifficulty:  24,
		maxChallenges:  64,
	})
	binding := terminated("session-a")
	calm := store.DifficultyFor(binding)
	for i := 0; i < 32; i++ {
		if store.Create(terminated("session-"+strconvFormat(uint64(i)))) == nil {
			t.Fatal("Create returned nil")
		}
	}
	loaded := store.DifficultyFor(binding)
	if loaded <= calm {
		t.Fatalf("load bump did not increase difficulty: calm=%d loaded=%d", calm, loaded)
	}
}

func TestMismatchBumpIndependentOfUA(t *testing.T) {
	store := testStore(storeConfig{baseDifficulty: 8, maxDifficulty: 24, mismatchBump: 4})
	matched := store.DifficultyFor(terminated("session-a"))
	mismatched := store.DifficultyFor(SessionBinding{SessionID: "session-a", Terminated: true, StackClassMismatch: true})
	if mismatched-matched != store.mismatchBump {
		t.Fatalf("mismatch bump = %d, want %d (independent of UA)", mismatched-matched, store.mismatchBump)
	}
}

func TestStackClassMismatchWiresFromFingerprint(t *testing.T) {
	store := testStore(storeConfig{baseDifficulty: 8, maxDifficulty: 24, mismatchBump: 4})
	chromeUA := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	goClass := "t13d0608h2_aaaaaaaaaaaa_bbbbbbbbbbbb|-|h2,http/1.1"
	chromeClass := "t13d1516h2_8daaf6152771_b186095e22b6|1:65536|0|0|m,a,s,p|h2,http/1.1"

	disagree := tlsRequest(chromeUA, goClass)
	agree := tlsRequest(chromeUA, chromeClass)
	honest := tlsRequest("Go-http-client/1.1", goClass)
	missing := tlsRequest(chromeUA, "")

	if !BindingFromRequest(disagree).StackClassMismatch {
		t.Fatal("Go stack claiming Chrome must set StackClassMismatch")
	}
	if BindingFromRequest(agree).StackClassMismatch {
		t.Fatal("matching Chrome stack must not set StackClassMismatch")
	}
	if BindingFromRequest(honest).StackClassMismatch {
		t.Fatal("honest library UA must not set StackClassMismatch")
	}
	if !BindingFromRequest(missing).StackClassMismatch {
		t.Fatal("missing stack with browser UA must set StackClassMismatch")
	}

	base := store.DifficultyFor(BindingFromRequest(agree))
	bumped := store.DifficultyFor(BindingFromRequest(disagree))
	if bumped-base != store.mismatchBump {
		t.Fatalf("wired mismatch bump = %d, want %d", bumped-base, store.mismatchBump)
	}
	if store.DifficultyFor(BindingFromRequest(honest)) != base {
		t.Fatal("honest library UA must not raise difficulty")
	}
}

func TestZeroTrustSubjectSeparatesVerifiedSet(t *testing.T) {
	store := testStore(storeConfig{})
	user1 := SessionBinding{SessionID: "tls-conn", Terminated: true, Subject: "user:1"}
	user2 := SessionBinding{SessionID: "tls-conn", Terminated: true, Subject: "user:2"}
	ch := store.Create(user1)
	counter := solveChallenge(t, ch)
	if store.Verify(user2, ch.Nonce, counter) {
		t.Fatal("user 2 must not spend user 1's puzzle")
	}
	if !store.Verify(user1, ch.Nonce, counter) {
		t.Fatal("user 1 should verify")
	}
	if store.IsVerified(user2) || !store.IsVerified(user1) {
		t.Fatal("verified set must be subject-scoped, not IP")
	}
}

func TestBindingFromRequestRequiresTLS(t *testing.T) {
	req := httptestRequest()
	binding := BindingFromRequest(req)
	if binding.Terminated || binding.SessionID != "" {
		t.Fatalf("HTTP-only request issued a session binding: %+v", binding)
	}

	req.TLS = &tls.ConnectionState{HandshakeComplete: true}
	binding = BindingFromRequest(req)
	if !binding.Terminated || binding.SessionID == "" {
		t.Fatalf("terminated TLS should mint a session id: %+v", binding)
	}
	if binding.StackClassMismatch {
		t.Fatal("empty UA must not set StackClassMismatch")
	}

	ctx := WithConnSessionID(context.Background())
	req = req.WithContext(ctx)
	first := BindingFromRequest(req)
	second := BindingFromRequest(req)
	if first.SessionID == "" || first.SessionID != second.SessionID {
		t.Fatalf("ConnContext session id was not stable: %q / %q", first.SessionID, second.SessionID)
	}

	std := httptest.NewRequest(http.MethodGet, "https://example.test/", nil)
	std.TLS = &tls.ConnectionState{HandshakeComplete: true}
	if got := BindingFromRequest(std); !got.Terminated || got.SessionID == "" {
		t.Fatalf("httptest request with TLS should terminate: %+v", got)
	}
}

func TestChallengeWireJSON(t *testing.T) {
	store := testStore(storeConfig{})
	ch := store.Create(terminated("session-a"))
	raw, err := json.Marshal(ch.Wire())
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"nonce", "difficulty", "expires", "mac"} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("wire JSON missing %q: %s", key, raw)
		}
	}
}

func TestRenderDropsPuzzleWidgets(t *testing.T) {
	store := testStore(storeConfig{})
	ch := store.Create(terminated("session-a"))
	html := RenderDynamicErrorPage(ch, 403, "Forbidden")
	for _, forbidden := range []string{"Click Verification", "Puzzle Verification", "Type the word", "sunrise", "challenge_id"} {
		if strings.Contains(html, forbidden) {
			t.Fatalf("render still contains puzzle UI %q", forbidden)
		}
	}
	if !strings.Contains(html, "nonce") || !strings.Contains(html, "difficulty") {
		t.Fatal("browser page should embed the JSON puzzle")
	}
	payload := string(RenderChallengeJSON(ch, 403, "Forbidden"))
	if !strings.Contains(payload, `"nonce"`) || !strings.Contains(payload, `"mac"`) {
		t.Fatalf("JSON wire = %s", payload)
	}
	simple := RenderDynamicErrorPage(nil, 502, "Bad Gateway")
	if strings.Contains(simple, "proof-of-work") {
		t.Fatal("simple error should not issue PoW")
	}
}

func TestStoreVerifyRemovesChallengeOnSuccess(t *testing.T) {
	store := testStore(storeConfig{})
	binding := terminated("session-a")
	ch := store.Create(binding)
	if !store.Verify(binding, ch.Nonce, solveChallenge(t, ch)) {
		t.Fatal("verify should succeed")
	}
	if _, ok := store.Get(ch.ID); ok {
		t.Fatal("nonce should be burned after success")
	}
}

func TestStoreIsVerifiedExpiration(t *testing.T) {
	now := time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC)
	store := testStore(storeConfig{now: func() time.Time { return now }})
	binding := terminated("session-a")
	ch := store.Create(binding)
	if !store.Verify(binding, ch.Nonce, solveChallenge(t, ch)) {
		t.Fatal("verify should succeed")
	}
	now = now.Add(defaultVerificationTTL)
	if store.IsVerified(binding) {
		t.Fatal("session verification should expire")
	}
}

func TestChallengeExpirationIsShort(t *testing.T) {
	store := testStore(storeConfig{})
	ch := store.Create(terminated("session-a"))
	if ch.ExpiresAt.After(time.Now().Add(3 * time.Minute)) {
		t.Fatal("challenge TTL should be seconds to a couple of minutes")
	}
	if defaultVerificationTTL >= time.Hour {
		t.Fatal("verified TTL must be shorter than the old 1-hour IP cookie")
	}
}

func TestChallengeTypes(t *testing.T) {
	for _, ctype := range []ChallengeType{ChallengeNone, ChallengePoW} {
		if string(ctype) == "" {
			t.Errorf("Challenge type %v should have string representation", ctype)
		}
	}
}

func httptestRequest() *http.Request {
	req, _ := http.NewRequest(http.MethodGet, "http://example.test/", nil)
	return req
}

func tlsRequest(userAgent, stackClass string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "https://example.test/", nil)
	req.TLS = &tls.ConnectionState{HandshakeComplete: true}
	req.Header.Set("User-Agent", userAgent)
	if stackClass != "" {
		req = req.WithContext(fingerprint.WithStackClass(req.Context(), stackClass))
	}
	return req
}
