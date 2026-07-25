package cloudflare

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	testIssuer   = "https://team.cloudflareaccess.com"
	testAudience = "example-audience"
	testZoneID   = "0123456789abcdef0123456789abcdef"
	testRecordID = "fedcba9876543210fedcba9876543210"
	testAccount  = "00112233445566778899aabbccddeeff"
	testTunnel   = "e590a2a4-8d8f-4c74-a315-6b19752bd85f"
)

func TestAccessValidatorValidatesAndCachesJWKS(t *testing.T) {
	privateKey := newRSAPrivateKey(t)
	service := newJWKSService(t, jwksDocument(t, rsaJWK("key-a", &privateKey.PublicKey)))
	defer service.Close()

	validator := newTestAccessValidator(t, service)
	now := time.Now().UTC()
	token := signedRS256(t, privateKey, "key-a", accessClaims(now))

	identity, err := validator.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}
	if identity.Subject != "user-123" || identity.Email != "user@example.com" {
		t.Fatalf("identity = %#v", identity)
	}
	if got := service.Requests(); got != 1 {
		t.Fatalf("JWKS requests after first token = %d, want 1", got)
	}
	if _, err := validator.ValidateToken(context.Background(), token); err != nil {
		t.Fatalf("second ValidateToken() error = %v", err)
	}
	if got := service.Requests(); got != 1 {
		t.Fatalf("JWKS cache was not used; requests = %d, want 1", got)
	}
}

func TestAccessValidatorRefreshesForRotatedKeyID(t *testing.T) {
	firstKey := newRSAPrivateKey(t)
	secondKey := newRSAPrivateKey(t)
	service := newJWKSService(t, jwksDocument(t, rsaJWK("key-a", &firstKey.PublicKey)))
	defer service.Close()

	validator := newTestAccessValidator(t, service)
	validator.refreshCooldown = 0
	now := time.Now().UTC()
	if _, err := validator.ValidateToken(context.Background(), signedRS256(t, firstKey, "key-a", accessClaims(now))); err != nil {
		t.Fatalf("initial key validation error = %v", err)
	}
	service.SetDocument(jwksDocument(t, rsaJWK("key-b", &secondKey.PublicKey)))
	if _, err := validator.ValidateToken(context.Background(), signedRS256(t, secondKey, "key-b", accessClaims(now))); err != nil {
		t.Fatalf("rotated key validation error = %v", err)
	}
	if got := service.Requests(); got != 2 {
		t.Fatalf("JWKS requests after key rotation = %d, want 2", got)
	}
}

func TestAccessValidatorSupportsES256(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	service := newJWKSService(t, jwksDocument(t, ecJWK("key-ec", &privateKey.PublicKey)))
	defer service.Close()

	validator := newTestAccessValidator(t, service)
	identity, err := validator.ValidateToken(context.Background(), signedES256(t, privateKey, "key-ec", accessClaims(time.Now().UTC())))
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}
	if identity.Subject != "user-123" {
		t.Fatalf("identity subject = %q, want user-123", identity.Subject)
	}
}

func TestAccessValidatorRejectsInvalidTimeAndUnsupportedAlgorithms(t *testing.T) {
	privateKey := newRSAPrivateKey(t)
	service := newJWKSService(t, jwksDocument(t, rsaJWK("key-a", &privateKey.PublicKey)))
	defer service.Close()
	validator := newTestAccessValidator(t, service)

	expired := accessClaims(time.Now().UTC().Add(-2 * time.Hour))
	if _, err := validator.ValidateToken(context.Background(), signedRS256(t, privateKey, "key-a", expired)); !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("expired assertion error = %v, want ErrAccessDenied", err)
	}
	if got := service.Requests(); got != 0 {
		t.Fatalf("expired token fetched JWKS %d times, want 0", got)
	}

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","kid":"key-a"}`))
	payload := base64.RawURLEncoding.EncodeToString(mustJSON(t, accessClaims(time.Now().UTC())))
	if _, err := validator.ValidateToken(context.Background(), header+"."+payload+".AQ"); !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("unsupported algorithm error = %v, want ErrAccessDenied", err)
	}
}

func TestAccessMiddlewareFailsClosedAndUsesConfiguredCookie(t *testing.T) {
	privateKey := newRSAPrivateKey(t)
	service := newJWKSService(t, jwksDocument(t, rsaJWK("key-a", &privateKey.PublicKey)))
	defer service.Close()
	validator := newTestAccessValidator(t, service)
	validator.config.Header = "Authorization"
	validator.config.Cookie = "access_token"

	called := false
	handler := validator.Middleware(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		called = true
		identity, found := IdentityFromContext(request.Context())
		if !found || identity.Subject != "user-123" {
			t.Errorf("identity in request = %#v, found=%v", identity, found)
		}
		writer.WriteHeader(http.StatusNoContent)
	}))

	missing := httptest.NewRequest(http.MethodGet, "https://proxy.example/private", nil)
	missingResponse := httptest.NewRecorder()
	handler.ServeHTTP(missingResponse, missing)
	if missingResponse.Code != http.StatusForbidden || called {
		t.Fatalf("missing assertion response = %d, called=%v", missingResponse.Code, called)
	}

	token := signedRS256(t, privateKey, "key-a", accessClaims(time.Now().UTC()))
	called = false
	headerRequest := httptest.NewRequest(http.MethodGet, "https://proxy.example/private", nil)
	headerRequest.Header.Set("Authorization", "Bearer "+token)
	headerResponse := httptest.NewRecorder()
	handler.ServeHTTP(headerResponse, headerRequest)
	if headerResponse.Code != http.StatusNoContent || !called {
		t.Fatalf("bearer assertion response = %d, called=%v", headerResponse.Code, called)
	}

	called = false
	request := httptest.NewRequest(http.MethodGet, "https://proxy.example/private", nil)
	request.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || !called {
		t.Fatalf("cookie assertion response = %d, called=%v", response.Code, called)
	}
}

func TestAccessConfigRejectsInsecureOrAmbiguousSettings(t *testing.T) {
	config := AccessConfig{Enabled: true, Issuer: "http://team.cloudflareaccess.com", Audience: []string{testAudience}}
	if err := config.Validate(); err == nil {
		t.Fatal("Validate() accepted an HTTP issuer")
	}
	config = AccessConfig{Enabled: true, Issuer: testIssuer, Audience: []string{testAudience, testAudience}}
	if err := config.Validate(); err == nil {
		t.Fatal("Validate() accepted duplicate audiences")
	}
}

func TestParseJWKSBoundsUntrustedInput(t *testing.T) {
	keys := make([]json.RawMessage, maxJWKSKeys+1)
	for index := range keys {
		keys[index] = json.RawMessage(`{}`)
	}
	if _, err := parseJWKS(mustJSON(t, map[string]any{"keys": keys})); err == nil {
		t.Fatal("parseJWKS() accepted an oversized key set")
	}
}

func TestAPIClientDryRunNeverPerformsNetworkIO(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls.Add(1)
	}))
	defer server.Close()

	client := newTestAPIClient(t, server, true)
	result, err := client.ReconcileDNSRecord(context.Background(), testZoneID, "", map[string]any{"type": "A", "name": "example.com", "content": "192.0.2.1"})
	if err != nil {
		t.Fatalf("ReconcileDNSRecord() error = %v", err)
	}
	if !result.DryRun || result.Method != http.MethodPost || !strings.HasSuffix(result.Path, "/zones/"+testZoneID+"/dns_records") {
		t.Fatalf("dry-run result = %#v", result)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("dry-run made %d network requests", got)
	}
}

func TestAPIClientAuthenticatesValidatedRequests(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		if request.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", request.Method)
		}
		if request.URL.Path != "/client/v4/zones/"+testZoneID+"/dns_records/"+testRecordID {
			t.Errorf("path = %q", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer test-api-token" {
			t.Errorf("authorization header = %q", request.Header.Get("Authorization"))
		}
		if request.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content type = %q", request.Header.Get("Content-Type"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"success":true,"result":{}}`))
	}))
	defer server.Close()

	client := newTestAPIClient(t, server, false)
	result, err := client.ReconcileDNSRecord(context.Background(), testZoneID, testRecordID, map[string]any{"type": "A"})
	if err != nil {
		t.Fatalf("ReconcileDNSRecord() error = %v", err)
	}
	if result.StatusCode != http.StatusOK || result.DryRun || calls.Load() != 1 {
		t.Fatalf("API result = %#v, calls=%d", result, calls.Load())
	}
}

func TestAPIClientUsesCurrentTunnelCreateAndUpdateMethods(t *testing.T) {
	requests := make([]struct {
		method string
		path   string
	}, 0, 2)
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests = append(requests, struct {
			method string
			path   string
		}{method: request.Method, path: request.URL.Path})
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"success":true,"result":{}}`))
	}))
	defer server.Close()

	client := newTestAPIClient(t, server, false)
	if _, err := client.CreateTunnel(context.Background(), map[string]any{"name": "new-tunnel", "config_src": "cloudflare"}); err != nil {
		t.Fatalf("CreateTunnel() error = %v", err)
	}
	if _, err := client.ReconcileTunnel(context.Background(), testTunnel, map[string]any{"name": "renamed-tunnel"}); err != nil {
		t.Fatalf("ReconcileTunnel() error = %v", err)
	}

	if len(requests) != 2 {
		t.Fatalf("tunnel requests = %d, want 2", len(requests))
	}
	if requests[0].method != http.MethodPost || requests[0].path != "/client/v4/accounts/"+testAccount+"/cfd_tunnel" {
		t.Fatalf("create request = %#v", requests[0])
	}
	if requests[1].method != http.MethodPatch || requests[1].path != "/client/v4/accounts/"+testAccount+"/cfd_tunnel/"+testTunnel {
		t.Fatalf("update request = %#v", requests[1])
	}
}

func TestAPIClientRejectsDisabledAndInvalidIdentifiersBeforeNetworkIO(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls.Add(1)
	}))
	defer server.Close()

	disabled, err := NewAPIClient(APIConfig{BaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatalf("NewAPIClient() error = %v", err)
	}
	if _, err := disabled.ReconcileDNSRecord(context.Background(), testZoneID, "", map[string]any{"type": "A"}); !errors.Is(err, ErrAPIUnavailable) {
		t.Fatalf("disabled client error = %v, want ErrAPIUnavailable", err)
	}
	enabled := newTestAPIClient(t, server, false)
	if _, err := enabled.ReconcileDNSRecord(context.Background(), "../../etc/passwd", "", map[string]any{"type": "A"}); !errors.Is(err, ErrInvalidIdentifier) {
		t.Fatalf("invalid zone error = %v, want ErrInvalidIdentifier", err)
	}
	if _, err := enabled.ReconcileTunnel(context.Background(), "bad tunnel", map[string]any{"name": "edge"}); !errors.Is(err, ErrInvalidIdentifier) {
		t.Fatalf("invalid tunnel error = %v, want ErrInvalidIdentifier", err)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("rejected operations made %d network requests", got)
	}
}

type jwksService struct {
	*httptest.Server
	mu       sync.RWMutex
	document []byte
	requests atomic.Int32
}

func newJWKSService(t *testing.T, document []byte) *jwksService {
	t.Helper()
	service := &jwksService{document: document}
	service.Server = httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		service.requests.Add(1)
		if request.Method != http.MethodGet {
			t.Errorf("JWKS method = %s, want GET", request.Method)
		}
		service.mu.RLock()
		body := append([]byte(nil), service.document...)
		service.mu.RUnlock()
		writer.Header().Set("Cache-Control", "max-age=3600")
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(body)
	}))
	return service
}

func (service *jwksService) SetDocument(document []byte) {
	service.mu.Lock()
	service.document = append([]byte(nil), document...)
	service.mu.Unlock()
}

func (service *jwksService) Requests() int32 {
	return service.requests.Load()
}

func newTestAccessValidator(t *testing.T, service *jwksService) *AccessValidator {
	t.Helper()
	validator, err := NewAccessValidator(AccessConfig{
		Enabled:    true,
		Issuer:     testIssuer,
		Audience:   []string{testAudience},
		JWKSURL:    service.URL,
		CacheTTL:   time.Hour,
		ClockSkew:  time.Second,
		HTTPClient: service.Client(),
	})
	if err != nil {
		t.Fatalf("NewAccessValidator() error = %v", err)
	}
	return validator
}

func newTestAPIClient(t *testing.T, server *httptest.Server, dryRun bool) *APIClient {
	t.Helper()
	client, err := NewAPIClient(APIConfig{
		Enabled:    true,
		APIToken:   "test-api-token",
		AccountID:  testAccount,
		BaseURL:    server.URL + "/client/v4",
		DryRun:     dryRun,
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("NewAPIClient() error = %v", err)
	}
	return client
}

func newRSAPrivateKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	return key
}

func accessClaims(now time.Time) map[string]any {
	return map[string]any{
		"iss":   testIssuer,
		"aud":   []string{"another-audience", testAudience},
		"exp":   now.Add(time.Hour).Unix(),
		"nbf":   now.Add(-time.Minute).Unix(),
		"sub":   "user-123",
		"email": "user@example.com",
	}
}

func signedRS256(t *testing.T, privateKey *rsa.PrivateKey, keyID string, claims map[string]any) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString(mustJSON(t, map[string]string{"alg": "RS256", "kid": keyID, "typ": "JWT"}))
	payload := base64.RawURLEncoding.EncodeToString(mustJSON(t, claims))
	input := header + "." + payload
	digest := sha256.Sum256([]byte(input))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("SignPKCS1v15() error = %v", err)
	}
	return input + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func signedES256(t *testing.T, privateKey *ecdsa.PrivateKey, keyID string, claims map[string]any) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString(mustJSON(t, map[string]string{"alg": "ES256", "kid": keyID, "typ": "JWT"}))
	payload := base64.RawURLEncoding.EncodeToString(mustJSON(t, claims))
	input := header + "." + payload
	digest := sha256.Sum256([]byte(input))
	r, s, err := ecdsa.Sign(rand.Reader, privateKey, digest[:])
	if err != nil {
		t.Fatalf("ecdsa.Sign() error = %v", err)
	}
	width := 32
	signature := append(paddedBigInt(r, width), paddedBigInt(s, width)...)
	return input + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func rsaJWK(keyID string, publicKey *rsa.PublicKey) map[string]string {
	return map[string]string{
		"kty": "RSA",
		"kid": keyID,
		"alg": "RS256",
		"use": "sig",
		"n":   base64.RawURLEncoding.EncodeToString(publicKey.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(publicKey.E)).Bytes()),
	}
}

func ecJWK(keyID string, publicKey *ecdsa.PublicKey) map[string]string {
	return map[string]string{
		"kty": "EC",
		"kid": keyID,
		"alg": "ES256",
		"use": "sig",
		"crv": "P-256",
		"x":   base64.RawURLEncoding.EncodeToString(paddedBigInt(publicKey.X, 32)),
		"y":   base64.RawURLEncoding.EncodeToString(paddedBigInt(publicKey.Y, 32)),
	}
}

func jwksDocument(t *testing.T, keys ...map[string]string) []byte {
	t.Helper()
	return mustJSON(t, map[string]any{"keys": keys})
}

func paddedBigInt(value *big.Int, width int) []byte {
	encoded := value.Bytes()
	result := make([]byte, width)
	copy(result[width-len(encoded):], encoded)
	return result
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return encoded
}
