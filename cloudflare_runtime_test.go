package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"netgoat.xyz/agent/internal/cloudflare"
	"netgoat.xyz/agent/internal/config"
)

func TestConfigureCloudflareAccessMapsConfigAndFailsClosed(t *testing.T) {
	cfg := &config.Config{}
	cfg.Cloudflare.Access.Enabled = true
	cfg.Cloudflare.Access.Issuer = "https://team.cloudflareaccess.com"
	cfg.Cloudflare.Access.Audience = []string{"audience-tag"}
	cfg.Cloudflare.Access.JWKSCacheSeconds = 90
	cfg.Cloudflare.Access.ClockSkewSeconds = 30
	cfg.Cloudflare.Access.FetchTimeoutSeconds = 5

	settings, err := cloudflareAccessSettingsFromConfig(cfg)
	if err != nil {
		t.Fatalf("cloudflareAccessSettingsFromConfig(): %v", err)
	}
	if settings.CacheTTL != 90*time.Second || settings.ClockSkew != 30*time.Second || settings.FetchTimeout != 5*time.Second {
		t.Fatalf("unexpected Cloudflare Access durations: %+v", settings)
	}
	validator, err := configureCloudflareAccess(cfg)
	if err != nil {
		t.Fatalf("configureCloudflareAccess(): %v", err)
	}
	if validator == nil {
		t.Fatal("configureCloudflareAccess() returned nil validator")
	}

	handler := validator.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("request with no Access assertion reached wrapped handler")
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "https://agent.example.test/", nil))
	if response.Code != http.StatusForbidden {
		t.Fatalf("missing Access assertion status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestCloudflareAccessDisabledDoesNothing(t *testing.T) {
	validator, err := configureCloudflareAccess(&config.Config{})
	if err != nil || validator != nil {
		t.Fatalf("disabled Access = %v, %v", validator, err)
	}
}

func TestCloudflareReconciliationDisabledDoesNothing(t *testing.T) {
	previousFactory := newCloudflareAPIClient
	defer func() { newCloudflareAPIClient = previousFactory }()
	newCloudflareAPIClient = func(cloudflare.APIConfig) (cloudflareReconciliationClient, error) {
		t.Fatal("disabled reconciliation constructed an API client")
		return nil, nil
	}
	results, err := reconcileCloudflare(context.Background(), &config.Config{})
	if err != nil || len(results) != 0 {
		t.Fatalf("disabled reconciliation = %#v, %v", results, err)
	}
}

func TestReconcileCloudflareMapsBoundedDryRunPlan(t *testing.T) {
	cfg := &config.Config{}
	cfg.Cloudflare.Reconciliation.Enabled = true
	cfg.Cloudflare.Reconciliation.AccountID = "0123456789abcdef0123456789abcdef"
	cfg.Cloudflare.Reconciliation.DNSRecords = []config.CloudflareDNSRecord{
		{
			ZoneID: "abcdef0123456789abcdef0123456789",
			Record: map[string]any{
				"type":    "A",
				"name":    "app.example.test",
				"content": "203.0.113.10",
			},
		},
		{
			ZoneID:   "abcdef0123456789abcdef0123456789",
			RecordID: "fedcba9876543210fedcba9876543210",
			Delete:   true,
		},
	}
	cfg.Cloudflare.Reconciliation.Tunnels = []config.CloudflareTunnel{
		{
			Tunnel: map[string]any{
				"name":       "new-netgoat",
				"config_src": "cloudflare",
			},
		},
		{
			TunnelID: "01234567-89ab-cdef-0123-456789abcdef",
			Tunnel: map[string]any{
				"name":       "netgoat",
				"config_src": "cloudflare",
			},
		},
	}
	t.Setenv(cloudflareAPITokenEnvironment, "test-api-token")

	fake := &recordingCloudflareClient{}
	previousFactory := newCloudflareAPIClient
	defer func() { newCloudflareAPIClient = previousFactory }()
	var received cloudflare.APIConfig
	newCloudflareAPIClient = func(settings cloudflare.APIConfig) (cloudflareReconciliationClient, error) {
		received = settings
		return fake, nil
	}

	results, err := reconcileCloudflare(context.Background(), cfg)
	if err != nil {
		t.Fatalf("reconcileCloudflare(): %v", err)
	}
	if !received.Enabled || !received.DryRun || received.APIToken != "test-api-token" || received.AccountID != cfg.Cloudflare.Reconciliation.AccountID {
		t.Fatalf("unexpected Cloudflare API settings: %+v", received)
	}
	if len(results) != 4 || len(fake.calls) != 4 {
		t.Fatalf("reconciliation calls/results = %d/%d, want 4/4", len(fake.calls), len(results))
	}
	if fake.calls[0].kind != "dns-upsert" || fake.calls[0].desired["name"] != "app.example.test" {
		t.Fatalf("DNS creation was not mapped correctly: %+v", fake.calls[0])
	}
	if fake.calls[1].kind != "dns-delete" || fake.calls[1].id != "fedcba9876543210fedcba9876543210" {
		t.Fatalf("DNS deletion was not mapped correctly: %+v", fake.calls[1])
	}
	if fake.calls[2].kind != "tunnel-create" || fake.calls[2].desired["name"] != "new-netgoat" {
		t.Fatalf("tunnel creation was not mapped correctly: %+v", fake.calls[2])
	}
	if fake.calls[3].kind != "tunnel-update" || fake.calls[3].desired["config_src"] != "cloudflare" {
		t.Fatalf("tunnel update was not mapped correctly: %+v", fake.calls[3])
	}
}

func TestCloudflareReconciliationRequiresEnvironmentCredential(t *testing.T) {
	cfg := &config.Config{}
	cfg.Cloudflare.Reconciliation.Enabled = true
	cfg.Cloudflare.Reconciliation.DNSRecords = []config.CloudflareDNSRecord{{
		ZoneID: "abcdef0123456789abcdef0123456789",
		Record: map[string]any{"type": "A"},
	}}
	t.Setenv(cloudflareAPITokenEnvironment, "")
	if _, err := cloudflareReconciliationPlanFromConfig(cfg); err == nil || !strings.Contains(err.Error(), cloudflareAPITokenEnvironment) {
		t.Fatalf("missing token error = %v", err)
	}
}

func TestCloudflareReconciliationRequiresTunnelIDForDeletion(t *testing.T) {
	cfg := &config.Config{}
	cfg.Cloudflare.Reconciliation.Enabled = true
	cfg.Cloudflare.Reconciliation.AccountID = "0123456789abcdef0123456789abcdef"
	cfg.Cloudflare.Reconciliation.Tunnels = []config.CloudflareTunnel{{Delete: true}}
	t.Setenv(cloudflareAPITokenEnvironment, "test-api-token")

	if _, err := cloudflareReconciliationPlanFromConfig(cfg); err == nil || !strings.Contains(err.Error(), "deletion requires tunnel_id") {
		t.Fatalf("missing tunnel ID deletion error = %v", err)
	}
}

func TestCloudflareReconciliationPreflightsIdentifiers(t *testing.T) {
	cfg := &config.Config{}
	cfg.Cloudflare.Reconciliation.Enabled = true
	cfg.Cloudflare.Reconciliation.DNSRecords = []config.CloudflareDNSRecord{{
		ZoneID: "not-a-zone-id",
		Record: map[string]any{"type": "A"},
	}}
	t.Setenv(cloudflareAPITokenEnvironment, "test-api-token")
	if _, err := cloudflareReconciliationPlanFromConfig(cfg); err == nil || !strings.Contains(err.Error(), "invalid Cloudflare DNS record") {
		t.Fatalf("invalid ID error = %v", err)
	}
}

type recordingCloudflareCall struct {
	kind    string
	zoneID  string
	id      string
	desired map[string]any
}

type recordingCloudflareClient struct {
	calls []recordingCloudflareCall
}

func (client *recordingCloudflareClient) ReconcileDNSRecord(_ context.Context, zoneID, recordID string, desired any) (cloudflare.APIResult, error) {
	object, _ := desired.(map[string]any)
	client.calls = append(client.calls, recordingCloudflareCall{kind: "dns-upsert", zoneID: zoneID, id: recordID, desired: object})
	return cloudflare.APIResult{DryRun: true, Method: http.MethodPost, Path: "/dns"}, nil
}

func (client *recordingCloudflareClient) DeleteDNSRecord(_ context.Context, zoneID, recordID string) (cloudflare.APIResult, error) {
	client.calls = append(client.calls, recordingCloudflareCall{kind: "dns-delete", zoneID: zoneID, id: recordID})
	return cloudflare.APIResult{DryRun: true, Method: http.MethodDelete, Path: "/dns"}, nil
}

func (client *recordingCloudflareClient) CreateTunnel(_ context.Context, desired any) (cloudflare.APIResult, error) {
	object, _ := desired.(map[string]any)
	client.calls = append(client.calls, recordingCloudflareCall{kind: "tunnel-create", desired: object})
	return cloudflare.APIResult{DryRun: true, Method: http.MethodPost, Path: "/tunnel"}, nil
}

func (client *recordingCloudflareClient) ReconcileTunnel(_ context.Context, tunnelID string, desired any) (cloudflare.APIResult, error) {
	object, _ := desired.(map[string]any)
	client.calls = append(client.calls, recordingCloudflareCall{kind: "tunnel-update", id: tunnelID, desired: object})
	return cloudflare.APIResult{DryRun: true, Method: http.MethodPatch, Path: "/tunnel"}, nil
}

func (client *recordingCloudflareClient) DeleteTunnel(_ context.Context, tunnelID string) (cloudflare.APIResult, error) {
	client.calls = append(client.calls, recordingCloudflareCall{kind: "tunnel-delete", id: tunnelID})
	return cloudflare.APIResult{DryRun: true, Method: http.MethodDelete, Path: "/tunnel"}, nil
}
