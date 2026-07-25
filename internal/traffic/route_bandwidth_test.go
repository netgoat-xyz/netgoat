package traffic

import (
	"testing"

	"netgoat.xyz/agent/internal/policy"
)

func TestBandwidthLimitersIsolateRoutesAndReplaceChangedPolicy(t *testing.T) {
	limiters := NewBandwidthLimiters()
	settings := policy.BandwidthSettings{Enabled: true, BytesPerSecond: 2048, BurstBytes: 4096, Key: policy.KeyIP}
	first := limiters.Limiter("domain:one", settings)
	second := limiters.Limiter("domain:two", settings)
	if first == second {
		t.Fatal("route bandwidth limiters must be isolated")
	}
	if again := limiters.Limiter("domain:one", settings); again != first {
		t.Fatal("same policy should reuse route limiter")
	}
	settings.BurstBytes = 2048
	if changed := limiters.Limiter("domain:one", settings); changed == first {
		t.Fatal("policy change should replace route limiter")
	}
}
