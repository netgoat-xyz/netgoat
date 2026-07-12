package traffic

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

type BandwidthLimiter struct {
	mu         sync.Mutex
	rate       float64
	burst      float64
	buckets    map[string]*bucket
	maxBuckets int
	ttl        time.Duration
	lastPrune  time.Time
}

func NewBandwidthLimiter(bytesPerSecond, burstBytes int) *BandwidthLimiter {
	if bytesPerSecond <= 0 {
		bytesPerSecond = 1 << 20
	}
	if burstBytes <= 0 {
		burstBytes = bytesPerSecond
	}
	return &BandwidthLimiter{
		rate:       float64(bytesPerSecond),
		burst:      float64(burstBytes),
		buckets:    make(map[string]*bucket),
		maxBuckets: defaultLimiterMaxBuckets,
		ttl:        defaultLimiterBucketTTL,
	}
}

func (l *BandwidthLimiter) Wait(ctx context.Context, key string, bytes int) error {
	if bytes <= 0 {
		return nil
	}
	if key == "" {
		key = "global"
	}

	remaining := bytes
	for remaining > 0 {
		chunk := remaining
		if chunk > int(l.burst) {
			chunk = int(l.burst)
		}
		if err := l.waitChunk(ctx, key, chunk); err != nil {
			return err
		}
		remaining -= chunk
	}
	return nil
}

func (l *BandwidthLimiter) waitChunk(ctx context.Context, key string, bytes int) error {
	for {
		wait := l.reserve(key, bytes, time.Now())
		if wait <= 0 {
			return nil
		}

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (l *BandwidthLimiter) reserve(key string, bytes int, now time.Time) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pruneLocked(now)

	b, ok := l.buckets[key]
	if !ok {
		l.ensureCapacityLocked(now)
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}

	elapsed := now.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * l.rate
		if b.tokens > l.burst {
			b.tokens = l.burst
		}
		b.last = now
	}

	need := float64(bytes)
	if b.tokens >= need {
		b.tokens -= need
		return 0
	}
	return time.Duration((need-b.tokens)/l.rate*float64(time.Second)) + time.Millisecond
}

func (l *BandwidthLimiter) pruneLocked(now time.Time) {
	if l.ttl <= 0 || now.Sub(l.lastPrune) < time.Minute {
		return
	}
	for key, b := range l.buckets {
		if now.Sub(b.last) > l.ttl {
			delete(l.buckets, key)
		}
	}
	l.lastPrune = now
}

func (l *BandwidthLimiter) ensureCapacityLocked(now time.Time) {
	if l.maxBuckets <= 0 || len(l.buckets) < l.maxBuckets {
		return
	}
	var oldestKey string
	var oldestTime time.Time
	for key, b := range l.buckets {
		if oldestKey == "" || b.last.Before(oldestTime) {
			oldestKey = key
			oldestTime = b.last
		}
	}
	if oldestKey != "" {
		delete(l.buckets, oldestKey)
	}
	l.lastPrune = now
}

type ThrottledReadCloser struct {
	io.ReadCloser
	limiter *BandwidthLimiter
	key     string
	ctx     context.Context
}

func WrapReadCloser(r io.ReadCloser, limiter *BandwidthLimiter, key string, ctx context.Context) io.ReadCloser {
	if r == nil || limiter == nil {
		return r
	}
	return &ThrottledReadCloser{ReadCloser: r, limiter: limiter, key: key, ctx: ctx}
}

func (r *ThrottledReadCloser) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if n > 0 {
		if waitErr := r.limiter.Wait(r.ctx, r.key, n); waitErr != nil {
			return n, waitErr
		}
	}
	return n, err
}

type BandwidthResponseWriter struct {
	http.ResponseWriter
	limiter *BandwidthLimiter
	key     string
	ctx     context.Context
}

func WrapResponseWriter(w http.ResponseWriter, limiter *BandwidthLimiter, key string, ctx context.Context) http.ResponseWriter {
	if limiter == nil {
		return w
	}
	return &BandwidthResponseWriter{ResponseWriter: w, limiter: limiter, key: key, ctx: ctx}
}

func (w *BandwidthResponseWriter) Write(b []byte) (int, error) {
	if len(b) > 0 {
		if err := w.limiter.Wait(w.ctx, w.key, len(b)); err != nil {
			return 0, err
		}
	}
	return w.ResponseWriter.Write(b)
}

func (w *BandwidthResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *BandwidthResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hijacker.Hijack()
}

func (w *BandwidthResponseWriter) Push(target string, opts *http.PushOptions) error {
	pusher, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, opts)
}

func (w *BandwidthResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
