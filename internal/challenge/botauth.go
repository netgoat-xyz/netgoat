package challenge

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	defaultJWKSCacheTTL        = 5 * time.Minute
	defaultBotAuthFetchTimeout = 5 * time.Second
	maxDirectoryBytes          = 1 << 20
	maxDirectoryKeys           = 64
	maxSignatureLifetime       = 5 * time.Minute
	botAuthClockSkew           = 30 * time.Second
)

// BotAuthConfig is the skip-lane verifier. Only operator-pinned directory
// URLs are fetched. An empty pin list means the lane never skips.
type BotAuthConfig struct {
	Enabled           bool
	PinnedDirectories []string
	CacheTTL          time.Duration
	HTTPClient        *http.Client
	Now               func() time.Time
}

type directoryKey struct {
	keyID      string
	thumbprint string
	public     ed25519.PublicKey
}

type cachedDirectory struct {
	keys      []directoryKey
	expiresAt time.Time
}

// BotAuthVerifier checks RFC 9421 web-bot-auth signatures against a pinned
// JWKS directory allowlist. Missing or unpinned signatures fail open.
type BotAuthVerifier struct {
	enabled bool
	pinned  map[string]string // normalized URL -> original pin
	cache   sync.Map          // string -> cachedDirectory
	ttl     time.Duration
	client  *http.Client
	now     func() time.Time
}

func NewBotAuthVerifier(config BotAuthConfig) (*BotAuthVerifier, error) {
	if !config.Enabled {
		return &BotAuthVerifier{enabled: false, now: config.Now}, nil
	}

	pinned := make(map[string]string, len(config.PinnedDirectories))
	for _, raw := range config.PinnedDirectories {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		normalized, err := normalizePinnedDirectory(raw)
		if err != nil {
			return nil, err
		}
		if _, exists := pinned[normalized]; exists {
			return nil, fmt.Errorf("bot_auth pinned directory %q is configured more than once", raw)
		}
		pinned[normalized] = strings.TrimRight(raw, "/")
	}

	ttl := config.CacheTTL
	if ttl < 0 {
		return nil, errors.New("bot_auth jwks cache TTL cannot be negative")
	}
	if ttl == 0 {
		ttl = defaultJWKSCacheTTL
	}
	if ttl > time.Hour {
		return nil, errors.New("bot_auth jwks cache TTL cannot exceed one hour")
	}

	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: defaultBotAuthFetchTimeout}
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}

	return &BotAuthVerifier{
		enabled: true,
		pinned:  pinned,
		ttl:     ttl,
		client:  client,
		now:     now,
	}, nil
}

// Skip reports whether the request presents a pinned web-bot-auth signature.
// Unpinned, malformed, or missing signatures return false and never raise
// suspicion: callers must fail open to the PoW lane.
func (v *BotAuthVerifier) Skip(req *http.Request) bool {
	if v == nil || !v.enabled || req == nil || len(v.pinned) == 0 {
		return false
	}

	agentURL, err := parseSignatureAgent(req.Header.Get("Signature-Agent"))
	if err != nil {
		return false
	}
	normalized, err := normalizePinnedDirectory(agentURL)
	if err != nil {
		return false
	}
	pin, ok := v.pinned[normalized]
	if !ok {
		return false
	}

	inputs, err := parseSignatureInput(strings.TrimSpace(req.Header.Get("Signature-Input")))
	if err != nil {
		return false
	}
	var selected *parsedSignature
	for i := range inputs {
		if inputs[i].tag == webBotAuthTag {
			selected = &inputs[i]
			break
		}
	}
	if selected == nil || selected.keyID == "" {
		return false
	}
	if selected.created <= 0 || selected.expires <= 0 {
		return false
	}
	now := v.now()
	if selected.expires-selected.created <= 0 || selected.expires-selected.created > int64(maxSignatureLifetime.Seconds()) {
		return false
	}
	if now.Add(botAuthClockSkew).Unix() < selected.created {
		return false
	}
	if now.Unix() >= selected.expires {
		return false
	}

	keys, err := v.keysForPinned(req.Context(), pin, normalized)
	if err != nil {
		return false
	}
	public, ok := lookupDirectoryKey(keys, selected.keyID)
	if !ok {
		return false
	}
	return verifyHTTPMessageSignature(req, public) == nil
}

func (v *BotAuthVerifier) keysForPinned(ctx context.Context, fetchURL, cacheKey string) ([]directoryKey, error) {
	now := v.now()
	if cached, ok := v.cache.Load(cacheKey); ok {
		entry := cached.(cachedDirectory)
		if now.Before(entry.expiresAt) {
			return entry.keys, nil
		}
	}

	keys, err := v.fetchDirectory(ctx, fetchURL)
	if err != nil {
		return nil, err
	}
	v.cache.Store(cacheKey, cachedDirectory{
		keys:      keys,
		expiresAt: now.Add(v.ttl),
	})
	return keys, nil
}

func (v *BotAuthVerifier) fetchDirectory(ctx context.Context, rawURL string) ([]directoryKey, error) {
	if _, ok := v.pinned[mustNormalizePinned(rawURL)]; !ok {
		return nil, errors.New("refusing to fetch unpinned Signature-Agent URL")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	fetchContext, cancel := context.WithTimeout(ctx, defaultBotAuthFetchTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(fetchContext, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := v.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("directory status %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxDirectoryBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxDirectoryBytes {
		return nil, errors.New("directory exceeds size limit")
	}
	return parseDirectoryJWKS(body)
}

func parseDirectoryJWKS(body []byte) ([]directoryKey, error) {
	var document struct {
		Keys []json.RawMessage `json:"keys"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, err
	}
	if len(document.Keys) == 0 {
		return nil, errors.New("directory contains no keys")
	}
	if len(document.Keys) > maxDirectoryKeys {
		return nil, errors.New("directory contains too many keys")
	}

	keys := make([]directoryKey, 0, len(document.Keys))
	for _, raw := range document.Keys {
		key, ok, err := parseEd25519DirectoryJWK(raw)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return nil, errors.New("directory contains no Ed25519 keys")
	}
	return keys, nil
}

func parseEd25519DirectoryJWK(raw json.RawMessage) (directoryKey, bool, error) {
	var jwk struct {
		KeyType   string `json:"kty"`
		Curve     string `json:"crv"`
		X         string `json:"x"`
		KeyID     string `json:"kid"`
		Algorithm string `json:"alg"`
	}
	if err := json.Unmarshal(raw, &jwk); err != nil {
		return directoryKey{}, false, err
	}
	if jwk.KeyType != "OKP" || jwk.Curve != "Ed25519" {
		return directoryKey{}, false, nil
	}
	if jwk.Algorithm != "" && jwk.Algorithm != "EdDSA" && !strings.EqualFold(jwk.Algorithm, "ed25519") {
		return directoryKey{}, false, nil
	}
	x, err := base64.RawURLEncoding.DecodeString(jwk.X)
	if err != nil || len(x) != ed25519.PublicKeySize {
		return directoryKey{}, false, errors.New("invalid Ed25519 JWK")
	}
	thumbprint, err := ed25519JWKThumbprint(jwk.X)
	if err != nil {
		return directoryKey{}, false, err
	}
	return directoryKey{
		keyID:      jwk.KeyID,
		thumbprint: thumbprint,
		public:     ed25519.PublicKey(x),
	}, true, nil
}

func ed25519JWKThumbprint(x string) (string, error) {
	// RFC 7638 / RFC 8037: lexicographic JSON of crv, kty, x.
	canonical := fmt.Sprintf(`{"crv":"Ed25519","kty":"OKP","x":%q}`, x)
	sum := sha256.Sum256([]byte(canonical))
	return base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

func lookupDirectoryKey(keys []directoryKey, keyID string) (ed25519.PublicKey, bool) {
	for _, key := range keys {
		if subtleStringEq(key.keyID, keyID) || subtleStringEq(key.thumbprint, keyID) {
			return key.public, true
		}
	}
	return nil, false
}

func subtleStringEq(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

func normalizePinnedDirectory(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
		return "", fmt.Errorf("bot_auth pinned directory %q must be an absolute URL without credentials or fragment", raw)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", fmt.Errorf("bot_auth pinned directory %q must be http or https", raw)
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	if parsed.Path == "" {
		parsed.Path = ""
	}
	return parsed.String(), nil
}

func mustNormalizePinned(raw string) string {
	normalized, err := normalizePinnedDirectory(raw)
	if err != nil {
		return ""
	}
	return normalized
}
