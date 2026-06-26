package health

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
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

func TestCheckHTTP_ReusesConnections(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("health-check-response-body"))
	}))
	defer server.Close()

	var dials atomic.Int32
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			dials.Add(1)
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		},
	}

	worker := NewWorker(time.Second, time.Second, "/")
	worker.client = &http.Client{
		Timeout:   worker.timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	target := Target{URL: server.URL, HealthCheck: "http"}
	for i := 0; i < 5; i++ {
		if !worker.probe(target) {
			t.Fatalf("probe %d: expected healthy upstream", i)
		}
	}

	if got := dials.Load(); got != 1 {
		t.Fatalf("dial count = %d, want 1 (connection should be reused)", got)
	}
}
