package main

import (
	"testing"

	"netgoat.xyz/agent/internal/config"
)

func TestSampleConfigUsesSafeOfflineDefaults(t *testing.T) {
	cfg, err := config.Load("config.yml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DebugLogs || cfg.DebugOverlay || cfg.Auth.Enabled || cfg.Telemetry.Enabled {
		t.Fatalf("sample enables debug/auth/telemetry unexpectedly: %+v", cfg)
	}
	if cfg.API.URL != "" || cfg.API.Key != "" {
		t.Fatalf("sample should start offline without placeholder credentials: %+v", cfg.API)
	}
	if len(cfg.TrustedProxies) != 0 {
		t.Fatalf("sample trusts forwarding peers by default: %v", cfg.TrustedProxies)
	}
	if cfg.Cloudflare.Access.Enabled || cfg.Cloudflare.Reconciliation.Enabled {
		t.Fatalf("sample enables Cloudflare integrations unexpectedly: %+v", cfg.Cloudflare)
	}
	if cfg.Cloudflare.Reconciliation.DryRun == nil || !*cfg.Cloudflare.Reconciliation.DryRun {
		t.Fatalf("sample Cloudflare reconciliation must remain dry-run by default: %+v", cfg.Cloudflare.Reconciliation)
	}
}
