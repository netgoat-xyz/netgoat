package database

import (
	"database/sql"
	"os"
	"path/filepath"
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

func TestGetRouteTargetsNormalizesDomain(t *testing.T) {
	db := newTestDB(t)
	insertRoute(t, db, "domain", "api.example.test", "http://exact")

	for _, host := range []string{"API.EXAMPLE.TEST", "Api.Example.Test:8443", "api.example.test."} {
		match, err := GetRouteTargets(db, host, "/")
		if err != nil {
			t.Fatalf("GetRouteTargets(%q) failed: %v", host, err)
		}
		if got := match.Targets[0].URL; got != "http://exact" {
			t.Fatalf("target for %q = %s, want http://exact", host, got)
		}
	}
}

func TestGetRouteTargetsUsesConfiguredPathAsBalancerKey(t *testing.T) {
	db := newTestDB(t)
	res, err := db.Exec(
		`INSERT INTO routes (route_type, domain, path_prefix, target_url, active) VALUES ('path', '', '/api/', 'http://path', 1)`,
	)
	if err != nil {
		t.Fatalf("insert path route: %v", err)
	}
	id, _ := res.LastInsertId()
	if err := SetRouteTargets(db, int(id), []RouteTarget{{URL: "http://path"}}); err != nil {
		t.Fatalf("SetRouteTargets: %v", err)
	}

	for _, path := range []string{"/api/users", "/api/orders/42"} {
		match, err := GetRouteTargets(db, "", path)
		if err != nil {
			t.Fatalf("GetRouteTargets(%q): %v", path, err)
		}
		if match.RouteKey != "path:/api/" {
			t.Fatalf("RouteKey for %q = %q, want path:/api/", path, match.RouteKey)
		}
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

func TestBackupToAndOpenWithFailoverPromotesStandby(t *testing.T) {
	dir := t.TempDir()
	primary := filepath.Join(dir, "proxy.db")
	standby := filepath.Join(dir, "proxy.standby.db")

	db, err := Init(primary)
	if err != nil {
		t.Fatalf("Init primary failed: %v", err)
	}
	insertRoute(t, db, "domain", "failover.example.test", "http://failover-target")
	if err := BackupTo(db, standby); err != nil {
		t.Fatalf("BackupTo failed: %v", err)
	}
	_ = db.Close()

	if err := os.WriteFile(primary, []byte("this is not a sqlite database"), 0644); err != nil {
		t.Fatalf("corrupt primary: %v", err)
	}
	_ = os.Remove(primary + "-wal")
	_ = os.Remove(primary + "-shm")

	recoveredDB, recovered, err := OpenWithFailover(primary, standby)
	if err != nil {
		t.Fatalf("OpenWithFailover failed: %v", err)
	}
	t.Cleanup(func() { _ = recoveredDB.Close() })
	if !recovered {
		t.Fatal("expected recovered=true after promoting standby")
	}

	match, err := GetRouteTargets(recoveredDB, "failover.example.test", "/")
	if err != nil {
		t.Fatalf("GetRouteTargets failed: %v", err)
	}
	if got := match.Targets[0].URL; got != "http://failover-target" {
		t.Fatalf("target = %s, want http://failover-target", got)
	}
}

func TestOpenWithFailoverRecreatesWhenPrimaryAndStandbyUnusable(t *testing.T) {
	dir := t.TempDir()
	primary := filepath.Join(dir, "proxy.db")
	standby := filepath.Join(dir, "proxy.standby.db")

	if err := os.WriteFile(primary, []byte("corrupt-primary"), 0644); err != nil {
		t.Fatalf("write primary: %v", err)
	}
	if err := os.WriteFile(standby, []byte("corrupt-standby"), 0644); err != nil {
		t.Fatalf("write standby: %v", err)
	}

	db, recovered, err := OpenWithFailover(primary, standby)
	if err != nil {
		t.Fatalf("OpenWithFailover failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if recovered {
		t.Fatal("expected recovered=false when recreating empty primary")
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM routes`).Scan(&count); err != nil {
		t.Fatalf("query routes: %v", err)
	}
	if count == 0 {
		t.Fatal("recreated database should include seeded defaults")
	}
}

func TestBackupToCreatesOpenableCopy(t *testing.T) {
	dir := t.TempDir()
	primary := filepath.Join(dir, "proxy.db")
	standby := filepath.Join(dir, "proxy.standby.db")

	db, err := Init(primary)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	insertRoute(t, db, "domain", "backup.example.test", "http://backup-target")

	if err := BackupTo(db, standby); err != nil {
		t.Fatalf("BackupTo failed: %v", err)
	}

	standbyDB, err := Init(standby)
	if err != nil {
		t.Fatalf("Init standby failed: %v", err)
	}
	t.Cleanup(func() { _ = standbyDB.Close() })

	match, err := GetRouteTargets(standbyDB, "backup.example.test", "/")
	if err != nil {
		t.Fatalf("GetRouteTargets failed: %v", err)
	}
	if got := match.Targets[0].URL; got != "http://backup-target" {
		t.Fatalf("target = %s, want http://backup-target", got)
	}
}

func TestBackupToOverwritesExistingStandby(t *testing.T) {
	dir := t.TempDir()
	primary := filepath.Join(dir, "proxy.db")
	standby := filepath.Join(dir, "proxy.standby.db")

	db, err := Init(primary)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := BackupTo(db, standby); err != nil {
		t.Fatalf("first BackupTo failed: %v", err)
	}
	insertRoute(t, db, "domain", "second.example.test", "http://second-target")
	if err := BackupTo(db, standby); err != nil {
		t.Fatalf("second BackupTo (overwrite) failed: %v", err)
	}

	standbyDB, err := Init(standby)
	if err != nil {
		t.Fatalf("Init standby failed: %v", err)
	}
	t.Cleanup(func() { _ = standbyDB.Close() })

	match, err := GetRouteTargets(standbyDB, "second.example.test", "/")
	if err != nil {
		t.Fatalf("GetRouteTargets failed: %v", err)
	}
	if got := match.Targets[0].URL; got != "http://second-target" {
		t.Fatalf("target = %s, want http://second-target", got)
	}
}

func TestReplaceFileOverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	if err := os.WriteFile(src, []byte("new"), 0644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	if err := os.WriteFile(dst, []byte("old"), 0644); err != nil {
		t.Fatalf("write dst: %v", err)
	}
	if err := replaceFile(src, dst); err != nil {
		t.Fatalf("replaceFile failed: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(got) != "new" {
		t.Fatalf("dst = %q, want %q", got, "new")
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("src should be gone after rename, stat err = %v", err)
	}
}
