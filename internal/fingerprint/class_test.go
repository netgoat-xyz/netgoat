package fingerprint

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

const chromeUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

func TestCanEmit(t *testing.T) {
	if CanEmit(false, false) {
		t.Fatal("HTTP / non-terminate must not emit")
	}
	if CanEmit(true, true) {
		t.Fatal("Cloudflare origin must not emit")
	}
	if CanEmit(false, true) {
		t.Fatal("pass-through behind Cloudflare must not emit")
	}
	if !CanEmit(true, false) {
		t.Fatal("terminate-only should emit")
	}
}

func TestFromRequestEmptyWhenHTTPOrCloudflare(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
	if got := FromRequest(req); got != "" {
		t.Fatalf("HTTP request emitted %q", got)
	}

	req = httptest.NewRequest(http.MethodGet, "https://example.test/", nil)
	req.TLS = &tls.ConnectionState{HandshakeComplete: true}
	req = req.WithContext(WithStackClass(req.Context(), "t13d1516h2_abc_def|-|h2,http/1.1"))
	if got := FromRequest(req); got == "" {
		t.Fatal("terminated request should expose class")
	}
	req.Header.Set("CF-Connecting-IP", "203.0.113.10")
	if got := FromRequest(req); got != "" {
		t.Fatalf("Cloudflare origin emitted %q", got)
	}
}

func TestFromRequestEmptyWhenNotAttached(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "https://example.test/", nil)
	req.TLS = &tls.ConnectionState{HandshakeComplete: true}
	if got := FromRequest(req); got != "" {
		t.Fatalf("pass-through / no peek emitted %q", got)
	}
}

func TestMismatchBrowserUAVersusLibraryStack(t *testing.T) {
	chromeClass := "t13d1516h2_8daaf6152771_b186095e22b6|1:65536|0|0|m,a,s,p|h2,http/1.1"
	goClass := "t13d0608h2_aaaaaaaaaaaa_bbbbbbbbbbbb|-|h2,http/1.1"

	if !Mismatch(goClass, chromeUA, true) {
		t.Fatal("Go stack claiming Chrome must mismatch")
	}
	if Mismatch(chromeClass, chromeUA, true) {
		t.Fatal("Chrome-class stack agreeing with Chrome UA must not mismatch")
	}
	if Mismatch(goClass, "Go-http-client/1.1", true) {
		t.Fatal("honest Go UA must not raise from UA alone")
	}
	if Mismatch(goClass, "python-requests/2.32.0", true) {
		t.Fatal("honest python UA must not mismatch")
	}
	if Mismatch(goClass, "curl/8.6.0", true) {
		t.Fatal("honest curl UA must not mismatch")
	}
	if !Mismatch("", chromeUA, true) {
		t.Fatal("missing stack when terminated and UA looks browser must mismatch")
	}
	if Mismatch("", "Go-http-client/1.1", true) {
		t.Fatal("missing stack with honest library UA must not mismatch")
	}
	if Mismatch("", chromeUA, false) {
		t.Fatal("non-terminate must not mismatch")
	}
}

func TestMismatchFromRequestIgnoresCloudflare(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "https://example.test/", nil)
	req.TLS = &tls.ConnectionState{HandshakeComplete: true}
	req.Header.Set("User-Agent", chromeUA)
	req.Header.Set("CF-Connecting-IP", "198.51.100.4")
	if MismatchFromRequest(req) {
		t.Fatal("Cloudflare origin must not treat missing class as a mismatch")
	}
	if FromRequest(req) != "" {
		t.Fatal("Cloudflare origin must emit nothing")
	}
}

func TestWithStackClassRoundTrip(t *testing.T) {
	ctx := WithStackClass(context.Background(), "t13d1516h2_abc_def|-|h2,http/1.1")
	req := httptest.NewRequest(http.MethodGet, "https://example.test/", nil)
	req.TLS = &tls.ConnectionState{HandshakeComplete: true}
	req = req.WithContext(ctx)
	if got := FromRequest(req); got != "t13d1516h2_abc_def|-|h2,http/1.1" {
		t.Fatalf("FromRequest = %q", got)
	}
}

func TestBrowserLikeUA(t *testing.T) {
	if !BrowserLikeUA(chromeUA) {
		t.Fatal("chrome UA should look like a browser")
	}
	if BrowserLikeUA("Go-http-client/1.1") || BrowserLikeUA("python-requests/2.26.0") || BrowserLikeUA("curl/8.0.0") {
		t.Fatal("library UAs must not look like a browser")
	}
}
