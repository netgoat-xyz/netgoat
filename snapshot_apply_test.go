package main

import (
	"database/sql"
	"testing"

	"netgoat.xyz/agent/internal/database"
	"netgoat.xyz/agent/internal/streaming"
)

func TestApplySnapshotReconcilesRoutesAndRulesAtomically(t *testing.T) {
	db, err := database.Init(":memory:")
	if err != nil {
		t.Fatalf("database.Init: %v", err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()

	first := &streaming.ConfigSnapshot{
		RoutesConfigured: true,
		Routes: map[string]streaming.RouteData{
			"old.example.test": {Type: "domain", Target: "http://127.0.0.1:9001"},
		},
		WAFRulesConfigured: true,
		WAFRules: map[string]streaming.WAFRuleData{
			"old": {Name: "old", Expression: `Path == "/old"`, Action: "BLOCK"},
		},
	}
	if err := applySnapshotToDB(db, first); err != nil {
		t.Fatalf("apply first snapshot: %v", err)
	}

	disabled := &streaming.ConfigSnapshot{
		RoutesConfigured: true,
		Routes: map[string]streaming.RouteData{
			"new.example.test": {Type: "domain", Target: "http://127.0.0.1:9002"},
		},
		WAFRulesConfigured: true,
		WAFRules: map[string]streaming.WAFRuleData{
			"new": {Name: "new", Expression: `Path == "/new"`, Action: "BLOCK"},
		},
		ZeroTrustConfigured: true,
		ZeroTrustEnabled:    false,
	}
	if err := applySnapshotToDB(db, disabled); err != nil {
		t.Fatalf("apply replacement snapshot: %v", err)
	}

	if _, err := database.GetRouteTargets(db, "old.example.test", "/"); err != sql.ErrNoRows {
		t.Fatalf("stale route lookup error = %v, want sql.ErrNoRows", err)
	}
	if _, err := database.GetRouteTargets(db, "new.example.test", "/"); err != nil {
		t.Fatalf("new route lookup: %v", err)
	}
	var oldRules, newRules int
	_ = db.QueryRow(`SELECT COUNT(*) FROM waf_rules WHERE name = 'old'`).Scan(&oldRules)
	_ = db.QueryRow(`SELECT COUNT(*) FROM waf_rules WHERE name = 'new'`).Scan(&newRules)
	if oldRules != 0 || newRules != 1 {
		t.Fatalf("WAF rule counts old/new = %d/%d", oldRules, newRules)
	}
	if database.IsZeroTrustEnabled(db) {
		t.Fatal("explicit zero-trust false was not applied")
	}
}

func TestApplySnapshotRollsBackInvalidReplacement(t *testing.T) {
	db, err := database.Init(":memory:")
	if err != nil {
		t.Fatalf("database.Init: %v", err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()

	valid := &streaming.ConfigSnapshot{
		RoutesConfigured: true,
		Routes: map[string]streaming.RouteData{
			"stable.example.test": {Type: "domain", Target: "http://127.0.0.1:9001"},
		},
	}
	if err := applySnapshotToDB(db, valid); err != nil {
		t.Fatalf("apply valid snapshot: %v", err)
	}

	invalid := &streaming.ConfigSnapshot{
		RoutesConfigured: true,
		Routes: map[string]streaming.RouteData{
			"broken.example.test": {Type: "domain", Target: "file:///etc/passwd"},
		},
	}
	if err := applySnapshotToDB(db, invalid); err == nil {
		t.Fatal("invalid replacement should fail")
	}
	if _, err := database.GetRouteTargets(db, "stable.example.test", "/"); err != nil {
		t.Fatalf("rollback did not preserve stable route: %v", err)
	}
	if _, err := database.GetRouteTargets(db, "broken.example.test", "/"); err != sql.ErrNoRows {
		t.Fatalf("invalid route lookup error = %v, want sql.ErrNoRows", err)
	}
}

func TestMergeConfigSnapshotsKeepsLocalFallbacks(t *testing.T) {
	local := &streaming.ConfigSnapshot{RoutesConfigured: true, Routes: map[string]streaming.RouteData{
		"local.example.test":  {Type: "domain", Target: "http://local"},
		"shared.example.test": {Type: "domain", Target: "http://local-shared"},
	}}
	remote := &streaming.ConfigSnapshot{RoutesConfigured: true, Routes: map[string]streaming.RouteData{
		"remote.example.test": {Type: "domain", Target: "http://remote"},
		"shared.example.test": {Type: "domain", Target: "http://remote-shared"},
	}}

	merged := mergeConfigSnapshots(local, remote)
	if len(merged.Routes) != 3 {
		t.Fatalf("merged routes = %d, want 3", len(merged.Routes))
	}
	if got := merged.Routes["shared.example.test"].Target; got != "http://remote-shared" {
		t.Fatalf("remote route did not override local fallback: %q", got)
	}
}
