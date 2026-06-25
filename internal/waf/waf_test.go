package waf

import (
	"database/sql"
	"net/http/httptest"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// setuptestDB creates an in-memory SQLite database and seeds it with mock rules for testing.
func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open in-memory db: %v", err)
	}
	db.SetMaxOpenConns(1)
	

	// Create the WAF rules table schema
	_, err = db.Exec(`CREATE TABLE waf_rules (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		expression TEXT NOT NULL,
		action TEXT NOT NULL DEFAULT 'BLOCK',
		priority INTEGER DEFAULT 0
	);`)
	if err != nil {
		t.Fatalf("Failed to create waf_rules table: %v", err)
	}

	// Seed test rules.
	// Note: We use \s (whitespace) here to prove that the URL decoding in waf.go
	// successfully translates %20 into real spaces before the regex runs
	rules := []struct {
		Name       string
		Expression string
	}{
		{"Block SQLi Query", `RawQuery matches ".*(?i)(union\\s+select|1=1).*"`},
		{"Block SSRF", `RawQuery matches ".*(?i)(169\\.254\\.169\\.254).*"`},
		{"Block Admin Path", `Path startsWith "/admin"`},
	}

	for _, rule := range rules {
		_, err = db.Exec(`INSERT INTO waf_rules (name, expression, action, priority) VALUES (?, ?, 'BLOCK', 10)`, rule.Name, rule.Expression)
		if err != nil {
			t.Fatalf("Failed to insert rule %s: %v", rule.Name, err)
		}
	}

	return db
}

func TestWAFCheck(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Table driven test cases
	tests := []struct {
		name          string
		method        string
		targetURL     string
		expectBlocked bool
		expectedRule  string
	}{
		{
			name:          "Safe Request - Root",
			method:        "GET",
			targetURL:     "/",
			expectBlocked: false,
			expectedRule:  "",
		},
		{
			name:          "Safe Request - Normal Query",
			method:        "GET",
			targetURL:     "/products?id=123&sort=asc",
			expectBlocked: false,
			expectedRule:  "",
		},
		{
			name:          "Blocked - Admin Path",
			method:        "GET",
			targetURL:     "/admin/dashboard",
			expectBlocked: true,
			expectedRule:  "Block Admin Path",
		},
		{
			name:          "Blocked - SSRF Attempt",
			method:        "GET",
			targetURL:     "/fetch?url=http://169.254.169.254/metadata",
			expectBlocked: true,
			expectedRule:  "Block SSRF",
		},
		{
			name:          "Blocked - SQLi with URL Encoded Spaces (%20)",
			method:        "GET",
			targetURL:     "/?id=1%20UNION%20SELECT%20password",
			expectBlocked: true,
			expectedRule:  "Block SQLi Query",
		},
		{
			name:          "Blocked - SQLi with URL Encoded Spaces (+)",
			method:        "GET",
			targetURL:     "/?id=1+UNION+SELECT+password",
			expectBlocked: true,
			expectedRule:  "Block SQLi Query",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// mock HTTP request
			req := httptest.NewRequest(tc.method, tc.targetURL, nil)

			// Run the WAF check
			blocked, ruleName := Check(db, req, false)

			if blocked != tc.expectBlocked {
				t.Errorf("Expected blocked: %v, got: %v", tc.expectBlocked, blocked)
			}

			if tc.expectBlocked && ruleName != tc.expectedRule {
				t.Errorf("Expected rule triggered: %s, got: %s", tc.expectedRule, ruleName)
			}
		})
	}
}
