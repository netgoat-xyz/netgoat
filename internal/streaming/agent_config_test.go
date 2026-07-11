package streaming

import "testing"

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
		},
	}

	copied := snapshot.copy()

	if !copied.AgentConfig.Cache.Enabled || copied.AgentConfig.Cache.TTLSeconds != 120 {
		t.Fatalf("agent cache config was not copied: %+v", copied.AgentConfig.Cache)
	}
	if copied.AgentConfig.RateLimit.Key != AgentKeyIP {
		t.Fatalf("agent rate limit config was not copied: %+v", copied.AgentConfig.RateLimit)
	}
}
