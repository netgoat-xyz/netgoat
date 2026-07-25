// Package cloudflare contains small, opt-in integrations for Cloudflare
// services. It deliberately keeps credentials out of logs and makes every
// request that depends on a third party fail closed.
package cloudflare

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultAccessHeader = "Cf-Access-Jwt-Assertion"
	defaultAccessCookie = "CF_Authorization"
	defaultJWKSCacheTTL = 5 * time.Minute
	defaultFetchTimeout = 10 * time.Second
	// Unknown key identifiers are entirely untrusted until a JWKS refresh
	// succeeds. Bound forced refreshes to keep malformed requests from using
	// the proxy as a JWKS-fetch amplifier while still allowing normal key
	// rotation to recover promptly.
	defaultUnknownKeyRefreshCooldown = 5 * time.Second
	maxJWTSize                       = 16 << 10
	maxJWKSSize                      = 1 << 20
	maxKeyIDLength                   = 256
)

// ErrAccessDenied is returned for malformed, expired, or untrusted Access
// assertions. Callers should not expose the wrapped error to clients.
var ErrAccessDenied = errors.New("cloudflare access denied")

// AccessConfig configures Cloudflare Access JWT verification. It is safe to
// leave disabled until the proxy has an issuer and audience configured.
//
// Issuer and JWKSURL must use HTTPS. When JWKSURL is empty, it is derived from
// Issuer using Cloudflare Access' /cdn-cgi/access/certs endpoint.
type AccessConfig struct {
	Enabled bool `json:"enabled" yaml:"enabled"`

	Issuer   string   `json:"issuer" yaml:"issuer"`
	Audience []string `json:"audience" yaml:"audience"`
	JWKSURL  string   `json:"jwks_url" yaml:"jwks_url"`

	// Header is checked first. It defaults to Cf-Access-Jwt-Assertion.
	Header string `json:"header" yaml:"header"`
	// Cookie is checked only if Header is absent. It defaults to
	// CF_Authorization.
	Cookie string `json:"cookie" yaml:"cookie"`

	// CacheTTL is an upper bound on JWKS cache lifetime. Zero selects five
	// minutes. Cloudflare's Cache-Control max-age can shorten it.
	CacheTTL time.Duration `json:"cache_ttl" yaml:"cache_ttl"`
	// ClockSkew permits small clock differences when evaluating exp and nbf.
	// Zero selects one minute; values above five minutes are rejected.
	ClockSkew time.Duration `json:"clock_skew" yaml:"clock_skew"`
	// FetchTimeout bounds each JWKS request. Zero selects ten seconds.
	FetchTimeout time.Duration `json:"fetch_timeout" yaml:"fetch_timeout"`

	// HTTPClient is intended for dependency injection and custom transport
	// policy. It is not a serializable configuration field.
	HTTPClient *http.Client `json:"-" yaml:"-"`
}

// Validate verifies that an enabled Access configuration is sufficiently
// explicit to be safe. Disabled configurations are intentionally accepted so
// a deployment can keep the integration configured but off.
func (config AccessConfig) Validate() error {
	_, err := config.normalized()
	return err
}

func (config AccessConfig) normalized() (AccessConfig, error) {
	if !config.Enabled {
		return config, nil
	}

	config.Issuer = strings.TrimSpace(config.Issuer)
	if err := validateHTTPSURL("Cloudflare Access issuer", config.Issuer); err != nil {
		return AccessConfig{}, err
	}

	seenAudience := make(map[string]struct{}, len(config.Audience))
	audience := make([]string, 0, len(config.Audience))
	for _, value := range config.Audience {
		value = strings.TrimSpace(value)
		if value == "" {
			return AccessConfig{}, errors.New("Cloudflare Access audience contains an empty value")
		}
		if _, exists := seenAudience[value]; exists {
			return AccessConfig{}, fmt.Errorf("Cloudflare Access audience %q is configured more than once", value)
		}
		seenAudience[value] = struct{}{}
		audience = append(audience, value)
	}
	if len(audience) == 0 {
		return AccessConfig{}, errors.New("Cloudflare Access audience is required when Access is enabled")
	}
	config.Audience = audience

	config.JWKSURL = strings.TrimSpace(config.JWKSURL)
	if config.JWKSURL == "" {
		config.JWKSURL = strings.TrimRight(config.Issuer, "/") + "/cdn-cgi/access/certs"
	}
	if err := validateHTTPSURL("Cloudflare Access JWKS URL", config.JWKSURL); err != nil {
		return AccessConfig{}, err
	}

	config.Header = strings.TrimSpace(config.Header)
	if config.Header == "" {
		config.Header = defaultAccessHeader
	}
	if !validHTTPToken(config.Header) {
		return AccessConfig{}, fmt.Errorf("Cloudflare Access header %q is invalid", config.Header)
	}

	config.Cookie = strings.TrimSpace(config.Cookie)
	if config.Cookie == "" {
		config.Cookie = defaultAccessCookie
	}
	if !validHTTPToken(config.Cookie) {
		return AccessConfig{}, fmt.Errorf("Cloudflare Access cookie %q is invalid", config.Cookie)
	}

	if config.CacheTTL < 0 {
		return AccessConfig{}, errors.New("Cloudflare Access JWKS cache TTL cannot be negative")
	}
	if config.CacheTTL == 0 {
		config.CacheTTL = defaultJWKSCacheTTL
	}
	if config.CacheTTL > 24*time.Hour {
		return AccessConfig{}, errors.New("Cloudflare Access JWKS cache TTL cannot exceed 24 hours")
	}

	if config.ClockSkew < 0 || config.ClockSkew > 5*time.Minute {
		return AccessConfig{}, errors.New("Cloudflare Access clock skew must be between zero and five minutes")
	}
	if config.ClockSkew == 0 {
		config.ClockSkew = time.Minute
	}

	if config.FetchTimeout < 0 || config.FetchTimeout > 30*time.Second {
		return AccessConfig{}, errors.New("Cloudflare Access JWKS fetch timeout must be between zero and 30 seconds")
	}
	if config.FetchTimeout == 0 {
		config.FetchTimeout = defaultFetchTimeout
	}

	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: config.FetchTimeout}
	}
	return config, nil
}

// Identity is the authenticated Cloudflare Access subject attached to a
// request. Claims contains a fresh map for each validated token; it must be
// treated as read-only by downstream handlers.
type Identity struct {
	Subject   string
	Email     string
	Issuer    string
	Audience  []string
	ExpiresAt time.Time
	NotBefore *time.Time
	Claims    map[string]any
}

type identityContextKey struct{}

// IdentityFromContext returns the Access identity added by Middleware.
func IdentityFromContext(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(identityContextKey{}).(Identity)
	if !ok {
		return Identity{}, false
	}
	identity.Audience = append([]string(nil), identity.Audience...)
	return identity, true
}

// AccessValidator verifies signed Cloudflare Access assertions. It maintains a
// bounded, synchronized JWKS cache and refreshes the key set when a new kid is
// observed, which supports normal Cloudflare signing-key rotation.
type AccessValidator struct {
	config AccessConfig
	client *http.Client
	now    func() time.Time

	mu              sync.RWMutex
	keys            map[string]verificationKey
	expiresAt       time.Time
	retryUntilByID  map[string]time.Time
	lastForcedFetch time.Time
	refreshCooldown time.Duration
	fetchMu         sync.Mutex
}

// NewAccessValidator creates a verifier. No network request is made until the
// first assertion is validated.
func NewAccessValidator(config AccessConfig) (*AccessValidator, error) {
	normalized, err := config.normalized()
	if err != nil {
		return nil, err
	}
	client := normalized.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: defaultFetchTimeout}
	}
	return &AccessValidator{
		config:          normalized,
		client:          cloneNoRedirectClient(client),
		now:             time.Now,
		keys:            make(map[string]verificationKey),
		retryUntilByID:  make(map[string]time.Time),
		refreshCooldown: defaultUnknownKeyRefreshCooldown,
	}, nil
}

// ValidateRequest extracts and validates a Cloudflare Access assertion from a
// configured header or cookie.
func (validator *AccessValidator) ValidateRequest(ctx context.Context, request *http.Request) (Identity, error) {
	if validator == nil || !validator.config.Enabled {
		return Identity{}, accessDenied("Cloudflare Access is not enabled")
	}
	token, err := validator.tokenFromRequest(request)
	if err != nil {
		return Identity{}, err
	}
	return validator.ValidateToken(ctx, token)
}

// ValidateToken verifies a serialized compact JWS assertion and its Access
// issuer, audience, exp, and nbf claims.
func (validator *AccessValidator) ValidateToken(ctx context.Context, token string) (Identity, error) {
	if validator == nil || !validator.config.Enabled {
		return Identity{}, accessDenied("Cloudflare Access is not enabled")
	}
	header, claims, signingInput, signature, err := parseAssertion(token)
	if err != nil {
		return Identity{}, err
	}
	if !supportedAlgorithm(header.Algorithm) {
		return Identity{}, accessDenied("unsupported Cloudflare Access signing algorithm")
	}
	if len(header.KeyID) == 0 || len(header.KeyID) > maxKeyIDLength {
		return Identity{}, accessDenied("Cloudflare Access assertion has an invalid key identifier")
	}
	identity, err := validator.validateClaims(claims)
	if err != nil {
		return Identity{}, err
	}

	key, err := validator.keyFor(ctx, header.KeyID, false)
	if err != nil {
		return Identity{}, err
	}
	if err := key.verify(header.Algorithm, signingInput, signature); err == nil {
		return identity, nil
	}

	// A key can occasionally be rotated while keeping its kid. Retry one JWKS
	// refresh for this kid before rejecting the assertion. The retry cache
	// prevents repeated bad tokens from forcing a fetch for every request.
	key, refreshErr := validator.keyFor(ctx, header.KeyID, true)
	if refreshErr == nil && key.verify(header.Algorithm, signingInput, signature) == nil {
		return identity, nil
	}
	return Identity{}, accessDenied("Cloudflare Access assertion signature is invalid")
}

// Middleware returns an HTTP middleware that refuses every request with an
// invalid or unavailable Access assertion when Access is enabled. It never
// falls through to next on authentication failure.
func (validator *AccessValidator) Middleware(next http.Handler) http.Handler {
	if next == nil {
		next = http.NotFoundHandler()
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if validator == nil || !validator.config.Enabled {
			next.ServeHTTP(writer, request)
			return
		}
		identity, err := validator.ValidateRequest(request.Context(), request)
		if err != nil {
			http.Error(writer, "Cloudflare Access authentication required", http.StatusForbidden)
			return
		}
		request = request.WithContext(context.WithValue(request.Context(), identityContextKey{}, identity))
		next.ServeHTTP(writer, request)
	})
}

func (validator *AccessValidator) tokenFromRequest(request *http.Request) (string, error) {
	if request == nil {
		return "", accessDenied("Cloudflare Access request is nil")
	}
	values := request.Header.Values(validator.config.Header)
	if len(values) > 1 {
		return "", accessDenied("Cloudflare Access header is ambiguous")
	}
	if len(values) == 1 && strings.TrimSpace(values[0]) != "" {
		return parseBearerToken(values[0])
	}

	var cookieValue string
	for _, cookie := range request.Cookies() {
		if cookie.Name != validator.config.Cookie {
			continue
		}
		if cookieValue != "" {
			return "", accessDenied("Cloudflare Access cookie is ambiguous")
		}
		cookieValue = cookie.Value
	}
	if cookieValue == "" {
		return "", accessDenied("Cloudflare Access assertion is missing")
	}
	return parseBearerToken(cookieValue)
}

func parseBearerToken(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maxJWTSize {
		return "", accessDenied("Cloudflare Access assertion is invalid")
	}
	fields := strings.Fields(value)
	if len(fields) == 1 {
		return fields[0], nil
	}
	if len(fields) == 2 && strings.EqualFold(fields[0], "Bearer") {
		return fields[1], nil
	}
	return "", accessDenied("Cloudflare Access assertion is invalid")
}

func (validator *AccessValidator) keyFor(ctx context.Context, keyID string, forceRefresh bool) (verificationKey, error) {
	now := validator.now()
	validator.mu.RLock()
	key, found := validator.keys[keyID]
	fresh := now.Before(validator.expiresAt)
	validator.mu.RUnlock()
	if found && fresh && !forceRefresh {
		return key, nil
	}

	if forceRefresh || !fresh || !found {
		if forceRefresh || !found {
			if !validator.allowRetryForKey(keyID, now) {
				return verificationKey{}, accessDenied("Cloudflare Access signing key is unavailable")
			}
		}
		if err := validator.refreshKeys(ctx, forceRefresh || !found); err != nil {
			return verificationKey{}, accessDenied("Cloudflare Access signing keys are unavailable")
		}
	}

	validator.mu.RLock()
	key, found = validator.keys[keyID]
	validator.mu.RUnlock()
	if !found {
		return verificationKey{}, accessDenied("Cloudflare Access signing key is unknown")
	}
	return key, nil
}

func (validator *AccessValidator) allowRetryForKey(keyID string, now time.Time) bool {
	validator.mu.Lock()
	defer validator.mu.Unlock()
	for candidate, until := range validator.retryUntilByID {
		if !until.After(now) {
			delete(validator.retryUntilByID, candidate)
		}
	}
	if until := validator.retryUntilByID[keyID]; until.After(now) {
		return false
	}
	if len(validator.keys) > 0 && validator.refreshCooldown > 0 && validator.lastForcedFetch.Add(validator.refreshCooldown).After(now) {
		return false
	}
	// Keep an attacker from growing the per-kid retry map without bound. A
	// legitimate rotation only needs one extra entry.
	if len(validator.retryUntilByID) >= 128 {
		for candidate := range validator.retryUntilByID {
			delete(validator.retryUntilByID, candidate)
			break
		}
	}
	retryDelay := validator.refreshCooldown
	if retryDelay <= 0 {
		retryDelay = time.Second
	}
	validator.retryUntilByID[keyID] = now.Add(retryDelay)
	if len(validator.keys) > 0 {
		validator.lastForcedFetch = now
	}
	return true
}

func (validator *AccessValidator) refreshKeys(ctx context.Context, force bool) error {
	validator.fetchMu.Lock()
	defer validator.fetchMu.Unlock()

	now := validator.now()
	validator.mu.RLock()
	fresh := now.Before(validator.expiresAt)
	validator.mu.RUnlock()
	if fresh && !force {
		return nil
	}

	fetchContext, cancel := context.WithTimeout(ctx, validator.config.FetchTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(fetchContext, http.MethodGet, validator.config.JWKSURL, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	response, err := validator.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected JWKS status %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxJWKSSize+1))
	if err != nil {
		return err
	}
	if len(body) > maxJWKSSize {
		return errors.New("JWKS response exceeds size limit")
	}
	keys, err := parseJWKS(body)
	if err != nil {
		return err
	}
	cacheTTL := validator.config.CacheTTL
	if responseTTL, ok := cacheControlTTL(response.Header.Get("Cache-Control")); ok && responseTTL < cacheTTL {
		cacheTTL = responseTTL
	}
	if cacheTTL < time.Second {
		cacheTTL = time.Second
	}

	now = validator.now()
	validator.mu.Lock()
	validator.keys = keys
	validator.expiresAt = now.Add(cacheTTL)
	validator.mu.Unlock()
	return nil
}

func cacheControlTTL(value string) (time.Duration, bool) {
	for _, directive := range strings.Split(value, ",") {
		parts := strings.SplitN(strings.TrimSpace(directive), "=", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "max-age") {
			continue
		}
		seconds, err := strconv.ParseInt(strings.Trim(strings.TrimSpace(parts[1]), "\""), 10, 64)
		if err != nil || seconds < 0 {
			return 0, false
		}
		return time.Duration(seconds) * time.Second, true
	}
	return 0, false
}

type assertionHeader struct {
	Algorithm string   `json:"alg"`
	KeyID     string   `json:"kid"`
	Critical  []string `json:"crit"`
}

type assertionClaims struct {
	Issuer    string
	Audience  []string
	ExpiresAt time.Time
	NotBefore *time.Time
	Subject   string
	Email     string
	Raw       map[string]any
}

func parseAssertion(token string) (assertionHeader, assertionClaims, []byte, []byte, error) {
	if len(token) == 0 || len(token) > maxJWTSize {
		return assertionHeader{}, assertionClaims{}, nil, nil, accessDenied("Cloudflare Access assertion is invalid")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return assertionHeader{}, assertionClaims{}, nil, nil, accessDenied("Cloudflare Access assertion is not a compact JWS")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || len(headerBytes) > 4096 {
		return assertionHeader{}, assertionClaims{}, nil, nil, accessDenied("Cloudflare Access assertion header is invalid")
	}
	var header assertionHeader
	if err := decodeJSON(headerBytes, &header); err != nil || len(header.Critical) != 0 {
		return assertionHeader{}, assertionClaims{}, nil, nil, accessDenied("Cloudflare Access assertion header is invalid")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(payload) > maxJWTSize {
		return assertionHeader{}, assertionClaims{}, nil, nil, accessDenied("Cloudflare Access assertion claims are invalid")
	}
	claims, err := parseClaims(payload)
	if err != nil {
		return assertionHeader{}, assertionClaims{}, nil, nil, err
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(signature) == 0 {
		return assertionHeader{}, assertionClaims{}, nil, nil, accessDenied("Cloudflare Access assertion signature is invalid")
	}
	return header, claims, []byte(parts[0] + "." + parts[1]), signature, nil
}

func parseClaims(payload []byte) (assertionClaims, error) {
	var raw map[string]json.RawMessage
	if err := decodeJSON(payload, &raw); err != nil {
		return assertionClaims{}, accessDenied("Cloudflare Access assertion claims are invalid")
	}
	issuer, err := stringClaim(raw, "iss", true)
	if err != nil {
		return assertionClaims{}, err
	}
	audience, err := audienceClaim(raw["aud"])
	if err != nil {
		return assertionClaims{}, err
	}
	expiresAt, _, err := numericDateClaim(raw, "exp", true)
	if err != nil {
		return assertionClaims{}, err
	}
	notBeforeValue, notBeforeSet, err := numericDateClaim(raw, "nbf", false)
	if err != nil {
		return assertionClaims{}, err
	}
	subject, err := stringClaim(raw, "sub", false)
	if err != nil {
		return assertionClaims{}, err
	}
	email, err := stringClaim(raw, "email", false)
	if err != nil {
		return assertionClaims{}, err
	}
	var values map[string]any
	if err := decodeJSON(payload, &values); err != nil {
		return assertionClaims{}, accessDenied("Cloudflare Access assertion claims are invalid")
	}
	var notBefore *time.Time
	if notBeforeSet {
		notBefore = &notBeforeValue
	}
	return assertionClaims{
		Issuer:    issuer,
		Audience:  audience,
		ExpiresAt: expiresAt,
		NotBefore: notBefore,
		Subject:   subject,
		Email:     email,
		Raw:       values,
	}, nil
}

func (validator *AccessValidator) validateClaims(claims assertionClaims) (Identity, error) {
	if subtle.ConstantTimeCompare([]byte(claims.Issuer), []byte(validator.config.Issuer)) != 1 {
		return Identity{}, accessDenied("Cloudflare Access assertion issuer is invalid")
	}
	matchedAudience := false
	for _, tokenAudience := range claims.Audience {
		for _, expectedAudience := range validator.config.Audience {
			if subtle.ConstantTimeCompare([]byte(tokenAudience), []byte(expectedAudience)) == 1 {
				matchedAudience = true
				break
			}
		}
	}
	if !matchedAudience {
		return Identity{}, accessDenied("Cloudflare Access assertion audience is invalid")
	}
	now := validator.now()
	if now.After(claims.ExpiresAt.Add(validator.config.ClockSkew)) {
		return Identity{}, accessDenied("Cloudflare Access assertion has expired")
	}
	if claims.NotBefore != nil && now.Add(validator.config.ClockSkew).Before(*claims.NotBefore) {
		return Identity{}, accessDenied("Cloudflare Access assertion is not active")
	}
	return Identity{
		Subject:   claims.Subject,
		Email:     claims.Email,
		Issuer:    claims.Issuer,
		Audience:  append([]string(nil), claims.Audience...),
		ExpiresAt: claims.ExpiresAt,
		NotBefore: claims.NotBefore,
		Claims:    claims.Raw,
	}, nil
}

func stringClaim(values map[string]json.RawMessage, name string, required bool) (string, error) {
	raw, found := values[name]
	if !found {
		if required {
			return "", accessDenied("Cloudflare Access assertion is missing " + name)
		}
		return "", nil
	}
	var value string
	if err := decodeJSON(raw, &value); err != nil || (required && value == "") {
		return "", accessDenied("Cloudflare Access assertion has an invalid " + name)
	}
	return value, nil
}

func audienceClaim(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 {
		return nil, accessDenied("Cloudflare Access assertion is missing aud")
	}
	var single string
	if err := decodeJSON(raw, &single); err == nil {
		if single == "" {
			return nil, accessDenied("Cloudflare Access assertion has an invalid aud")
		}
		return []string{single}, nil
	}
	var multiple []string
	if err := decodeJSON(raw, &multiple); err != nil || len(multiple) == 0 {
		return nil, accessDenied("Cloudflare Access assertion has an invalid aud")
	}
	seen := make(map[string]struct{}, len(multiple))
	for _, value := range multiple {
		if value == "" {
			return nil, accessDenied("Cloudflare Access assertion has an invalid aud")
		}
		if _, duplicate := seen[value]; duplicate {
			return nil, accessDenied("Cloudflare Access assertion has a duplicate aud")
		}
		seen[value] = struct{}{}
	}
	return multiple, nil
}

func numericDateClaim(values map[string]json.RawMessage, name string, required bool) (time.Time, bool, error) {
	raw, found := values[name]
	if !found {
		if required {
			return time.Time{}, false, accessDenied("Cloudflare Access assertion is missing " + name)
		}
		return time.Time{}, false, nil
	}
	var number json.Number
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&number); err != nil {
		return time.Time{}, false, accessDenied("Cloudflare Access assertion has an invalid " + name)
	}
	if err := ensureEOF(decoder); err != nil {
		return time.Time{}, false, accessDenied("Cloudflare Access assertion has an invalid " + name)
	}
	seconds, err := strconv.ParseFloat(number.String(), 64)
	if err != nil || seconds != seconds || seconds > 253402300799 || seconds < -62135596800 {
		return time.Time{}, false, accessDenied("Cloudflare Access assertion has an invalid " + name)
	}
	wholeSeconds, fraction := mathModf(seconds)
	return time.Unix(int64(wholeSeconds), int64(fraction*float64(time.Second))).UTC(), true, nil
}

// mathModf avoids importing a broad math surface for a single finite-value
// conversion. Seconds have already been constrained to the range supported by
// time.Unix.
func mathModf(value float64) (float64, float64) {
	whole := float64(int64(value))
	return whole, value - whole
}

func decodeJSON(input []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return ensureEOF(decoder)
}

func ensureEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("unexpected trailing JSON value")
	}
	return err
}

func accessDenied(message string) error {
	return fmt.Errorf("%w: %s", ErrAccessDenied, message)
}

type verificationKey struct {
	algorithm string
	key       any
}

func (key verificationKey) verify(algorithm string, signingInput, signature []byte) error {
	if key.algorithm != "" && key.algorithm != algorithm {
		return errors.New("JWK algorithm does not match assertion")
	}
	switch publicKey := key.key.(type) {
	case *rsa.PublicKey:
		hash, err := hashForAlgorithm(algorithm)
		if err != nil {
			return err
		}
		digest := digestForHash(hash, signingInput)
		if strings.HasPrefix(algorithm, "PS") {
			return rsa.VerifyPSS(publicKey, hash, digest, signature, &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash})
		}
		return rsa.VerifyPKCS1v15(publicKey, hash, digest, signature)
	case *ecdsa.PublicKey:
		hash, err := hashForAlgorithm(algorithm)
		if err != nil {
			return err
		}
		size := (publicKey.Curve.Params().BitSize + 7) / 8
		if len(signature) != 2*size {
			return errors.New("invalid ECDSA signature length")
		}
		r := new(big.Int).SetBytes(signature[:size])
		s := new(big.Int).SetBytes(signature[size:])
		if r.Sign() <= 0 || s.Sign() <= 0 || !ecdsa.Verify(publicKey, digestForHash(hash, signingInput), r, s) {
			return errors.New("invalid ECDSA signature")
		}
		return nil
	case ed25519.PublicKey:
		if algorithm != "EdDSA" || !ed25519.Verify(publicKey, signingInput, signature) {
			return errors.New("invalid EdDSA signature")
		}
		return nil
	default:
		return errors.New("unsupported JWK key type")
	}
}

func supportedAlgorithm(algorithm string) bool {
	switch algorithm {
	case "RS256", "RS384", "RS512", "PS256", "PS384", "PS512", "ES256", "ES384", "ES512", "EdDSA":
		return true
	default:
		return false
	}
}

func hashForAlgorithm(algorithm string) (crypto.Hash, error) {
	switch algorithm {
	case "RS256", "PS256", "ES256":
		return crypto.SHA256, nil
	case "RS384", "PS384", "ES384":
		return crypto.SHA384, nil
	case "RS512", "PS512", "ES512":
		return crypto.SHA512, nil
	default:
		return 0, errors.New("unsupported signature algorithm")
	}
}

func digestForHash(hash crypto.Hash, input []byte) []byte {
	switch hash {
	case crypto.SHA256:
		digest := sha256.Sum256(input)
		return digest[:]
	case crypto.SHA384:
		digest := sha512.Sum384(input)
		return digest[:]
	case crypto.SHA512:
		digest := sha512.Sum512(input)
		return digest[:]
	default:
		return nil
	}
}

type jwk struct {
	KeyType   string `json:"kty"`
	KeyID     string `json:"kid"`
	Algorithm string `json:"alg"`
	Use       string `json:"use"`
	Modulus   string `json:"n"`
	Exponent  string `json:"e"`
	Curve     string `json:"crv"`
	X         string `json:"x"`
	Y         string `json:"y"`
}

func parseJWKS(input []byte) (map[string]verificationKey, error) {
	var document struct {
		Keys []json.RawMessage `json:"keys"`
	}
	if err := decodeJSON(input, &document); err != nil || len(document.Keys) == 0 {
		return nil, errors.New("JWKS does not contain keys")
	}
	keys := make(map[string]verificationKey, len(document.Keys))
	seenKeyIDs := make(map[string]struct{}, len(document.Keys))
	for _, raw := range document.Keys {
		var value jwk
		if err := decodeJSON(raw, &value); err != nil {
			continue
		}
		if len(value.KeyID) == 0 || len(value.KeyID) > maxKeyIDLength {
			continue
		}
		if _, duplicate := seenKeyIDs[value.KeyID]; duplicate {
			return nil, errors.New("JWKS contains duplicate key identifiers")
		}
		seenKeyIDs[value.KeyID] = struct{}{}
		if value.Use != "" && value.Use != "sig" {
			continue
		}
		key, err := parseJWK(value)
		if err != nil {
			continue
		}
		keys[value.KeyID] = key
	}
	if len(keys) == 0 {
		return nil, errors.New("JWKS contains no supported signing keys")
	}
	return keys, nil
}

func parseJWK(value jwk) (verificationKey, error) {
	if value.Algorithm != "" && !supportedAlgorithm(value.Algorithm) {
		return verificationKey{}, errors.New("unsupported JWK algorithm")
	}
	switch value.KeyType {
	case "RSA":
		if value.Algorithm != "" && !strings.HasPrefix(value.Algorithm, "RS") && !strings.HasPrefix(value.Algorithm, "PS") {
			return verificationKey{}, errors.New("JWK algorithm does not use RSA")
		}
		modulus, err := decodeBase64URL(value.Modulus)
		if err != nil || len(modulus) == 0 {
			return verificationKey{}, errors.New("invalid RSA modulus")
		}
		exponentBytes, err := decodeBase64URL(value.Exponent)
		if err != nil || len(exponentBytes) == 0 {
			return verificationKey{}, errors.New("invalid RSA exponent")
		}
		exponentBig := new(big.Int).SetBytes(exponentBytes)
		if !exponentBig.IsInt64() || exponentBig.Sign() <= 0 || exponentBig.Int64() > int64(^uint(0)>>1) {
			return verificationKey{}, errors.New("invalid RSA exponent")
		}
		key := &rsa.PublicKey{N: new(big.Int).SetBytes(modulus), E: int(exponentBig.Int64())}
		if key.N.BitLen() < 2048 || key.E < 3 || key.E%2 == 0 {
			return verificationKey{}, errors.New("invalid RSA public key")
		}
		return verificationKey{algorithm: value.Algorithm, key: key}, nil
	case "EC":
		curve, expectedAlgorithm, err := jwkCurve(value.Curve)
		if err != nil || (value.Algorithm != "" && value.Algorithm != expectedAlgorithm) {
			return verificationKey{}, errors.New("invalid EC JWK")
		}
		x, err := decodeBase64URL(value.X)
		if err != nil {
			return verificationKey{}, errors.New("invalid EC x coordinate")
		}
		y, err := decodeBase64URL(value.Y)
		if err != nil {
			return verificationKey{}, errors.New("invalid EC y coordinate")
		}
		size := (curve.Params().BitSize + 7) / 8
		if len(x) != size || len(y) != size {
			return verificationKey{}, errors.New("invalid EC coordinate length")
		}
		pointX, pointY := elliptic.Unmarshal(curve, append(append([]byte{4}, x...), y...))
		if pointX == nil || pointY == nil {
			return verificationKey{}, errors.New("invalid EC public key")
		}
		return verificationKey{algorithm: value.Algorithm, key: &ecdsa.PublicKey{Curve: curve, X: pointX, Y: pointY}}, nil
	case "OKP":
		if value.Curve != "Ed25519" || (value.Algorithm != "" && value.Algorithm != "EdDSA") {
			return verificationKey{}, errors.New("invalid Ed25519 JWK")
		}
		key, err := decodeBase64URL(value.X)
		if err != nil || len(key) != ed25519.PublicKeySize {
			return verificationKey{}, errors.New("invalid Ed25519 public key")
		}
		return verificationKey{algorithm: value.Algorithm, key: ed25519.PublicKey(key)}, nil
	default:
		return verificationKey{}, errors.New("unsupported JWK key type")
	}
}

func jwkCurve(name string) (elliptic.Curve, string, error) {
	switch name {
	case "P-256":
		return elliptic.P256(), "ES256", nil
	case "P-384":
		return elliptic.P384(), "ES384", nil
	case "P-521":
		return elliptic.P521(), "ES512", nil
	default:
		return nil, "", errors.New("unsupported EC curve")
	}
}

func decodeBase64URL(value string) ([]byte, error) {
	if value == "" || strings.Contains(value, "=") {
		return nil, errors.New("invalid base64url value")
	}
	return base64.RawURLEncoding.DecodeString(value)
}

func validateHTTPSURL(name, raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" || parsed.RawQuery != "" {
		return fmt.Errorf("%s must be an absolute HTTPS URL without credentials, query, or fragment", name)
	}
	return nil
}

func validHTTPToken(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if (character >= '0' && character <= '9') || (character >= 'A' && character <= 'Z') || (character >= 'a' && character <= 'z') {
			continue
		}
		switch character {
		case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
			continue
		default:
			return false
		}
	}
	return true
}
