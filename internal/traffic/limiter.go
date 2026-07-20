package traffic

import (
	"container/list"
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
	mu         sync.Mutex
	rate       float64
	burst      float64
	buckets    map[string]*rateBucket
	recency    list.List
	maxBuckets int
	ttl        time.Duration
	lastPrune  time.Time
}

type rateBucket struct {
	tokens      float64
	last        time.Time
	rateElement *list.Element
}

type bucket struct {
	tokens float64
	last   time.Time
}

const (
	defaultLimiterMaxBuckets = 10000
	defaultLimiterBucketTTL  = 10 * time.Minute
)

func NewRateLimiter(requestsPerMinute, burst int) *RateLimiter {
	if requestsPerMinute <= 0 {
		requestsPerMinute = 60
	}
	if burst <= 0 {
		burst = requestsPerMinute
	}
	return &RateLimiter{
		rate:       float64(requestsPerMinute) / 60,
		burst:      float64(burst),
		buckets:    make(map[string]*rateBucket),
		maxBuckets: defaultLimiterMaxBuckets,
		ttl:        defaultLimiterBucketTTL,
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
	l.pruneLocked(now)

	b, ok := l.buckets[key]
	if !ok {
		l.ensureCapacityLocked(now)
		b = &rateBucket{tokens: l.burst - 1, last: now}
		b.rateElement = l.recency.PushFront(key)
		l.buckets[key] = b
		return true
	}
	l.recency.MoveToFront(b.rateElement)

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

func (l *RateLimiter) pruneLocked(now time.Time) {
	if l.ttl <= 0 || now.Sub(l.lastPrune) < time.Minute {
		return
	}
	for key, b := range l.buckets {
		if now.Sub(b.last) > l.ttl {
			l.removeBucketLocked(key, b)
		}
	}
	l.lastPrune = now
}

func (l *RateLimiter) ensureCapacityLocked(now time.Time) {
	if l.maxBuckets <= 0 || len(l.buckets) < l.maxBuckets {
		return
	}
	oldest := l.recency.Back()
	if oldest != nil {
		key := oldest.Value.(string)
		l.removeBucketLocked(key, l.buckets[key])
	}
	l.lastPrune = now
}

func (l *RateLimiter) removeBucketLocked(key string, b *rateBucket) {
	delete(l.buckets, key)
	if b != nil && b.rateElement != nil {
		l.recency.Remove(b.rateElement)
		b.rateElement = nil
	}
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
