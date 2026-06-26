package health

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthWorker_HTTP(t *testing.T) {
	healthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthyServer.Close()

	failingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer failingServer.Close()

	worker := NewWorker(100*time.Millisecond, 50*time.Millisecond, "/")
	worker.Sync([]Target{
		{URL: healthyServer.URL, HealthCheck: "http"},
		{URL: failingServer.URL, HealthCheck: "http"},
	})

	if !worker.probe(Target{URL: healthyServer.URL, HealthCheck: "http"}) {
		t.Fatal("expected healthy upstream probe to succeed")
	}
	if worker.probe(Target{URL: failingServer.URL, HealthCheck: "http"}) {
		t.Fatal("expected failing upstream probe to return unhealthy")
	}

	worker.ProbeAllOnce()
	if !worker.IsHealthy(healthyServer.URL) {
		t.Fatal("expected healthy server to be marked healthy after probe cycle")
	}
	if worker.IsHealthy(failingServer.URL) {
		t.Fatal("expected failing server to be marked unhealthy after probe cycle")
	}
}

func TestNewWorker_NormalizesInvalidTiming(t *testing.T) {
	worker := NewWorker(0, -1*time.Second, "")
	if worker.interval != 10*time.Second {
		t.Fatalf("interval = %v, want %v", worker.interval, 10*time.Second)
	}
	if worker.timeout != 3*time.Second {
		t.Fatalf("timeout = %v, want %v", worker.timeout, 3*time.Second)
	}
	if worker.path != "/" {
		t.Fatalf("path = %q, want %q", worker.path, "/")
	}
}
