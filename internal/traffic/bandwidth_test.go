package traffic

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBandwidthLimiterAllowsBurst(t *testing.T) {
	limiter := NewBandwidthLimiter(1024, 16)

	start := time.Now()
	if err := limiter.Wait(context.Background(), "client", 16); err != nil {
		t.Fatalf("Wait failed: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 20*time.Millisecond {
		t.Fatalf("burst wait took %s, want near-immediate", elapsed)
	}
}

func TestBandwidthLimiterHonorsContext(t *testing.T) {
	limiter := NewBandwidthLimiter(1, 1)
	ctx, cancel := context.WithCancel(context.Background())

	if err := limiter.Wait(ctx, "client", 1); err != nil {
		t.Fatalf("first Wait failed: %v", err)
	}
	cancel()
	if err := limiter.Wait(ctx, "client", 1); err == nil {
		t.Fatal("Wait should fail after context cancellation")
	}
}

func TestBandwidthLimiterPrunesIdleBuckets(t *testing.T) {
	limiter := NewBandwidthLimiter(1024, 1024)
	limiter.ttl = time.Second
	limiter.lastPrune = time.Unix(0, 0)

	if wait := limiter.reserve("old", 1, time.Unix(100, 0)); wait > 0 {
		t.Fatalf("old reserve wait = %s, want immediate", wait)
	}
	if wait := limiter.reserve("new", 1, time.Unix(200, 0)); wait > 0 {
		t.Fatalf("new reserve wait = %s, want immediate", wait)
	}

	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	if _, ok := limiter.buckets["old"]; ok {
		t.Fatal("old bucket should be pruned")
	}
	if _, ok := limiter.buckets["new"]; !ok {
		t.Fatal("new bucket should remain")
	}
}

func TestThrottledReadCloser(t *testing.T) {
	limiter := NewBandwidthLimiter(1024, 1024)
	body := io.NopCloser(bytes.NewBufferString("hello"))
	wrapped := WrapReadCloser(body, limiter, "upload", context.Background())

	data, err := io.ReadAll(wrapped)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("body = %q, want hello", data)
	}
}

func TestBandwidthResponseWriter(t *testing.T) {
	limiter := NewBandwidthLimiter(1024, 1024)
	rec := httptest.NewRecorder()
	wrapped := WrapResponseWriter(rec, limiter, "download", context.Background())

	n, err := wrapped.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != 5 {
		t.Fatalf("Write bytes = %d, want 5", n)
	}
	if rec.Body.String() != "hello" {
		t.Fatalf("body = %q, want hello", rec.Body.String())
	}
	if _, ok := wrapped.(http.Flusher); !ok {
		t.Fatal("wrapped writer should expose Flusher")
	}
}
