package streaming

import (
	"testing"

	"netgoat.xyz/agent/internal/config"
)

func TestConfigSnapshotCopyIncludesAgentConfig(t *testing.T) {
	snapshot := &ConfigSnapshot{
		Routes: map[string]RouteData{
			"example.com": {Type: "domain", Target: "http://localhost:9000"},
		},
		WAFRules: map[string]WAFRuleData{},
		Users:    []UserData{},
		AgentConfig: AgentConfigData{
			Cache: AgentCacheConfig{
				Enabled:    true,
				TTLSeconds: 120,
			},
			RateLimit: AgentRateLimitConfig{
				Enabled: true,
				Key:     AgentKeyIP,
			},
			DynamicRules: AgentDynamicRulesConfig{
				Enabled: true,
				Rules: []AgentDynamicRuleData{{
					Name:     "block-admin",
					Language: "typescript",
					Source:   "export function evaluate() { return null; }",
				}},
			},
		},
		PluginsConfigured: true,
		Plugins: config.PluginConfig{Installations: []config.PluginInstallation{{
			PluginID: "example.test/plugin",
			Config:   map[string]any{"nested": map[string]any{"mode": "observe"}},
		}}},
	}

	copied := snapshot.copy()

	if !copied.AgentConfig.Cache.Enabled || copied.AgentConfig.Cache.TTLSeconds != 120 {
		t.Fatalf("agent cache config was not copied: %+v", copied.AgentConfig.Cache)
	}
	if copied.AgentConfig.RateLimit.Key != AgentKeyIP {
		t.Fatalf("agent rate limit config was not copied: %+v", copied.AgentConfig.RateLimit)
	}
	copied.AgentConfig.DynamicRules.Rules[0].Name = "mutated"
	if snapshot.AgentConfig.DynamicRules.Rules[0].Name != "block-admin" {
		t.Fatalf("agent dynamic rules were not independently copied: %+v", snapshot.AgentConfig.DynamicRules)
	}
	if !copied.PluginsConfigured || len(copied.Plugins.Installations) != 1 {
		t.Fatalf("plugin selection was not copied: %+v", copied.Plugins)
	}
	copied.Plugins.Installations[0].Config["nested"].(map[string]any)["mode"] = "mutated"
	if snapshot.Plugins.Installations[0].Config["nested"].(map[string]any)["mode"] != "observe" {
		t.Fatalf("plugin config was not independently copied: %+v", snapshot.Plugins)
	}
}
