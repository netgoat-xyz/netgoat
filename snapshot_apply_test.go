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

func TestApplySnapshotReconcilesOnlyAuthoritativeUsersAndDomains(t *testing.T) {
	db, err := database.Init(":memory:")
	if err != nil {
		t.Fatalf("database.Init: %v", err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()

	initial := &streaming.ConfigSnapshot{
		UsersConfigured: true,
		Users: []streaming.UserData{
			{Username: "alice", PasswordHash: "alice-v1"},
			{Username: "stale", PasswordHash: "stale-v1"},
		},
		UserDomainsConfigured: true,
		UserDomains: []streaming.UserDomainData{
			{Username: "alice", Domain: "alice.example.test", TargetURL: "http://alice", Active: true},
			{Username: "stale", Domain: "stale.example.test", TargetURL: "http://stale", Active: true},
		},
	}
	if err := applySnapshotToDB(db, initial); err != nil {
		t.Fatalf("apply initial user snapshot: %v", err)
	}

	replacement := &streaming.ConfigSnapshot{
		UsersConfigured: true,
		Users: []streaming.UserData{
			{Username: "alice", PasswordHash: "alice-v2", Email: "alice@example.test"},
		},
		UserDomainsConfigured: true,
		UserDomains: []streaming.UserDomainData{
			{Username: "alice", Domain: "new.example.test", TargetURL: "http://new", Active: true},
		},
	}
	if err := applySnapshotToDB(db, replacement); err != nil {
		t.Fatalf("apply replacement user snapshot: %v", err)
	}

	var users, domains int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&users); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM user_proxy_records`).Scan(&domains); err != nil {
		t.Fatalf("count user domains: %v", err)
	}
	if users != 1 || domains != 1 {
		t.Fatalf("authoritative replacement kept stale data: users=%d domains=%d", users, domains)
	}
	if _, err := database.GetUserID(db, "stale"); err != sql.ErrNoRows {
		t.Fatalf("stale user lookup = %v, want sql.ErrNoRows", err)
	}
	var passwordHash, email string
	if err := db.QueryRow(`SELECT password_hash, COALESCE(email, '') FROM users WHERE username = 'alice'`).Scan(&passwordHash, &email); err != nil {
		t.Fatalf("load retained user: %v", err)
	}
	if passwordHash != "alice-v2" || email != "alice@example.test" {
		t.Fatalf("retained user was not refreshed: hash=%q email=%q", passwordHash, email)
	}
}

func TestApplySnapshotDoesNotReconcileUsersWithoutConfiguredSignal(t *testing.T) {
	db, err := database.Init(":memory:")
	if err != nil {
		t.Fatalf("database.Init: %v", err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()

	seed := &streaming.ConfigSnapshot{
		UsersConfigured:       true,
		Users:                 []streaming.UserData{{Username: "local", PasswordHash: "local-hash"}},
		UserDomainsConfigured: true,
		UserDomains: []streaming.UserDomainData{{
			Username: "local", Domain: "local.example.test", TargetURL: "http://local", Active: true,
		}},
	}
	if err := applySnapshotToDB(db, seed); err != nil {
		t.Fatalf("seed user snapshot: %v", err)
	}

	// This mirrors an older control plane: the lists are absent/empty, but it
	// never claims ownership of either collection.
	if err := applySnapshotToDB(db, &streaming.ConfigSnapshot{}); err != nil {
		t.Fatalf("apply non-authoritative snapshot: %v", err)
	}

	var users, domains int
	_ = db.QueryRow(`SELECT COUNT(*) FROM users WHERE username = 'local'`).Scan(&users)
	_ = db.QueryRow(`SELECT COUNT(*) FROM user_proxy_records WHERE domain = 'local.example.test'`).Scan(&domains)
	if users != 1 || domains != 1 {
		t.Fatalf("non-authoritative snapshot removed local data: users=%d domains=%d", users, domains)
	}
}

func TestApplySnapshotRollsBackConfiguredUserReconciliation(t *testing.T) {
	db, err := database.Init(":memory:")
	if err != nil {
		t.Fatalf("database.Init: %v", err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()

	if err := applySnapshotToDB(db, &streaming.ConfigSnapshot{
		UsersConfigured: true,
		Users:           []streaming.UserData{{Username: "stable", PasswordHash: "hash"}},
	}); err != nil {
		t.Fatalf("seed stable user: %v", err)
	}

	err = applySnapshotToDB(db, &streaming.ConfigSnapshot{
		UsersConfigured:       true,
		UserDomainsConfigured: true,
		UserDomains: []streaming.UserDomainData{{
			Username: "missing", Domain: "broken.example.test", TargetURL: "http://broken", Active: true,
		}},
	})
	if err == nil {
		t.Fatal("invalid configured user-domain snapshot should fail")
	}
	if _, err := database.GetUserID(db, "stable"); err != nil {
		t.Fatalf("rollback did not preserve stable user: %v", err)
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
