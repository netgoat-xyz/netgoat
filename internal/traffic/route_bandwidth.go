package traffic

import (
	"fmt"
	"sync"
	"time"

	"netgoat.xyz/agent/internal/policy"
)

// BandwidthLimiters owns independent token-bucket sets for route policies.
// Replacing a setting drops only the affected route's buckets.
type BandwidthLimiters struct {
	mu       sync.Mutex
	limiters map[string]bandwidthLimiter
}

type bandwidthLimiter struct {
	signature string
	lastUsed  time.Time
	limiter   *BandwidthLimiter
}

func NewBandwidthLimiters() *BandwidthLimiters {
	return &BandwidthLimiters{limiters: make(map[string]bandwidthLimiter)}
}

func (b *BandwidthLimiters) Limiter(routeKey string, settings policy.BandwidthSettings) *BandwidthLimiter {
	if b == nil || !settings.Enabled {
		return nil
	}
	if routeKey == "" {
		routeKey = "unknown"
	}
	signature := fmt.Sprintf("%d:%d", settings.BytesPerSecond, settings.BurstBytes)
	now := time.Now()
	b.mu.Lock()
	defer b.mu.Unlock()
	if existing, ok := b.limiters[routeKey]; ok && existing.signature == signature {
		existing.lastUsed = now
		b.limiters[routeKey] = existing
		return existing.limiter
	}
	limiter := NewBandwidthLimiter(settings.BytesPerSecond, settings.BurstBytes)
	b.limiters[routeKey] = bandwidthLimiter{signature: signature, lastUsed: now, limiter: limiter}
	return limiter
}

func (b *BandwidthLimiters) Retain(routeKeys map[string]struct{}) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for key := range b.limiters {
		if _, keep := routeKeys[key]; !keep {
			delete(b.limiters, key)
		}
	}
}
