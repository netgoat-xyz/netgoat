package main

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"testing"
	"time"

	"netgoat.xyz/agent/internal/cache"

	"netgoat.xyz/agent/internal/challenge"
	"netgoat.xyz/agent/internal/config"
	"netgoat.xyz/agent/internal/database"
	"netgoat.xyz/agent/internal/streaming"
)

func TestProxyErrorHandlerReturnsBadGatewayOnConnectRefused(t *testing.T) {
	// Choose a port that is very likely closed by binding to :0, closing, then using it.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	targetURL, _ := url.Parse("http://" + addr)
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.Transport = newStableProxyTransport()

	pages := &errorPageStore{}
	store := challenge.NewStore()

	proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, perr error) {
		writeError(rw, pages, store, req, http.StatusBadGateway, "Bad Gateway")
	}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	rr := httptest.NewRecorder()

	proxy.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadGateway)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("content-type = %q, want contains text/html", ct)
	}
}

func TestDirectorSetsForwardedHeaders(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Seen-XFF", r.Header.Get("X-Forwarded-For"))
		w.Header().Set("X-Seen-XFH", r.Header.Get("X-Forwarded-Host"))
		w.Header().Set("X-Seen-XFP", r.Header.Get("X-Forwarded-Proto"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer up.Close()

	targetURL, _ := url.Parse(up.URL)
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.Transport = newStableProxyTransport()

	originalHost := "app.example.com:8080"
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = targetURL.Host
		if req.Header.Get("X-Forwarded-Host") == "" && originalHost != "" {
			req.Header.Set("X-Forwarded-Host", originalHost)
		}
		req.Header.Set("X-Forwarded-Proto", "http")
	}

	req := httptest.NewRequest(http.MethodGet, "http://"+originalHost+"/", nil)
	req.Host = originalHost
	req.Header.Set("X-Forwarded-For", "198.51.100.1")
	req.RemoteAddr = "203.0.113.10:12345"
	rr := httptest.NewRecorder()

	proxy.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if got := rr.Header().Get("X-Seen-XFH"); got != originalHost {
		t.Fatalf("X-Forwarded-Host seen = %q, want %q", got, originalHost)
	}
	if got := rr.Header().Get("X-Seen-XFP"); got != "http" {
		t.Fatalf("X-Forwarded-Proto seen = %q, want %q", got, "http")
	}
	if got := rr.Header().Get("X-Seen-XFF"); got != "198.51.100.1, 203.0.113.10" {
		t.Fatalf("X-Forwarded-For seen = %q, want %q", got, "198.51.100.1, 203.0.113.10")
	}
}

func TestShouldInjectOverlaySkipsCompressedOrUnknownLength(t *testing.T) {
	res := &http.Response{
		Header:        make(http.Header),
		ContentLength: 10,
		Body:          io.NopCloser(strings.NewReader("<html></html>")),
	}
	res.Header.Set("Content-Type", "text/html; charset=utf-8")
	res.Header.Set("Content-Encoding", "gzip")
	if shouldInjectOverlay(res) {
		t.Fatalf("shouldInjectOverlay(gzip) = true, want false")
	}

	res2 := &http.Response{
		Header:        make(http.Header),
		ContentLength: -1,
		Body:          io.NopCloser(strings.NewReader("<html></html>")),
	}
	res2.Header.Set("Content-Type", "text/html")
	if shouldInjectOverlay(res2) {
		t.Fatalf("shouldInjectOverlay(unknown length) = true, want false")
	}
}

func TestSharedCacheRequiresPublicAnonymousRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	store := cache.NewStore(time.Minute, 10, 1024)
	if !isRequestCacheableForSharedStore(store, req) {
		t.Fatal("plain GET should be eligible for shared cache lookup")
	}

	req.Header.Set("Authorization", "Bearer secret")
	if isRequestCacheableForSharedStore(store, req) {
		t.Fatal("authorized request should not use shared cache")
	}
}

func TestSharedCacheableResponseRejectsPrivateState(t *testing.T) {
	res := &http.Response{Header: make(http.Header), StatusCode: http.StatusOK}
	res.Header.Set("Cache-Control", "public, max-age=60")
	if !isSharedCacheableResponse(res) {
		t.Fatal("public response should be cacheable")
	}

	res.Header.Set("Set-Cookie", "session=abc")
	if isSharedCacheableResponse(res) {
		t.Fatal("Set-Cookie response should not be cacheable")
	}

	res.Header.Del("Set-Cookie")
	res.Header.Set("Cache-Control", "private, max-age=60")
	if isSharedCacheableResponse(res) {
		t.Fatal("private response should not be cacheable")
	}
}

func TestLocalConfigSnapshotAppliesDocumentedRoutes(t *testing.T) {
	db, err := database.Init(":memory:")
	if err != nil {
		t.Fatalf("database.Init: %v", err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()

	cfg := &config.Config{Routes: map[string]config.Route{
		"local.example.test": {
			Type: "domain",
			Targets: []config.RouteTarget{
				{URL: "http://127.0.0.1:9001/base", HealthCheck: "http"},
				{URL: "http://127.0.0.1:9002", HealthCheck: "tcp"},
			},
		},
	}}

	snapshot := localConfigSnapshot(cfg)
	applySnapshotToDB(db, snapshot)
	match, err := database.GetRouteTargets(db, "LOCAL.EXAMPLE.TEST", "/")
	if err != nil {
		t.Fatalf("GetRouteTargets: %v", err)
	}
	if len(match.Targets) != 2 || match.Targets[0].URL != "http://127.0.0.1:9001/base" || match.Targets[1].HealthCheck != "tcp" {
		t.Fatalf("local route targets = %+v", match.Targets)
	}
}

func TestEmptyRecoverySnapshotHasNoContent(t *testing.T) {
	if snapshotHasContent(nil) {
		t.Fatal("nil snapshot should be empty")
	}
	if snapshotHasContent(&streaming.ConfigSnapshot{}) {
		t.Fatal("zero-value snapshot should not override local state")
	}
}
