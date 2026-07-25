package policy

import "testing"

func boolPointer(value bool) *bool      { return &value }
func intPointer(value int) *int         { return &value }
func keyPointer(value KeyMode) *KeyMode { return &value }

func TestRoutePolicyUsesInheritanceAndExplicitDisable(t *testing.T) {
	globalCache := CacheSettings{Enabled: true, TTLSeconds: 120, MaxEntries: 12, MaxBodyBytes: 4096}
	globalBandwidth := BandwidthSettings{Enabled: true, BytesPerSecond: 4096, BurstBytes: 8192, Key: KeyHost}

	cache, err := ResolveCache(globalCache, &CacheOverride{Enabled: boolPointer(false)})
	if err != nil {
		t.Fatalf("ResolveCache: %v", err)
	}
	if cache.Enabled || cache.TTLSeconds != 120 || cache.MaxEntries != 12 {
		t.Fatalf("cache = %+v, want inherited disabled override", cache)
	}

	bandwidth, err := ResolveBandwidth(globalBandwidth, &BandwidthOverride{
		Enabled: boolPointer(true), BytesPerSecond: intPointer(2048), Key: keyPointer(KeyIP),
	})
	if err != nil {
		t.Fatalf("ResolveBandwidth: %v", err)
	}
	if !bandwidth.Enabled || bandwidth.BytesPerSecond != 2048 || bandwidth.BurstBytes != 8192 || bandwidth.Key != KeyIP {
		t.Fatalf("bandwidth = %+v, want merged override", bandwidth)
	}
}

func TestRoutePolicyRejectsUnsafeValues(t *testing.T) {
	_, err := ResolveCache(DefaultCacheSettings(), &CacheOverride{TTLSeconds: intPointer(0)})
	if err == nil {
		t.Fatal("expected zero TTL to be rejected")
	}

	badKey := KeyMode("tenant")
	_, err = ResolveBandwidth(DefaultBandwidthSettings(), &BandwidthOverride{Key: &badKey})
	if err == nil {
		t.Fatal("expected unsupported key mode to be rejected")
	}
}

func TestCloneDoesNotShareOverridePointers(t *testing.T) {
	enabled := true
	ttl := 60
	original := RoutePolicy{Cache: &CacheOverride{Enabled: &enabled, TTLSeconds: &ttl}}
	clone := original.Clone()
	*clone.Cache.Enabled = false
	*clone.Cache.TTLSeconds = 30
	if !*original.Cache.Enabled || *original.Cache.TTLSeconds != 60 {
		t.Fatalf("clone mutated original: %+v", original)
	}
}
