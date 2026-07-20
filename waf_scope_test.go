package main

import (
	"strings"
	"testing"

	"netgoat.xyz/agent/internal/waf"
)

func TestSnapshotScopesWAFRulesAndKeepsDuplicateNames(t *testing.T) {
	payload := domainsResponse{WAFRules: []wafRuleRecord{
		{ID: "one", Name: "block admin", Expression: `Path == "/admin"`, Hosts: []string{"API.Example.Test."}, ProxyConfigID: "proxy-one"},
		{ID: "two", Name: "block admin", Expression: `Path == "/admin"`, Hosts: []string{"www.example.test"}, ProxyConfigID: "proxy-two"},
		{ID: "stale", Name: "stale", Expression: "true", ProxyConfigID: "missing"},
	}}
	snapshot := snapshotFromDomainsResponse(payload)
	if len(snapshot.WAFRules) != 2 {
		t.Fatalf("WAF rule count = %d, want 2", len(snapshot.WAFRules))
	}
	for _, rule := range snapshot.WAFRules {
		if !strings.Contains(rule.Expression, "Host ==") || !strings.Contains(rule.Name, "[") {
			t.Fatalf("rule was not scoped: %+v", rule)
		}
		if err := waf.ValidateExpression(rule.Expression); err != nil {
			t.Fatalf("scoped expression is invalid: %v", err)
		}
	}
}

func TestScopedWAFExpressionEscapesAndDeduplicatesHosts(t *testing.T) {
	expression, hosts := scopedWAFExpression("Method == `GET`", []string{"api.example.test", "API.EXAMPLE.TEST", `odd\"host`})
	if len(hosts) != 2 || !strings.Contains(expression, `odd\\\"host`) {
		t.Fatalf("expression/hosts = %q / %#v", expression, hosts)
	}
	if err := waf.ValidateExpression(expression); err != nil {
		t.Fatalf("scoped expression is invalid: %v", err)
	}
}
