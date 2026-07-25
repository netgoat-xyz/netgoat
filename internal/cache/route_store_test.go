package cache

import (
	"net/http/httptest"
	"testing"

	"netgoat.xyz/agent/internal/policy"
)

func TestRouteStoresIsolateRoutesAndReplaceChangedPolicy(t *testing.T) {
	stores := NewRouteStores()
	settings := policy.CacheSettings{Enabled: true, TTLSeconds: 60, MaxEntries: 2, MaxBodyBytes: 1024}
	first := stores.Store("domain:one", settings)
	second := stores.Store("domain:two", settings)
	if first == second {
		t.Fatal("route caches must be isolated")
	}
	if again := stores.Store("domain:one", settings); again != first {
		t.Fatal("same policy should reuse route cache")
	}
	settings.TTLSeconds = 30
	if changed := stores.Store("domain:one", settings); changed == first {
		t.Fatal("policy change should replace route cache")
	}

	request := httptest.NewRequest("GET", "http://example.test/a", nil)
	if stores.Key("domain:one", request) == stores.Key("domain:two", request) {
		t.Fatal("cache keys must carry route identity")
	}
}
