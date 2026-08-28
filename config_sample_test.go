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
	if cfg.Routes == nil {
		t.Fatal("sample must explicitly configure an empty route set")
	}
	if len(cfg.Routes) != 0 {
		t.Fatalf("sample exposes upstream routes by default: %+v", cfg.Routes)
	}
	snapshot := localConfigSnapshot(cfg)
	if !snapshot.RoutesConfigured || len(snapshot.Routes) != 0 {
		t.Fatalf("sample must authoritatively clear persisted routes: %+v", snapshot)
	}
	if cfg.Cloudflare.Access.Enabled || cfg.Cloudflare.Reconciliation.Enabled {
		t.Fatalf("sample enables Cloudflare integrations unexpectedly: %+v", cfg.Cloudflare)
	}
	if len(cfg.Plugins.Installations) != 0 {
		t.Fatalf("sample should not select developer plugins: %+v", cfg.Plugins)
	}
	if cfg.Cloudflare.Reconciliation.DryRun == nil || !*cfg.Cloudflare.Reconciliation.DryRun {
		t.Fatalf("sample Cloudflare reconciliation must remain dry-run by default: %+v", cfg.Cloudflare.Reconciliation)
	}
	if cfg.AllowInsecurePublicHTTP {
		t.Fatal("sample must not enable the public plaintext HTTP escape hatch")
	}
	if cfg.SSL.Enabled {
		t.Fatal("sample should keep TLS off when binding loopback HTTP")
	}
	if got := cfg.HTTPListenAddr(); got != "127.0.0.1:8080" {
		t.Fatalf("sample listen = %q, want loopback 127.0.0.1:8080", got)
	}
	if err := cfg.ValidateListenSafety(); err != nil {
		t.Fatalf("sample must be safe to start: %v", err)
	}
}
