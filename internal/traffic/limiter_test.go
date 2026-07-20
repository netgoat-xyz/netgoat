package traffic

import (
	"context"
	"errors"
	"fmt"
	"sync"
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
	_, oldExists := limiter.buckets["old"]
	_, newExists := limiter.buckets["new"]
	limiter.mu.Unlock()
	if oldExists {
		t.Fatal("old bucket should be pruned")
	}
	if !newExists {
		t.Fatal("new bucket should remain")
	}
	assertRateLimiterIndexConsistent(t, limiter, 1)
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
	if got := limiter.recency.Len(); got != 1 {
		t.Fatalf("recency entries = %d, want 1", got)
	}
}

func TestRateLimiterRefreshesLeastRecentlyUsedBucket(t *testing.T) {
	limiter := NewRateLimiter(1, 1)
	limiter.maxBuckets = 2
	now := time.Unix(100, 0)

	if !limiter.allowAt("active", now) {
		t.Fatal("active initial request should pass")
	}
	if !limiter.allowAt("idle", now.Add(time.Second)) {
		t.Fatal("idle initial request should pass")
	}
	if limiter.allowAt("active", now.Add(2*time.Second)) {
		t.Fatal("active refresh should remain rate limited")
	}
	if !limiter.allowAt("new", now.Add(3*time.Second)) {
		t.Fatal("new request should pass")
	}

	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	if _, ok := limiter.buckets["idle"]; ok {
		t.Fatal("least recently used bucket should be evicted")
	}
	if _, ok := limiter.buckets["active"]; !ok {
		t.Fatal("recently refreshed bucket should be retained")
	}
	if _, ok := limiter.buckets["new"]; !ok {
		t.Fatal("new bucket should be retained")
	}
}

func TestRateLimiterBoundsAttackerControlledKeys(t *testing.T) {
	limiter := NewRateLimiter(60, 1)
	limiter.maxBuckets = 32
	now := time.Unix(100, 0)

	for i := 0; i < 10_000; i++ {
		if !limiter.allowAt(fmt.Sprintf("attacker-%d", i), now.Add(time.Duration(i))) {
			t.Fatalf("new key %d should receive its initial token", i)
		}
	}

	assertRateLimiterIndexConsistent(t, limiter, 32)
}

func TestRateLimiterConcurrentCapacityChurn(t *testing.T) {
	limiter := NewRateLimiter(6000, 10)
	limiter.maxBuckets = 64

	const (
		goroutines = 32
		iterations = 500
	)
	start := make(chan struct{})
	var workers sync.WaitGroup
	workers.Add(goroutines)
	for workerID := range goroutines {
		go func() {
			defer workers.Done()
			<-start
			for i := range iterations {
				limiter.Allow(fmt.Sprintf("client-%d", (workerID*iterations+i)%256))
			}
		}()
	}
	close(start)
	workers.Wait()

	assertRateLimiterIndexConsistent(t, limiter, limiter.maxBuckets)
}

func assertRateLimiterIndexConsistent(t *testing.T, limiter *RateLimiter, want int) {
	t.Helper()
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	if got := len(limiter.buckets); got != want {
		t.Fatalf("bucket count = %d, want %d", got, want)
	}
	if got := limiter.recency.Len(); got != len(limiter.buckets) {
		t.Fatalf("recency entries = %d, want %d", got, len(limiter.buckets))
	}
	for key, b := range limiter.buckets {
		if b.rateElement == nil {
			t.Fatalf("bucket %q has no recency entry", key)
		}
		if got, ok := b.rateElement.Value.(string); !ok || got != key {
			t.Fatalf("bucket %q recency value = %#v", key, b.rateElement.Value)
		}
	}
}

func BenchmarkRateLimiterCapacityChurn(b *testing.B) {
	limiter := NewRateLimiter(60, 1)
	limiter.maxBuckets = defaultLimiterMaxBuckets
	limiter.ttl = 0
	now := time.Unix(100, 0)
	for i := 0; i < limiter.maxBuckets; i++ {
		limiter.allowAt(fmt.Sprintf("initial-%d", i), now.Add(time.Duration(i)))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.allowAt(fmt.Sprintf("attacker-%d", i), now.Add(time.Hour+time.Duration(i)))
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
