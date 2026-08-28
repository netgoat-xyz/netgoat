package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRequiredConfigMissingFileIsFatal(t *testing.T) {
	_, err := loadRequiredConfig(filepath.Join(t.TempDir(), "config.yml"))
	if err == nil || !strings.Contains(err.Error(), "failed to read") {
		t.Fatalf("missing config error = %v", err)
	}
}

func TestLoadRequiredConfigInvalidYAMLIsFatal(t *testing.T) {
	path := writeStartupConfig(t, "debug_logs: [not: valid\n")
	_, err := loadRequiredConfig(path)
	if err == nil || !strings.Contains(err.Error(), "failed to read") {
		t.Fatalf("invalid YAML error = %v", err)
	}
}

func TestLoadRequiredConfigRejectsPublicHTTPWithoutTLS(t *testing.T) {
	for _, listen := range []string{":8080", "0.0.0.0:8080", "[::]:8080", "192.0.2.10:8080"} {
		path := writeStartupConfig(t, "listen: \""+listen+"\"\nssl:\n  enabled: false\n")
		_, err := loadRequiredConfig(path)
		if err == nil || !strings.Contains(err.Error(), "allow_insecure_public_http") {
			t.Fatalf("listen %q error = %v, want public HTTP fatal", listen, err)
		}
	}
}

func TestLoadRequiredConfigAllowsLoopbackHTTPWithoutTLS(t *testing.T) {
	for _, listen := range []string{"127.0.0.1:8080", "[::1]:8080", "localhost:8080"} {
		path := writeStartupConfig(t, "listen: \""+listen+"\"\nauth:\n  enabled: false\nssl:\n  enabled: false\n")
		cfg, err := loadRequiredConfig(path)
		if err != nil {
			t.Fatalf("listen %q: %v", listen, err)
		}
		if cfg.Auth.Enabled || cfg.SSL.Enabled {
			t.Fatalf("loopback HTTP must not force auth or TLS on: auth=%v ssl=%v", cfg.Auth.Enabled, cfg.SSL.Enabled)
		}
	}
}

func TestLoadRequiredConfigAllowsPublicHTTPWithInsecureFlag(t *testing.T) {
	path := writeStartupConfig(t, "listen: \":8080\"\nallow_insecure_public_http: true\nssl:\n  enabled: false\n")
	cfg, err := loadRequiredConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.AllowInsecurePublicHTTP {
		t.Fatal("insecure flag should remain set")
	}
}

func TestLoadRequiredConfigAllowsPublicHTTPS(t *testing.T) {
	path := writeStartupConfig(t, "ssl:\n  enabled: true\n  port: \":8443\"\n")
	if _, err := loadRequiredConfig(path); err != nil {
		t.Fatal(err)
	}
}

func TestLoadRequiredConfigEmptyFileIsFatal(t *testing.T) {
	path := writeStartupConfig(t, "")
	_, err := loadRequiredConfig(path)
	if err == nil || !strings.Contains(err.Error(), "public address") {
		t.Fatalf("empty config error = %v, want public HTTP fatal", err)
	}
}

func writeStartupConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestSampleConfigIsSafeToStart(t *testing.T) {
	cfg, err := loadRequiredConfig("config.yml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPListenAddr() != "127.0.0.1:8080" {
		t.Fatalf("sample listen = %q, want 127.0.0.1:8080", cfg.HTTPListenAddr())
	}
	if cfg.AllowInsecurePublicHTTP || cfg.SSL.Enabled {
		t.Fatalf("sample should use loopback plaintext HTTP: %+v", cfg)
	}
	if cfg.Auth.Enabled {
		t.Fatal("sample must not enable auth on loopback")
	}
}
