package database

import (
	"database/sql"
	"testing"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := Init(":memory:")
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func insertRoute(t *testing.T, db *sql.DB, routeType, domain, target string) {
	t.Helper()

	res, err := db.Exec(
		`INSERT INTO routes (route_type, domain, path_prefix, target_url, active) VALUES (?, ?, '', ?, 1)`,
		routeType, domain, target)
	if err != nil {
		t.Fatalf("insert route %s failed: %v", domain, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId failed: %v", err)
	}
	if err := SetRouteTargets(db, int(id), []RouteTarget{{URL: target, HealthCheck: "http"}}); err != nil {
		t.Fatalf("SetRouteTargets failed: %v", err)
	}
}

func TestGetRouteTargetsExactDomainWinsOverWildcard(t *testing.T) {
	db := newTestDB(t)
	insertRoute(t, db, "wildcard", "*.example.test", "http://wildcard")
	insertRoute(t, db, "domain", "api.example.test", "http://exact")

	match, err := GetRouteTargets(db, "api.example.test", "/")
	if err != nil {
		t.Fatalf("GetRouteTargets failed: %v", err)
	}
	if got := match.Targets[0].URL; got != "http://exact" {
		t.Fatalf("target = %s, want http://exact", got)
	}
}

func TestGetRouteTargetsWildcardDomain(t *testing.T) {
	db := newTestDB(t)
	insertRoute(t, db, "wildcard", "*.example.test", "http://wildcard")

	match, err := GetRouteTargets(db, "app.example.test", "/")
	if err != nil {
		t.Fatalf("GetRouteTargets failed: %v", err)
	}
	if got := match.Targets[0].URL; got != "http://wildcard" {
		t.Fatalf("target = %s, want http://wildcard", got)
	}
	if match.RouteKey != "domain:*.example.test" {
		t.Fatalf("RouteKey = %s, want domain:*.example.test", match.RouteKey)
	}
}

func TestGetRouteTargetsRegexDomain(t *testing.T) {
	db := newTestDB(t)
	insertRoute(t, db, "regex", `^api-[0-9]+\.example\.test$`, "http://regex")

	match, err := GetRouteTargets(db, "api-42.example.test", "/")
	if err != nil {
		t.Fatalf("GetRouteTargets failed: %v", err)
	}
	if got := match.Targets[0].URL; got != "http://regex" {
		t.Fatalf("target = %s, want http://regex", got)
	}
}
