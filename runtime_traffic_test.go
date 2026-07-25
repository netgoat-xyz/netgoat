package main

import (
	"testing"

	"netgoat.xyz/agent/internal/config"
)

func TestTrafficRuntimeReplacesLiveControls(t *testing.T) {
	cfg := &config.Config{}
	cfg.Cache.Enabled = true
	cfg.Cache.TTLSeconds = 30
	cfg.Cache.MaxEntries = 4
	cfg.Cache.MaxBodyBytes = 2048
	cfg.RateLimit.Enabled = true
	cfg.RateLimit.RequestsPerMinute = 10
	cfg.RateLimit.Burst = 2
	cfg.Bandwidth.Enabled = true
	cfg.Bandwidth.BytesPerSecond = 4096
	cfg.Bandwidth.BurstBytes = 8192
	cfg.Metrics.Enabled = true
	cfg.Metrics.Path = "/metrics"

	runtime := newTrafficRuntime(cfg)
	first := runtime.Load()
	if first.cache.TTLSeconds != 30 || first.rateLimiter == nil || first.metricsPath != "/metrics" {
		t.Fatalf("initial runtime = %+v", first)
	}

	cfg.Cache.TTLSeconds = 90
	cfg.RateLimit.Enabled = false
	cfg.Bandwidth.Enabled = false
	cfg.Metrics.Enabled = false
	runtime.Update(cfg)
	second := runtime.Load()
	if second == first || second.cache.TTLSeconds != 90 || second.rateLimiter != nil || second.bandwidth.Enabled || second.metricsOn {
		t.Fatalf("updated runtime = %+v", second)
	}
}
