package balancer

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"netgoat.xyz/agent/internal/health"
)

func TestProxyHandlerOverwritesClientSuppliedForwardedMetadata(t *testing.T) {
	seen := make(chan http.Header, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- r.Header.Clone()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	handler := NewProxyHandler(New(health.NewWorker(time.Second, time.Second, "/")), nil)
	req := httptest.NewRequest(http.MethodGet, "http://app.example.test/path", nil)
	req.Host = "app.example.test"
	req.Header.Set("X-Forwarded-Host", "attacker.test")
	req.Header.Set("X-Forwarded-Proto", "https")
	response := httptest.NewRecorder()
	if err := handler.Serve(response, req, "route", []string{upstream.URL}, nil); err != nil {
		t.Fatal(err)
	}
	headers := <-seen
	if got := headers.Get("X-Forwarded-Host"); got != "app.example.test" {
		t.Fatalf("X-Forwarded-Host = %q", got)
	}
	if got := headers.Get("X-Forwarded-Proto"); got != "http" {
		t.Fatalf("X-Forwarded-Proto = %q", got)
	}
}
