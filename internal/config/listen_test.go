package config

import (
	"strings"
	"testing"
)

func TestHTTPListenAddrDefaultsToPublicBind(t *testing.T) {
	if got := (*Config)(nil).HTTPListenAddr(); got != ":8080" {
		t.Fatalf("nil config HTTPListenAddr = %q, want :8080", got)
	}
	if got := (&Config{}).HTTPListenAddr(); got != ":8080" {
		t.Fatalf("empty config HTTPListenAddr = %q, want :8080", got)
	}
	cfg := &Config{Listen: " 127.0.0.1:8080 "}
	if got := cfg.HTTPListenAddr(); got != "127.0.0.1:8080" {
		t.Fatalf("configured HTTPListenAddr = %q", got)
	}
}

func TestValidateListenSafety(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		listen     string
		ssl        bool
		auth       bool
		insecure   bool
		wantErr    string
		wantNilCfg bool
	}{
		{
			name:    "public default without TLS is fatal",
			wantErr: "public address",
		},
		{
			name:    "empty host is public",
			listen:  ":8080",
			wantErr: "public address",
		},
		{
			name:    "0.0.0.0 is public",
			listen:  "0.0.0.0:8080",
			wantErr: "public address",
		},
		{
			name:    "unspecified IPv6 is public",
			listen:  "[::]:8080",
			wantErr: "public address",
		},
		{
			name:    "LAN address is public",
			listen:  "192.168.1.10:8080",
			wantErr: "public address",
		},
		{
			name:    "auth does not allow public HTTP",
			listen:  ":8080",
			auth:    true,
			wantErr: "public address",
		},
		{
			name:   "loopback IPv4 without TLS is allowed",
			listen: "127.0.0.1:8080",
		},
		{
			name:   "loopback IPv6 without TLS is allowed",
			listen: "[::1]:8080",
		},
		{
			name:   "localhost without TLS is allowed",
			listen: "localhost:8080",
		},
		{
			name:     "explicit insecure flag allows public HTTP",
			listen:   ":8080",
			insecure: true,
		},
		{
			name:   "TLS allows a public bind",
			listen: ":8080",
			ssl:    true,
		},
		{
			name:    "invalid address is fatal",
			listen:  "127.0.0.1",
			wantErr: "invalid listen address",
		},
		{
			name:       "nil config is fatal",
			wantNilCfg: true,
			wantErr:    "configuration is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var cfg *Config
			if !tc.wantNilCfg {
				cfg = &Config{Listen: tc.listen, AllowInsecurePublicHTTP: tc.insecure}
				cfg.SSL.Enabled = tc.ssl
				cfg.Auth.Enabled = tc.auth
			}
			err := cfg.ValidateListenSafety()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateListenSafety() = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("ValidateListenSafety() = %v, want error containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestListenAddrIsLoopback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		addr     string
		loopback bool
		wantErr  bool
	}{
		{name: "all interfaces", addr: ":8080"},
		{name: "unspecified IPv4", addr: "0.0.0.0:80"},
		{name: "unspecified IPv6", addr: "[::]:80"},
		{name: "loopback IPv4", addr: "127.0.0.1:8080", loopback: true},
		{name: "loopback IPv4 subnet", addr: "127.0.0.2:8080", loopback: true},
		{name: "loopback IPv6", addr: "[::1]:8080", loopback: true},
		{name: "localhost", addr: "localhost:8080", loopback: true},
		{name: "localhost case", addr: "LOCALHOST:8080", loopback: true},
		{name: "hostname", addr: "example.test:8080"},
		{name: "missing port", addr: "127.0.0.1", wantErr: true},
		{name: "empty", addr: "", wantErr: true},
		{name: "empty port", addr: "127.0.0.1:", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := listenAddrIsLoopback(tc.addr)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("listenAddrIsLoopback(%q) = %v", tc.addr, err)
			}
			if got != tc.loopback {
				t.Fatalf("listenAddrIsLoopback(%q) = %v, want %v", tc.addr, got, tc.loopback)
			}
		})
	}
}
