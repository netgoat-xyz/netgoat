package tlsmanager

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

func TestManagerSelectsExactWildcardAndFallback(t *testing.T) {
	_, _, fallback := newTestCertificate(t, "fallback.test")
	exactPEM, exactKey, _ := newTestCertificate(t, "api.example.test")
	wildcardPEM, wildcardKey, _ := newTestCertificate(t, "wildcard.example.test")

	manager := New(&fallback)
	if err := manager.Reload([]DomainCertificate{
		{
			Domain:         "API.EXAMPLE.TEST.",
			CertificatePEM: exactPEM,
			PrivateKeyPEM:  exactKey,
		},
		{
			Domain:         "*.example.test",
			CertificatePEM: wildcardPEM,
			PrivateKeyPEM:  wildcardKey,
		},
	}); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}

	for _, testCase := range []struct {
		name       string
		serverName string
		want       string
	}{
		{name: "exact certificate takes precedence", serverName: "api.example.test", want: "api.example.test"},
		{name: "case and trailing dot normalize", serverName: "API.EXAMPLE.TEST.", want: "api.example.test"},
		{name: "wildcard direct child", serverName: "www.example.test", want: "wildcard.example.test"},
		{name: "wildcard does not span labels", serverName: "a.b.example.test", want: "fallback.test"},
		{name: "unknown domain uses fallback", serverName: "other.test", want: "fallback.test"},
		{name: "missing SNI uses fallback", want: "fallback.test"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			certificate, err := manager.GetCertificate(&tls.ClientHelloInfo{ServerName: testCase.serverName})
			if err != nil {
				t.Fatalf("GetCertificate() error = %v", err)
			}
			if got := certificateName(t, certificate); got != testCase.want {
				t.Errorf("selected certificate = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestReloadRetainsLastKnownGoodCertificateForInvalidPEM(t *testing.T) {
	_, _, fallback := newTestCertificate(t, "fallback.test")
	oldPEM, oldKey, _ := newTestCertificate(t, "old.api.test")
	retainedPEM, retainedKey, _ := newTestCertificate(t, "retained.example.test")
	newPEM, newKey, _ := newTestCertificate(t, "new.api.test")

	manager := New(&fallback)
	if err := manager.Reload([]DomainCertificate{
		{Domain: "api.example.test", CertificatePEM: oldPEM, PrivateKeyPEM: oldKey},
		{Domain: "retained.example.test", CertificatePEM: retainedPEM, PrivateKeyPEM: retainedKey},
	}); err != nil {
		t.Fatalf("initial Reload() error = %v", err)
	}

	err := manager.Reload([]DomainCertificate{
		{Domain: "api.example.test", CertificatePEM: newPEM, PrivateKeyPEM: newKey},
		{Domain: "retained.example.test", CertificatePEM: "not a PEM certificate", PrivateKeyPEM: "not a PEM key"},
	})
	if err == nil {
		t.Fatal("Reload() error = nil, want malformed PEM error")
	}

	if certificate, err := manager.GetCertificate(&tls.ClientHelloInfo{ServerName: "api.example.test"}); err != nil {
		t.Fatalf("GetCertificate(api) error = %v", err)
	} else if got := certificateName(t, certificate); got != "new.api.test" {
		t.Errorf("updated certificate = %q, want new.api.test", got)
	}
	if certificate, err := manager.GetCertificate(&tls.ClientHelloInfo{ServerName: "retained.example.test"}); err != nil {
		t.Fatalf("GetCertificate(retained) error = %v", err)
	} else if got := certificateName(t, certificate); got != "retained.example.test" {
		t.Errorf("retained certificate = %q, want retained.example.test", got)
	}

	if err := manager.Reload(nil); err != nil {
		t.Fatalf("Reload(nil) error = %v", err)
	}
	if certificate, err := manager.GetCertificate(&tls.ClientHelloInfo{ServerName: "api.example.test"}); err != nil {
		t.Fatalf("GetCertificate() after removal error = %v", err)
	} else if got := certificateName(t, certificate); got != "fallback.test" {
		t.Errorf("certificate after omitted record = %q, want fallback.test", got)
	}
}

func TestReloadAndCertificateSelectionAreConcurrentSafe(t *testing.T) {
	_, _, fallback := newTestCertificate(t, "fallback.test")
	firstPEM, firstKey, _ := newTestCertificate(t, "first.example.test")
	secondPEM, secondKey, _ := newTestCertificate(t, "second.example.test")

	manager := New(&fallback)
	if err := manager.Reload([]DomainCertificate{{
		Domain:         "api.example.test",
		CertificatePEM: firstPEM,
		PrivateKeyPEM:  firstKey,
	}}); err != nil {
		t.Fatalf("initial Reload() error = %v", err)
	}

	start := make(chan struct{})
	errorChannel := make(chan error, 9)
	var workers sync.WaitGroup
	for range 8 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			for range 250 {
				certificate, err := manager.GetCertificate(&tls.ClientHelloInfo{ServerName: "api.example.test"})
				if err != nil {
					errorChannel <- err
					return
				}
				if certificate == nil || len(certificate.Certificate) == 0 {
					errorChannel <- errors.New("GetCertificate returned an empty certificate")
					return
				}
			}
		}()
	}
	workers.Add(1)
	go func() {
		defer workers.Done()
		<-start
		for index := range 100 {
			certificatePEM, privateKeyPEM := firstPEM, firstKey
			if index%2 == 1 {
				certificatePEM, privateKeyPEM = secondPEM, secondKey
			}
			if err := manager.Reload([]DomainCertificate{{
				Domain:         "api.example.test",
				CertificatePEM: certificatePEM,
				PrivateKeyPEM:  privateKeyPEM,
			}}); err != nil {
				errorChannel <- err
				return
			}
		}
	}()

	close(start)
	workers.Wait()
	close(errorChannel)
	for err := range errorChannel {
		t.Errorf("concurrent TLS manager operation failed: %v", err)
	}
}

func TestEncryptedCacheRoundTripAndTamperResistance(t *testing.T) {
	key := bytes.Repeat([]byte{0x7a}, 32)
	cache, err := NewEncryptedCache(t.TempDir(), key)
	if err != nil {
		t.Fatalf("NewEncryptedCache() error = %v", err)
	}

	ctx := context.Background()
	cacheKey := "cert-example.test"
	payload := []byte("private ACME material must never be written in plaintext")
	if err := cache.Put(ctx, cacheKey, payload); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	encoded, err := os.ReadFile(cache.pathFor(cacheKey))
	if err != nil {
		t.Fatalf("read encrypted entry: %v", err)
	}
	if bytes.Contains(encoded, payload) {
		t.Fatal("encrypted cache entry contains plaintext payload")
	}
	if got, err := cache.Get(ctx, cacheKey); err != nil {
		t.Fatalf("Get() error = %v", err)
	} else if !bytes.Equal(got, payload) {
		t.Errorf("Get() = %q, want %q", got, payload)
	}

	encoded[len(encoded)-1] ^= 0x01
	if err := os.WriteFile(cache.pathFor(cacheKey), encoded, 0o600); err != nil {
		t.Fatalf("tamper cache entry: %v", err)
	}
	if _, err := cache.Get(ctx, cacheKey); err == nil {
		t.Fatal("Get() after tampering error = nil, want authentication failure")
	} else if errors.Is(err, autocert.ErrCacheMiss) {
		t.Fatalf("Get() after tampering error = %v, must not be cache miss", err)
	}
}

func TestEncryptedCacheBindsEntriesToTheirCacheKey(t *testing.T) {
	cache, err := NewEncryptedCache(t.TempDir(), bytes.Repeat([]byte{0x5b}, 32))
	if err != nil {
		t.Fatalf("NewEncryptedCache() error = %v", err)
	}
	ctx := context.Background()
	if err := cache.Put(ctx, "first", []byte("first data")); err != nil {
		t.Fatalf("Put(first) error = %v", err)
	}
	if err := cache.Put(ctx, "second", []byte("second data")); err != nil {
		t.Fatalf("Put(second) error = %v", err)
	}
	first, err := os.ReadFile(cache.pathFor("first"))
	if err != nil {
		t.Fatalf("read first entry: %v", err)
	}
	if err := os.WriteFile(cache.pathFor("second"), first, 0o600); err != nil {
		t.Fatalf("replace second entry: %v", err)
	}
	if _, err := cache.Get(ctx, "second"); err == nil {
		t.Fatal("Get(second) after substituted entry error = nil, want authentication failure")
	}
}

func TestNewEncryptedCacheFromEnvRequiresBase64AES256Key(t *testing.T) {
	t.Setenv(CacheKeyEnvironment, "not base64")
	if _, err := NewEncryptedCacheFromEnv(t.TempDir()); err == nil {
		t.Fatal("NewEncryptedCacheFromEnv() error = nil, want invalid base64 error")
	}

	expectedKey := bytes.Repeat([]byte{0x24}, 32)
	t.Setenv(CacheKeyEnvironment, base64.StdEncoding.EncodeToString(expectedKey))
	cache, err := NewEncryptedCacheFromEnv(t.TempDir())
	if err != nil {
		t.Fatalf("NewEncryptedCacheFromEnv() error = %v", err)
	}
	if err := cache.Put(context.Background(), "round-trip", []byte("ok")); err != nil {
		t.Fatalf("cache created from environment Put() error = %v", err)
	}
}

func TestEnableACMERequiresEncryptedCacheKeyAndConfiguresALPN(t *testing.T) {
	manager := New(nil)
	t.Setenv(CacheKeyEnvironment, "")
	if err := manager.EnableACME(ACMEConfig{
		CacheDir: t.TempDir(),
		Hosts:    []string{"acme.example.test"},
		Prompt:   autocert.AcceptTOS,
	}); err == nil {
		t.Fatal("EnableACME() error = nil without cache key")
	}

	t.Setenv(CacheKeyEnvironment, base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x33}, 32)))
	if err := manager.EnableACME(ACMEConfig{
		CacheDir: t.TempDir(),
		Hosts:    []string{"ACME.EXAMPLE.TEST."},
		Prompt:   autocert.AcceptTOS,
	}); err != nil {
		t.Fatalf("EnableACME() error = %v", err)
	}
	if !containsProtocol(manager.TLSConfig().NextProtos, acme.ALPNProto) {
		t.Errorf("TLSConfig().NextProtos = %v, missing %q", manager.TLSConfig().NextProtos, acme.ALPNProto)
	}
}

func newTestCertificate(t *testing.T, commonName string) (string, string, tls.Certificate) {
	t.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate private key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: commonName},
		DNSNames:     []string{commonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}
	certificatePEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	privateKeyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))
	certificate, err := tls.X509KeyPair([]byte(certificatePEM), []byte(privateKeyPEM))
	if err != nil {
		t.Fatalf("load generated certificate: %v", err)
	}
	return certificatePEM, privateKeyPEM, certificate
}

func certificateName(t *testing.T, certificate *tls.Certificate) string {
	t.Helper()
	if certificate == nil || len(certificate.Certificate) == 0 {
		t.Fatal("certificate is empty")
	}
	parsed, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	return parsed.Subject.CommonName
}

func containsProtocol(protocols []string, want string) bool {
	for _, protocol := range protocols {
		if protocol == want {
			return true
		}
	}
	return false
}
