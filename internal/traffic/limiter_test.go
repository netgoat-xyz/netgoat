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
