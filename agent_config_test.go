package main

import (
	"testing"

	"netgoat.xyz/agent/internal/config"
	"netgoat.xyz/agent/internal/streaming"
)

func TestApplyAgentConfigToConfig(t *testing.T) {
	cfg := &config.Config{}
	applyAgentConfigToConfig(cfg, streaming.AgentConfigData{
		Cache: streaming.AgentCacheConfig{
			Enabled:      true,
			TTLSeconds:   90,
			MaxEntries:   2048,
			MaxBodyBytes: 2 << 20,
		},
		RateLimit: streaming.AgentRateLimitConfig{
			Enabled:           true,
			RequestsPerMinute: 120,
			Burst:             30,
			Key:               streaming.AgentKeyHost,
		},
		RequestQueue: streaming.AgentRequestQueueConfig{
			Enabled:        true,
			MaxConcurrent:  20,
			MaxQueued:      200,
			TimeoutSeconds: 7,
		},
		Bandwidth: streaming.AgentBandwidthConfig{
			Enabled:        true,
			BytesPerSecond: 4096,
			BurstBytes:     8192,
			Key:            streaming.AgentKeyGlobal,
		},
		Metrics: streaming.AgentMetricsConfig{
			Enabled: true,
			Path:    "/metrics",
		},
		KodaWaf: streaming.AgentModelConfig{
			Enabled:       true,
			Threshold:     0.8,
			ModelPath:     "models/waf.pkl",
			ScalerPath:    "models/waf-features.pkl",
			PythonScript:  "models/waf.py",
			FeatureHeader: "X-WAF",
		},
		Koda2: streaming.AgentModelConfig{
			Enabled:       true,
			Threshold:     0.65,
			ModelPath:     "models/koda2.keras",
			ScalerPath:    "models/koda2.pkl",
			PythonScript:  "models/koda2.py",
			FeatureHeader: "X-Koda2",
		},
	})

	if !cfg.Cache.Enabled || cfg.Cache.TTLSeconds != 90 || cfg.Cache.MaxEntries != 2048 {
		t.Fatalf("cache config was not applied: %+v", cfg.Cache)
	}
	if !cfg.RateLimit.Enabled || cfg.RateLimit.Key != "host" || cfg.RateLimit.Burst != 30 {
		t.Fatalf("rate limit config was not applied: %+v", cfg.RateLimit)
	}
	if !cfg.RequestQueue.Enabled || cfg.RequestQueue.MaxConcurrent != 20 || cfg.RequestQueue.TimeoutSeconds != 7 {
		t.Fatalf("request queue config was not applied: %+v", cfg.RequestQueue)
	}
	if !cfg.Bandwidth.Enabled || cfg.Bandwidth.Key != "global" || cfg.Bandwidth.BurstBytes != 8192 {
		t.Fatalf("bandwidth config was not applied: %+v", cfg.Bandwidth)
	}
	if !cfg.Metrics.Enabled || cfg.Metrics.Path != "/metrics" {
		t.Fatalf("metrics config was not applied: %+v", cfg.Metrics)
	}
	if !cfg.KodaWaf.Enabled || cfg.KodaWaf.FeatureHeader != "X-WAF" || cfg.KodaWaf.Threshold != 0.8 {
		t.Fatalf("koda-waf config was not applied: %+v", cfg.KodaWaf)
	}
	if !cfg.Koda2.Enabled || cfg.Koda2.FeatureHeader != "X-Koda2" || cfg.Koda2.Threshold != 0.65 {
		t.Fatalf("koda-2 config was not applied: %+v", cfg.Koda2)
	}
}

func TestApplyAgentConfigToConfigSkipsEmptySnapshot(t *testing.T) {
	cfg := &config.Config{}
	cfg.Cache.Enabled = true
	cfg.Cache.TTLSeconds = 30

	applyAgentConfigToConfig(cfg, streaming.AgentConfigData{})

	if !cfg.Cache.Enabled || cfg.Cache.TTLSeconds != 30 {
		t.Fatalf("empty agent config should not overwrite local config: %+v", cfg.Cache)
	}
}
