package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"netgoat.xyz/agent/internal/config"
	"netgoat.xyz/agent/internal/policy"
	"netgoat.xyz/agent/internal/streaming"
)

func TestStreamSettingsFromConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.API.PollIntervalSeconds = 7
	cfg.API.ConnectionTimeoutSeconds = 3
	cfg.API.MaxRetryIntervalSeconds = 45

	settings := streamSettingsFromConfig(cfg)
	if settings.pollInterval != 7*time.Second || settings.requestTimeout != 3*time.Second || settings.maxRetryInterval != 45*time.Second {
		t.Fatalf("settings = %+v", settings)
	}
}

func TestSnapshotFromDomainsResponseHonorsActiveFlags(t *testing.T) {
	disabled := false
	payload := domainsResponse{
		ZeroTrustEnabled:  &disabled,
		PluginsConfigured: true,
		Plugins: config.PluginConfig{Installations: []config.PluginInstallation{{
			PluginID: "netgoat/noop",
			Config:   map[string]any{"mode": "observe"},
		}}},
		Domains: []domainRecord{
			{Domain: "enabled.example.test", TargetURL: "http://enabled"},
			{Domain: "disabled.example.test", TargetURL: "http://disabled", Active: false},
			{
				Domain:    "parent.example.test",
				TargetURL: "http://parent",
				Active:    "0",
				Subdomains: []subdomainRecord{
					{FullDomain: "child.parent.example.test", TargetURL: "http://child"},
				},
			},
			{
				Domain:    "mixed.example.test",
				TargetURL: "http://mixed",
				Subdomains: []subdomainRecord{
					{FullDomain: "off.mixed.example.test", TargetURL: "http://off", Active: 0.0},
					{FullDomain: "on.mixed.example.test", TargetURL: "http://on", Active: "enabled"},
				},
			},
		},
	}

	snapshot := snapshotFromDomainsResponse(payload)
	for _, route := range []string{"enabled.example.test", "mixed.example.test", "on.mixed.example.test"} {
		if _, ok := snapshot.Routes[route]; !ok {
			t.Errorf("expected active route %q", route)
		}
	}
	for _, route := range []string{"disabled.example.test", "parent.example.test", "child.parent.example.test", "off.mixed.example.test"} {
		if _, ok := snapshot.Routes[route]; ok {
			t.Errorf("inactive route %q was included", route)
		}
	}
	if !snapshot.ZeroTrustConfigured || snapshot.ZeroTrustEnabled {
		t.Fatalf("explicit false zero trust was not preserved: %+v", snapshot)
	}
	if !snapshot.PluginsConfigured || len(snapshot.Plugins.Installations) != 1 ||
		snapshot.Plugins.Installations[0].PluginID != "netgoat/noop" {
		t.Fatalf("plugin selection was not preserved: %+v", snapshot.Plugins)
	}
	payload.Plugins.Installations[0].Config["mode"] = "mutated"
	if snapshot.Plugins.Installations[0].Config["mode"] != "observe" {
		t.Fatalf("plugin selection was not cloned: %+v", snapshot.Plugins)
	}
}

func TestPollDomainsSkipsUnchangedSnapshots(t *testing.T) {
	payload := map[string]any{
		"domains": []map[string]any{{
			"domain":     "api.example.test",
			"target_url": "http://127.0.0.1:9000",
			"active":     true,
		}},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer server.Close()

	mgr := streaming.NewManager("")
	defer mgr.Close()
	state := &domainPollState{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	changed, err := pollDomains(ctx, mgr, server.URL, "", state)
	if err != nil || !changed {
		t.Fatalf("first poll changed/error = %v/%v", changed, err)
	}
	firstVersion := mgr.GetSnapshot().Version

	changed, err = pollDomains(ctx, mgr, server.URL, "", state)
	if err != nil || changed {
		t.Fatalf("second poll changed/error = %v/%v", changed, err)
	}
	if got := mgr.GetSnapshot().Version; got != firstVersion {
		t.Fatalf("unchanged poll advanced version from %d to %d", firstVersion, got)
	}
}

func TestSnapshotFromDomainsResponsePreservesRoutePolicies(t *testing.T) {
	enabled := true
	ttl := 30
	bytesPerSecond := 4096
	payload := domainsResponse{Domains: []domainRecord{{
		Domain:    "example.test",
		TargetURL: "http://upstream",
		Policy: policy.RoutePolicy{
			Cache:     &policy.CacheOverride{Enabled: &enabled, TTLSeconds: &ttl},
			Bandwidth: &policy.BandwidthOverride{BytesPerSecond: &bytesPerSecond},
		},
	}}}

	snapshot := snapshotFromDomainsResponse(payload)
	route := snapshot.Routes["example.test"]
	if route.Policy.Cache == nil || route.Policy.Cache.Enabled == nil || !*route.Policy.Cache.Enabled ||
		route.Policy.Cache.TTLSeconds == nil || *route.Policy.Cache.TTLSeconds != 30 {
		t.Fatalf("cache policy was not preserved: %+v", route.Policy)
	}
	if route.Policy.Bandwidth == nil || route.Policy.Bandwidth.BytesPerSecond == nil ||
		*route.Policy.Bandwidth.BytesPerSecond != 4096 {
		t.Fatalf("bandwidth policy was not preserved: %+v", route.Policy)
	}
}

func TestSnapshotFromDomainsResponseAcceptsRoutePolicyAlias(t *testing.T) {
	raw := []byte(`{
		"domains": [{
			"domain": "alias.example.test",
			"target_url": "http://upstream",
			"route_policy": {
				"cache": {"enabled": true, "ttl_seconds": 15}
			},
			"subdomains": [{
				"full_domain": "child.alias.example.test",
				"target_url": "http://child",
				"route_policy": {
					"bandwidth": {"bytes_per_second": 2048}
				}
			}]
		}]
	}`)
	var payload domainsResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	snapshot := snapshotFromDomainsResponse(payload)

	parent := snapshot.Routes["alias.example.test"]
	if parent.Policy.Cache == nil || parent.Policy.Cache.TTLSeconds == nil || *parent.Policy.Cache.TTLSeconds != 15 {
		t.Fatalf("domain route_policy alias was not applied: %+v", parent.Policy)
	}
	child := snapshot.Routes["child.alias.example.test"]
	if child.Policy.Bandwidth == nil || child.Policy.Bandwidth.BytesPerSecond == nil ||
		*child.Policy.Bandwidth.BytesPerSecond != 2048 {
		t.Fatalf("subdomain route_policy alias was not applied: %+v", child.Policy)
	}
}

func TestSnapshotFromDomainsResponsePrefersPolicyOverRoutePolicyAlias(t *testing.T) {
	raw := []byte(`{
		"domains": [{
			"domain": "both.example.test",
			"target_url": "http://upstream",
			"policy": {
				"cache": {"enabled": true, "ttl_seconds": 30}
			},
			"route_policy": {
				"cache": {"enabled": true, "ttl_seconds": 99},
				"bandwidth": {"bytes_per_second": 1024}
			}
		}]
	}`)
	var payload domainsResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	snapshot := snapshotFromDomainsResponse(payload)
	route := snapshot.Routes["both.example.test"]
	if route.Policy.Cache == nil || route.Policy.Cache.TTLSeconds == nil || *route.Policy.Cache.TTLSeconds != 30 {
		t.Fatalf("canonical policy should win: %+v", route.Policy)
	}
	if route.Policy.Bandwidth != nil {
		t.Fatalf("alias bandwidth should not merge when policy is set: %+v", route.Policy)
	}
}

func TestPollDomainsAppliesRoutePolicyAlias(t *testing.T) {
	payload := map[string]any{
		"domains": []map[string]any{{
			"domain":     "polled.example.test",
			"target_url": "http://127.0.0.1:9000",
			"active":     true,
			"route_policy": map[string]any{
				"cache": map[string]any{"enabled": true, "ttl_seconds": 12},
			},
		}},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer server.Close()

	mgr := streaming.NewManager("")
	defer mgr.Close()
	state := &domainPollState{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	changed, err := pollDomains(ctx, mgr, server.URL, "", state)
	if err != nil || !changed {
		t.Fatalf("poll changed/error = %v/%v", changed, err)
	}
	route := mgr.GetSnapshot().Routes["polled.example.test"]
	if route.Policy.Cache == nil || route.Policy.Cache.TTLSeconds == nil || *route.Policy.Cache.TTLSeconds != 12 {
		t.Fatalf("polled route_policy alias was not applied: %+v", route.Policy)
	}
}
