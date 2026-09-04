package fingerprint

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestTerminatedHTTPSEmitsStableClass(t *testing.T) {
	var mu sync.Mutex
	var classes []string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		classes = append(classes, FromRequest(r))
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	url, closeServer := startFingerprintServer(t, handler)
	defer closeServer()

	client := fingerprintClient(t, []string{"h2", "http/1.1"})
	for i := 0; i < 2; i++ {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			t.Fatal(err)
		}
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		if res.StatusCode != http.StatusNoContent {
			t.Fatalf("status = %d", res.StatusCode)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(classes) != 2 || classes[0] == "" || classes[0] != classes[1] {
		t.Fatalf("two chrome-class (Go) hellos produced %q", classes)
	}
	if splitClass(classes[0])[1] == "-" {
		t.Fatalf("h2 connection should record Akamai H2 field, got %q", classes[0])
	}
}

func TestH1OnlyConnectionUsesDashH2Field(t *testing.T) {
	var got string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = FromRequest(r)
		w.WriteHeader(http.StatusNoContent)
	})
	url, closeServer := startFingerprintServer(t, handler)
	defer closeServer()

	client := fingerprintClient(t, []string{"http/1.1"})
	res, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if !hasH2Field(got, "-") {
		t.Fatalf("H1-only class = %q, want H2 field -", got)
	}
}

func TestHTTPListenerEmitsNothing(t *testing.T) {
	var got string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = FromRequest(r)
		w.WriteHeader(http.StatusNoContent)
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{
		Handler: handler,
		ConnContext: func(ctx context.Context, c net.Conn) context.Context {
			return WithConn(ctx, c)
		},
	}
	go func() { _ = server.Serve(ln) }()
	defer server.Close()

	res, err := http.Get("http://" + ln.Addr().String() + "/")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if got != "" {
		t.Fatalf("plaintext HTTP emitted %q", got)
	}
}

func TestCloudflareHeaderSuppressesClass(t *testing.T) {
	var got string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = FromRequest(r)
		w.WriteHeader(http.StatusNoContent)
	})
	url, closeServer := startFingerprintServer(t, handler)
	defer closeServer()

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("CF-Connecting-IP", "203.0.113.50")
	res, err := fingerprintClient(t, nil).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if got != "" {
		t.Fatalf("behind Cloudflare emitted %q", got)
	}
}

func TestGetCertificateUnchanged(t *testing.T) {
	var calls int
	cert := selfSignedCert(t, "api.example.test")
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
		TLSConfig: &tls.Config{
			GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
				calls++
				if hello.ServerName != "api.example.test" {
					t.Errorf("GetCertificate SNI = %q", hello.ServerName)
				}
				return &cert, nil
			},
			MinVersion: tls.VersionTLS12,
		},
		ConnContext: func(ctx context.Context, c net.Conn) context.Context {
			return WithConn(ctx, c)
		},
	}
	if err := ConfigureHTTP2(server); err != nil {
		t.Fatal(err)
	}
	go func() { _ = server.ServeTLS(WrapListener(ln), "", "") }()
	defer server.Close()

	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true, ServerName: "api.example.test"},
	}}
	res, err := client.Get("https://" + ln.Addr().String() + "/")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if calls == 0 {
		t.Fatal("GetCertificate was not used")
	}
}

func startFingerprintServer(t *testing.T, handler http.Handler) (string, func()) {
	t.Helper()
	cert := selfSignedCert(t, "test.example")
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{
		Handler: handler,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
				return &cert, nil
			},
			MinVersion: tls.VersionTLS12,
		},
		ConnContext: func(ctx context.Context, c net.Conn) context.Context {
			return WithConn(ctx, c)
		},
	}
	if err := ConfigureHTTP2(server); err != nil {
		t.Fatal(err)
	}
	go func() { _ = server.ServeTLS(WrapListener(ln), "", "") }()
	return "https://" + ln.Addr().String(), func() { _ = server.Close() }
}

func fingerprintClient(t *testing.T, nextProtos []string) *http.Client {
	t.Helper()
	cfg := &tls.Config{InsecureSkipVerify: true, ServerName: "test.example"}
	if len(nextProtos) > 0 {
		cfg.NextProtos = append([]string(nil), nextProtos...)
	}
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig:   cfg,
			DisableKeepAlives: true,
			ForceAttemptHTTP2: len(nextProtos) == 0 || containsString(nextProtos, "h2"),
		},
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func selfSignedCert(t *testing.T, name string) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: name},
		DNSNames:     []string{name},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}
