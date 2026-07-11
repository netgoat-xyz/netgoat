package traffic

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrRateLimited = errors.New("rate limit exceeded")
	ErrQueueFull   = errors.New("request queue full")
	ErrQueueWait   = errors.New("request queue wait timed out")
)

type RateLimiter struct {
	mu      sync.Mutex
	rate    float64
	burst   float64
	buckets map[string]*bucket
}

type bucket struct {
	tokens float64
	last   time.Time
}

func NewRateLimiter(requestsPerMinute, burst int) *RateLimiter {
	if requestsPerMinute <= 0 {
		requestsPerMinute = 60
	}
	if burst <= 0 {
		burst = requestsPerMinute
	}
	return &RateLimiter{
		rate:    float64(requestsPerMinute) / 60,
		burst:   float64(burst),
		buckets: make(map[string]*bucket),
	}
}

func (l *RateLimiter) Allow(key string) bool {
	return l.allowAt(key, time.Now())
}

func (l *RateLimiter) allowAt(key string, now time.Time) bool {
	if key == "" {
		key = "global"
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok {
		l.buckets[key] = &bucket{tokens: l.burst - 1, last: now}
		return true
	}

	elapsed := now.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * l.rate
		if b.tokens > l.burst {
			b.tokens = l.burst
		}
		b.last = now
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

type Queue struct {
	sem       chan struct{}
	maxQueued int
	timeout   time.Duration

	mu      sync.Mutex
	waiting int
}

func NewQueue(maxConcurrent, maxQueued int, timeout time.Duration) *Queue {
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	}
	if maxQueued < 0 {
		maxQueued = 0
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Queue{
		sem:       make(chan struct{}, maxConcurrent),
		maxQueued: maxQueued,
		timeout:   timeout,
	}
}

func (q *Queue) Acquire(ctx context.Context) (func(), error) {
	select {
	case q.sem <- struct{}{}:
		return q.release, nil
	default:
	}

	q.mu.Lock()
	if q.waiting >= q.maxQueued {
		q.mu.Unlock()
		return nil, ErrQueueFull
	}
	q.waiting++
	q.mu.Unlock()

	defer func() {
		q.mu.Lock()
		q.waiting--
		q.mu.Unlock()
	}()

	timer := time.NewTimer(q.timeout)
	defer timer.Stop()

	select {
	case q.sem <- struct{}{}:
		return q.release, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return nil, ErrQueueWait
	}
}

func (q *Queue) release() {
	select {
	case <-q.sem:
	default:
	}
}
