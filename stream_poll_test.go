package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"netgoat.xyz/agent/internal/config"
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
		ZeroTrustEnabled: &disabled,
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
