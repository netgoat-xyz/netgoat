package traffic

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRateLimiterAllowsBurstThenRefills(t *testing.T) {
	limiter := NewRateLimiter(60, 2)
	now := time.Unix(100, 0)

	if !limiter.allowAt("client", now) {
		t.Fatal("first request should pass")
	}
	if !limiter.allowAt("client", now) {
		t.Fatal("second request should pass")
	}
	if limiter.allowAt("client", now) {
		t.Fatal("third request should be limited")
	}
	if !limiter.allowAt("client", now.Add(time.Second)) {
		t.Fatal("request should pass after token refill")
	}
}

func TestRateLimiterSeparatesKeys(t *testing.T) {
	limiter := NewRateLimiter(60, 1)
	now := time.Unix(100, 0)

	if !limiter.allowAt("a", now) || !limiter.allowAt("b", now) {
		t.Fatal("separate clients should get separate buckets")
	}
	if limiter.allowAt("a", now) {
		t.Fatal("same client should be limited after burst")
	}
}

func TestRateLimiterPrunesIdleBuckets(t *testing.T) {
	limiter := NewRateLimiter(60, 1)
	limiter.ttl = time.Second
	limiter.lastPrune = time.Unix(0, 0)

	if !limiter.allowAt("old", time.Unix(100, 0)) {
		t.Fatal("old bucket initial request should pass")
	}
	if !limiter.allowAt("new", time.Unix(200, 0)) {
		t.Fatal("new bucket request should pass")
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

func TestRateLimiterCapsBucketCount(t *testing.T) {
	limiter := NewRateLimiter(60, 1)
	limiter.maxBuckets = 1
	now := time.Unix(100, 0)

	if !limiter.allowAt("first", now) {
		t.Fatal("first request should pass")
	}
	if !limiter.allowAt("second", now.Add(time.Second)) {
		t.Fatal("second request should pass")
	}

	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	if len(limiter.buckets) != 1 {
		t.Fatalf("bucket count = %d, want 1", len(limiter.buckets))
	}
	if _, ok := limiter.buckets["second"]; !ok {
		t.Fatal("newest bucket should be retained")
	}
}

func TestQueueRejectsWhenFull(t *testing.T) {
	queue := NewQueue(1, 0, time.Second)

	release, err := queue.Acquire(context.Background())
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	defer release()

	if _, err := queue.Acquire(context.Background()); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("Acquire error = %v, want ErrQueueFull", err)
	}
}

func TestQueueWaitsForRelease(t *testing.T) {
	queue := NewQueue(1, 1, time.Second)

	release, err := queue.Acquire(context.Background())
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		nextRelease, err := queue.Acquire(context.Background())
		if err == nil {
			nextRelease()
		}
		done <- err
	}()

	release()
	if err := <-done; err != nil {
		t.Fatalf("queued acquire failed: %v", err)
	}
}

func TestQueueTimesOut(t *testing.T) {
	queue := NewQueue(1, 1, time.Millisecond)

	release, err := queue.Acquire(context.Background())
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	defer release()

	if _, err := queue.Acquire(context.Background()); !errors.Is(err, ErrQueueWait) {
		t.Fatalf("Acquire error = %v, want ErrQueueWait", err)
	}
}
