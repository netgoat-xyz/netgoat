package balancer

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"netgoat.xyz/agent/internal/health"
)

func TestBalancer_RoundRobinAndFailover(t *testing.T) {
	healthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthyServer.Close()

	unhealthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer unhealthyServer.Close()

	targets := []string{healthyServer.URL, unhealthyServer.URL}
	worker := health.NewWorker(time.Second, time.Second, "/")
	worker.Sync([]health.Target{
		{URL: healthyServer.URL, HealthCheck: "http"},
		{URL: unhealthyServer.URL, HealthCheck: "http"},
	})
	worker.ProbeAllOnce()

	b := New(worker)
	routeKey := "test-route"

	picked, err := b.Pick(routeKey, targets)
	if err != nil {
		t.Fatalf("Pick() error = %v", err)
	}
	if picked != healthyServer.URL {
		t.Fatalf("Pick() = %q, want only healthy target %q", picked, healthyServer.URL)
	}

	// Round-robin across two healthy targets.
	worker.Sync([]health.Target{
		{URL: healthyServer.URL, HealthCheck: "http"},
		{URL: "http://node-2:8080", HealthCheck: "http"},
	})
	rrTargets := []string{healthyServer.URL, "http://node-2:8080"}

	first, err := b.Pick(routeKey, rrTargets)
	if err != nil {
		t.Fatalf("Pick() error = %v", err)
	}
	second, err := b.Pick(routeKey, rrTargets)
	if err != nil {
		t.Fatalf("Pick() error = %v", err)
	}
	if first == second {
		t.Fatalf("round-robin returned same target twice: %q", first)
	}
}

func TestProxyHandler_FailoverOn5xx(t *testing.T) {
	failServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer failServer.Close()

	okServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer okServer.Close()

	worker := health.NewWorker(time.Second, time.Second, "/")
	worker.Sync([]health.Target{
		{URL: failServer.URL, HealthCheck: "http"},
		{URL: okServer.URL, HealthCheck: "http"},
	})

	handler := NewProxyHandler(New(worker), nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)

	targets := []string{failServer.URL, okServer.URL}
	if err := handler.Serve(rec, req, "route", targets, nil); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); body != "ok" {
		t.Fatalf("body = %q, want %q", body, "ok")
	}
}

func TestProxyHandler_NoFailoverForPost(t *testing.T) {
	failServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer failServer.Close()

	okServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer okServer.Close()

	worker := health.NewWorker(time.Second, time.Second, "/")
	handler := NewProxyHandler(New(worker), nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "http://example.com/", nil)

	targets := []string{failServer.URL, okServer.URL}
	if err := handler.Serve(rec, req, "route", targets, nil); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("POST should not failover; status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

type flushRecorder struct {
	httptest.ResponseRecorder
	flushes int
}

func (f *flushRecorder) Flush() {
	f.flushes++
}

func TestStreamWriter_Flusher(t *testing.T) {
	t.Run("delegates flush to underlying writer", func(t *testing.T) {
		rec := &flushRecorder{}
		sw := &streamWriter{w: rec, header: make(http.Header)}
		sw.WriteHeader(http.StatusOK)

		if _, ok := any(sw).(http.Flusher); !ok {
			t.Fatal("streamWriter should implement http.Flusher")
		}
		sw.Flush()
		if rec.flushes != 1 {
			t.Fatalf("Flush() calls = %d, want 1", rec.flushes)
		}
	})

	t.Run("does not flush while buffering retryable 5xx", func(t *testing.T) {
		rec := &flushRecorder{}
		sw := &streamWriter{w: rec, header: make(http.Header)}
		sw.WriteHeader(http.StatusInternalServerError)
		sw.Flush()
		if rec.flushes != 0 {
			t.Fatalf("Flush() during retry buffering = %d, want 0", rec.flushes)
		}
	})

	t.Run("unwraps to underlying writer", func(t *testing.T) {
		rec := &flushRecorder{}
		sw := &streamWriter{w: rec, header: make(http.Header)}
		if err := http.NewResponseController(sw).Flush(); err != nil {
			t.Fatalf("ResponseController.Flush() error = %v", err)
		}
		if rec.flushes != 1 {
			t.Fatalf("Flush() via Unwrap = %d, want 1", rec.flushes)
		}
	})
}
