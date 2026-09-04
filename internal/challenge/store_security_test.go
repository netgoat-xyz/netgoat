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
		stores[i] = NewStore(WithSecret(testSecret), WithDifficulty(8, 8, 4))
	}
	runtime.Gosched()
	after := runtime.NumGoroutine()
	runtime.KeepAlive(stores)

	if delta := after - before; delta > 8 {
		t.Fatalf("creating stores added %d goroutines; want no per-store workers", delta)
	}
}

func TestStoreBoundsOutstandingChallenges(t *testing.T) {
	store := testStore(storeConfig{maxChallenges: 3})
	created := make([]*Challenge, 0, 4)
	for i := 0; i < 4; i++ {
		created = append(created, store.Create(terminated(fmt.Sprintf("session-%d", i))))
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
	store := testStore(storeConfig{maxVerified: 2})
	verifyBinding(t, store, "session-1")
	verifyBinding(t, store, "session-2")
	verifyBinding(t, store, "session-1")
	verifyBinding(t, store, "session-3")

	if got := verifiedCount(store); got != 2 {
		t.Fatalf("verified count = %d, want 2", got)
	}
	if store.IsVerified(terminated("session-2")) {
		t.Fatal("least-recently verified binding survived capacity eviction")
	}
	if !store.IsVerified(terminated("session-1")) || !store.IsVerified(terminated("session-3")) {
		t.Fatal("recently verified bindings were unexpectedly evicted")
	}
}

func TestStoreCleansExpirationOpportunistically(t *testing.T) {
	now := time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC)
	store := testStore(storeConfig{now: func() time.Time { return now }})
	pending := store.Create(terminated("session-1"))
	verifyBinding(t, store, "session-2")

	now = now.Add(defaultVerificationTTL)
	if _, ok := store.Get(pending.ID); ok {
		t.Fatal("expired challenge remained readable")
	}
	if store.IsVerified(terminated("session-2")) {
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
	store := testStore(storeConfig{maxFailedAttempts: 3})
	binding := terminated("session-1")
	challenge := store.Create(binding)

	for i := 0; i < 10; i++ {
		if store.Verify(terminated("session-other"), challenge.ID, "1") {
			t.Fatal("wrong binding verified challenge")
		}
	}
	if _, ok := store.Get(challenge.ID); !ok {
		t.Fatal("unrelated bindings consumed the challenge attempt budget")
	}

	for i := 0; i < 2; i++ {
		if store.Verify(binding, challenge.ID, "nope") {
			t.Fatal("wrong counter verified challenge")
		}
		if _, ok := store.Get(challenge.ID); !ok {
			t.Fatalf("challenge removed after only %d matching failures", i+1)
		}
	}
	if store.Verify(binding, challenge.ID, strings.Repeat("9", maxAnswerBytes+1)) {
		t.Fatal("oversized answer verified challenge")
	}
	if _, ok := store.Get(challenge.ID); ok {
		t.Fatal("challenge survived its failed-attempt budget")
	}
}

func TestStoreReturnsBoundedSnapshots(t *testing.T) {
	store := testStore(storeConfig{})
	binding := SessionBinding{
		SessionID:  strings.Repeat("s", maxStoredBindingBytes*4),
		Terminated: true,
		Subject:    strings.Repeat("u", maxStoredSubjectBytes*4),
	}
	challenge := store.Create(binding)
	originalID := challenge.ID
	originalMAC := challenge.MAC

	challenge.MAC = "tampered"
	challenge.ExpiresAt = time.Time{}
	challenge.Nonce = "tampered"
	retrieved, ok := store.Get(originalID)
	if !ok {
		t.Fatal("mutating Create result changed stored challenge")
	}
	if retrieved.MAC != originalMAC || retrieved.Nonce == "tampered" {
		t.Fatalf("stored challenge was mutated through Create result: %+v", retrieved)
	}

	retrieved.MAC = "tampered again"
	fresh, ok := store.Get(originalID)
	if !ok {
		t.Fatal("Get after mutation lost the puzzle")
	}
	counter := solveChallenge(t, fresh)
	if !store.Verify(binding, originalID, counter) {
		t.Fatal("mutating Get result changed stored puzzle or session match")
	}
}

func TestStoreConcurrentOperationsRemainBounded(t *testing.T) {
	const (
		maxChallenges = 64
		maxVerified   = 32
		workers       = 24
		iterations    = 150
	)
	store := testStore(storeConfig{
		maxChallenges:  maxChallenges,
		maxVerified:    maxVerified,
		baseDifficulty: 4,
		maxDifficulty:  4,
	})

	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for iteration := 0; iteration < iterations; iteration++ {
				binding := terminated(fmt.Sprintf("session-%d-%d", worker, iteration))
				challenge := store.Create(binding)
				if challenge == nil {
					continue
				}
				if iteration%3 == 0 {
					_, _ = store.Get(challenge.ID)
					_ = store.Verify(binding, challenge.ID, "1")
				} else {
					_ = store.Verify(binding, challenge.ID, solveQuiet(challenge))
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

func verifyBinding(t *testing.T, store *Store, sessionID string) {
	t.Helper()
	binding := terminated(sessionID)
	challenge := store.Create(binding)
	if challenge == nil {
		t.Fatal("Create returned nil")
	}
	if !store.Verify(binding, challenge.ID, solveChallenge(t, challenge)) {
		t.Fatalf("failed to verify session %q", sessionID)
	}
}

func solveQuiet(ch *Challenge) string {
	mac, ok := decodeMAC(ch.MAC)
	if !ok {
		return "0"
	}
	for counter := uint64(0); counter < 1<<20; counter++ {
		if leadingZeroBits(powDigest(mac, counter)) >= ch.Difficulty {
			return strconvFormat(counter)
		}
	}
	return "0"
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
