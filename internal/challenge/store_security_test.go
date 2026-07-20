package challenge

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStoreDoesNotStartPerStoreGoroutines(t *testing.T) {
	before := runtime.NumGoroutine()
	stores := make([]*Store, 512)
	for i := range stores {
		stores[i] = NewStore()
	}
	runtime.Gosched()
	after := runtime.NumGoroutine()
	runtime.KeepAlive(stores)

	if delta := after - before; delta > 8 {
		t.Fatalf("creating stores added %d goroutines; want no per-store workers", delta)
	}
}

func TestStoreBoundsOutstandingChallenges(t *testing.T) {
	store := newStore(storeConfig{maxChallenges: 3})
	created := make([]*Challenge, 0, 4)
	for i := 0; i < 4; i++ {
		created = append(created, store.Create(
			fmt.Sprintf("192.0.2.%d", i),
			"test agent",
			60,
			ChallengeClick,
		))
	}

	if got := challengeCount(store); got != 3 {
		t.Fatalf("challenge count = %d, want 3", got)
	}
	if _, ok := store.Get(created[0].ID); ok {
		t.Fatal("oldest challenge survived capacity eviction")
	}
	for _, challenge := range created[1:] {
		if _, ok := store.Get(challenge.ID); !ok {
			t.Fatalf("newer challenge %q was unexpectedly evicted", challenge.ID)
		}
	}
}

func TestStoreBoundsVerifiedBindingsAndRefreshesRecency(t *testing.T) {
	store := newStore(storeConfig{maxVerified: 2})
	verifyBinding(t, store, "192.0.2.1")
	verifyBinding(t, store, "192.0.2.2")
	verifyBinding(t, store, "192.0.2.1")
	verifyBinding(t, store, "192.0.2.3")

	if got := verifiedCount(store); got != 2 {
		t.Fatalf("verified count = %d, want 2", got)
	}
	if store.IsVerified("192.0.2.2") {
		t.Fatal("least-recently verified binding survived capacity eviction")
	}
	if !store.IsVerified("192.0.2.1") || !store.IsVerified("192.0.2.3") {
		t.Fatal("recently verified bindings were unexpectedly evicted")
	}
}

func TestStoreCleansExpirationOpportunistically(t *testing.T) {
	now := time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC)
	store := newStore(storeConfig{now: func() time.Time { return now }})
	pending := store.Create("192.0.2.1", "test agent", 40, ChallengeText)
	verifyBinding(t, store, "192.0.2.2")

	now = now.Add(defaultVerificationTTL)
	if _, ok := store.Get(pending.ID); ok {
		t.Fatal("expired challenge remained readable")
	}
	if store.IsVerified("192.0.2.2") {
		t.Fatal("expired verification remained valid")
	}
	if got := challengeCount(store); got != 0 {
		t.Fatalf("expired challenge count = %d, want 0", got)
	}
	if got := verifiedCount(store); got != 0 {
		t.Fatalf("expired verified count = %d, want 0", got)
	}
}

func TestStoreLimitsFailedAttemptsForMatchingBinding(t *testing.T) {
	store := newStore(storeConfig{maxFailedAttempts: 3})
	challenge := store.Create("192.0.2.1", "test agent", 40, ChallengeText)

	for i := 0; i < 10; i++ {
		if store.Verify(challenge.ID, "wrong", "192.0.2.99") {
			t.Fatal("wrong binding verified challenge")
		}
	}
	if _, ok := store.Get(challenge.ID); !ok {
		t.Fatal("unrelated bindings consumed the challenge attempt budget")
	}

	for i := 0; i < 2; i++ {
		if store.Verify(challenge.ID, "wrong", "192.0.2.1") {
			t.Fatal("wrong answer verified challenge")
		}
		if _, ok := store.Get(challenge.ID); !ok {
			t.Fatalf("challenge removed after only %d matching failures", i+1)
		}
	}
	if store.Verify(challenge.ID, strings.Repeat("x", maxAnswerBytes+1), "192.0.2.1") {
		t.Fatal("oversized answer verified challenge")
	}
	if _, ok := store.Get(challenge.ID); ok {
		t.Fatal("challenge survived its failed-attempt budget")
	}
}

func TestStoreReturnsBoundedSnapshots(t *testing.T) {
	store := NewStore()
	binding := strings.Repeat("b", maxStoredBindingBytes*4)
	userAgent := strings.Repeat("u", maxStoredUserAgentBytes*4)
	challenge := store.Create(binding, userAgent, 40, ChallengeText)
	originalID := challenge.ID
	originalAnswer := challenge.Answer

	if len(challenge.IP) != maxStoredBindingBytes {
		t.Fatalf("stored binding length = %d, want %d", len(challenge.IP), maxStoredBindingBytes)
	}
	if len(challenge.UserAgent) != maxStoredUserAgentBytes {
		t.Fatalf("stored user-agent length = %d, want %d", len(challenge.UserAgent), maxStoredUserAgentBytes)
	}

	challenge.Answer = "tampered"
	challenge.ExpiresAt = time.Time{}
	challenge.IP = "tampered"
	retrieved, ok := store.Get(originalID)
	if !ok {
		t.Fatal("mutating Create result changed stored challenge")
	}
	if retrieved.Answer != originalAnswer || retrieved.IP == "tampered" {
		t.Fatalf("stored challenge was mutated through Create result: %+v", retrieved)
	}

	retrieved.Answer = "tampered again"
	if !store.Verify(originalID, originalAnswer, binding) {
		t.Fatal("mutating Get result changed stored answer or full binding match")
	}

	customType := ChallengeType(strings.Repeat("t", maxStoredChallengeTypeBytes*4))
	custom := store.Create("192.0.2.1", "test agent", 0, customType)
	if len(custom.Type) != maxStoredChallengeTypeBytes {
		t.Fatalf("stored challenge type length = %d, want %d", len(custom.Type), maxStoredChallengeTypeBytes)
	}
}

func TestStoreConcurrentOperationsRemainBounded(t *testing.T) {
	const (
		maxChallenges = 64
		maxVerified   = 32
		workers       = 24
		iterations    = 150
	)
	store := newStore(storeConfig{
		maxChallenges: maxChallenges,
		maxVerified:   maxVerified,
	})

	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for iteration := 0; iteration < iterations; iteration++ {
				binding := fmt.Sprintf("198.51.%d.%d", worker, iteration)
				challenge := store.Create(binding, "concurrent agent", 40, ChallengeText)
				if iteration%3 == 0 {
					_, _ = store.Get(challenge.ID)
					_ = store.Verify(challenge.ID, "wrong", binding)
				} else {
					_ = store.Verify(challenge.ID, challenge.Answer, binding)
					_ = store.IsVerified(binding)
				}
			}
		}(worker)
	}
	wg.Wait()

	store.mu.RLock()
	defer store.mu.RUnlock()
	if got := len(store.challenges); got > maxChallenges {
		t.Fatalf("challenge count = %d, maximum %d", got, maxChallenges)
	}
	if got := len(store.verified); got > maxVerified {
		t.Fatalf("verified count = %d, maximum %d", got, maxVerified)
	}
	if store.challengeOrder.Len() != len(store.challenges) {
		t.Fatalf("challenge order length = %d, map length = %d", store.challengeOrder.Len(), len(store.challenges))
	}
	if store.verifiedOrder.Len() != len(store.verified) {
		t.Fatalf("verified order length = %d, map length = %d", store.verifiedOrder.Len(), len(store.verified))
	}
}

func verifyBinding(t *testing.T, store *Store, binding string) {
	t.Helper()
	challenge := store.Create(binding, "test agent", 40, ChallengeText)
	if !store.Verify(challenge.ID, challenge.Answer, binding) {
		t.Fatalf("failed to verify binding %q", binding)
	}
}

func challengeCount(store *Store) int {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return len(store.challenges)
}

func verifiedCount(store *Store) int {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return len(store.verified)
}
