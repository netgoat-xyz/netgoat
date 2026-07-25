package main

import (
	"sync/atomic"
	"time"

	"netgoat.xyz/agent/internal/config"
	"netgoat.xyz/agent/internal/policy"
	"netgoat.xyz/agent/internal/traffic"
)

// trafficRuntimeState is immutable once published. Replacing it lets stream
// updates retune bounded request controls without data races in the hot path.
type trafficRuntimeState struct {
	cache        policy.CacheSettings
	bandwidth    policy.BandwidthSettings
	rateLimiter  *traffic.RateLimiter
	rateLimitKey string
	requestQueue *traffic.Queue
	metricsOn    bool
	metricsPath  string
}

type trafficRuntime struct {
	state atomic.Pointer[trafficRuntimeState]
}

func newTrafficRuntime(cfg *config.Config) *trafficRuntime {
	runtime := &trafficRuntime{}
	runtime.Update(cfg)
	return runtime
}

func (r *trafficRuntime) Load() *trafficRuntimeState {
	if r == nil {
		return &trafficRuntimeState{
			cache:       policy.DefaultCacheSettings(),
			bandwidth:   policy.DefaultBandwidthSettings(),
			metricsPath: "/__netgoat/metrics",
		}
	}
	if state := r.state.Load(); state != nil {
		return state
	}
	return &trafficRuntimeState{
		cache:       policy.DefaultCacheSettings(),
		bandwidth:   policy.DefaultBandwidthSettings(),
		metricsPath: "/__netgoat/metrics",
	}
}

// Update atomically publishes components constructed from one config snapshot.
// The values are normalized here so malformed or partially specified control
// plane settings cannot create an unbounded runtime object.
func (r *trafficRuntime) Update(cfg *config.Config) {
	if r == nil {
		return
	}
	state := &trafficRuntimeState{
		cache:       policy.DefaultCacheSettings(),
		bandwidth:   policy.DefaultBandwidthSettings(),
		metricsPath: "/__netgoat/metrics",
	}
	if cfg == nil {
		r.state.Store(state)
		return
	}

	cacheSettings, err := policy.ResolveCache(policy.CacheSettings{
		Enabled:      cfg.Cache.Enabled,
		TTLSeconds:   cfg.Cache.TTLSeconds,
		MaxEntries:   cfg.Cache.MaxEntries,
		MaxBodyBytes: cfg.Cache.MaxBodyBytes,
	}, nil)
	if err == nil {
		state.cache = cacheSettings
	}

	bandwidthSettings, err := policy.ResolveBandwidth(policy.BandwidthSettings{
		Enabled:        cfg.Bandwidth.Enabled,
		BytesPerSecond: cfg.Bandwidth.BytesPerSecond,
		BurstBytes:     cfg.Bandwidth.BurstBytes,
		Key:            policy.KeyMode(cfg.Bandwidth.Key),
	}, nil)
	if err == nil {
		state.bandwidth = bandwidthSettings
	}

	if cfg.RateLimit.Enabled {
		state.rateLimiter = traffic.NewRateLimiter(cfg.RateLimit.RequestsPerMinute, cfg.RateLimit.Burst)
		state.rateLimitKey = cfg.RateLimit.Key
	}
	if cfg.RequestQueue.Enabled {
		timeout := time.Duration(ifZeroInt(cfg.RequestQueue.TimeoutSeconds, 5)) * time.Second
		state.requestQueue = traffic.NewQueue(cfg.RequestQueue.MaxConcurrent, cfg.RequestQueue.MaxQueued, timeout)
	}
	state.metricsOn = cfg.Metrics.Enabled
	if path, valid := metricsEndpointPath(cfg.Metrics.Path); valid {
		state.metricsPath = path
	}

	r.state.Store(state)
}
