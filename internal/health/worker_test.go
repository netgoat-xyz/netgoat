package health

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
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

func TestCheckHTTP_BoundsResponseDrain(t *testing.T) {
	body := &endlessResponseBody{}
	worker := NewWorker(time.Second, time.Second, "/health")
	worker.client = &http.Client{
		Timeout: worker.timeout,
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       body,
				Request:    req,
			}, nil
		}),
	}

	if !worker.checkHTTP("http://upstream.invalid") {
		t.Fatal("expected bounded response to retain healthy status")
	}
	if got, want := body.bytesRead, int64(maxProbeResponseDrainSize+1); got != want {
		t.Fatalf("response bytes read = %d, want %d", got, want)
	}
	if !body.closed {
		t.Fatal("expected response body to be closed")
	}
}

func TestProbeAll_BoundsConcurrency(t *testing.T) {
	var active atomic.Int32
	var maximum atomic.Int32
	entered := make(chan struct{}, maxConcurrentProbes*3)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseAll := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseAll()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := active.Add(1)
		defer active.Add(-1)
		for {
			previous := maximum.Load()
			if current <= previous || maximum.CompareAndSwap(previous, current) {
				break
			}
		}
		entered <- struct{}{}
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	worker := NewWorker(time.Second, 2*time.Second, "/health")
	targets := make([]Target, maxConcurrentProbes*3)
	for i := range targets {
		targets[i] = Target{
			URL:         fmt.Sprintf("%s?probe=%d", server.URL, i),
			HealthCheck: "http",
		}
	}
	worker.Sync(targets)

	done := make(chan struct{})
	go func() {
		worker.ProbeAllOnce()
		close(done)
	}()

	for range maxConcurrentProbes {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for concurrent probes")
		}
	}
	select {
	case <-entered:
		t.Fatal("probe concurrency exceeded configured bound")
	case <-time.After(50 * time.Millisecond):
	}

	releaseAll()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("probe cycle did not finish")
	}
	if got := maximum.Load(); got != maxConcurrentProbes {
		t.Fatalf("maximum concurrent probes = %d, want %d", got, maxConcurrentProbes)
	}
}

func TestProbeAll_SerializesOverlappingCycles(t *testing.T) {
	var calls atomic.Int32
	firstEntered := make(chan struct{})
	secondEntered := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseFirst := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseFirst()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch calls.Add(1) {
		case 1:
			close(firstEntered)
			<-release
		case 2:
			close(secondEntered)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	worker := NewWorker(time.Second, 2*time.Second, "/health")
	worker.Sync([]Target{{URL: server.URL, HealthCheck: "http"}})

	firstDone := make(chan struct{})
	go func() {
		worker.ProbeAllOnce()
		close(firstDone)
	}()
	select {
	case <-firstEntered:
	case <-time.After(time.Second):
		t.Fatal("first probe cycle did not start")
	}

	secondDone := make(chan struct{})
	go func() {
		worker.ProbeAllOnce()
		close(secondDone)
	}()
	select {
	case <-secondEntered:
		t.Fatal("second probe cycle overlapped the first")
	case <-time.After(50 * time.Millisecond):
	}

	releaseFirst()
	for name, done := range map[string]<-chan struct{}{
		"first":  firstDone,
		"second": secondDone,
	} {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("%s probe cycle did not finish", name)
		}
	}
}

func TestProbeAll_DoesNotRestoreRemovedTarget(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseProbe := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseProbe()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	worker := NewWorker(time.Second, 2*time.Second, "/health")
	worker.Sync([]Target{{URL: server.URL, HealthCheck: "http"}})
	done := make(chan struct{})
	go func() {
		worker.ProbeAllOnce()
		close(done)
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("probe did not start")
	}

	worker.Sync(nil)
	releaseProbe()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("probe did not finish")
	}

	worker.mu.RLock()
	defer worker.mu.RUnlock()
	if _, exists := worker.healthy[server.URL]; exists {
		t.Fatal("removed target was restored to healthy state by a stale probe")
	}
	if _, exists := worker.checked[server.URL]; exists {
		t.Fatal("removed target was restored to checked state by a stale probe")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

type endlessResponseBody struct {
	bytesRead int64
	closed    bool
}

func (b *endlessResponseBody) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	b.bytesRead += int64(len(p))
	return len(p), nil
}

func (b *endlessResponseBody) Close() error {
	b.closed = true
	return nil
}
