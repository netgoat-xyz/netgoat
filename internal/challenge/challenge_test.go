package challenge

import (
	"strings"
	"testing"
	"time"
)

func TestNewStore(t *testing.T) {
	store := NewStore()

	if store == nil {
		t.Fatal("NewStore returned nil")
	}
	if store.challenges == nil {
		t.Error("challenges map should be initialized")
	}
	if store.verified == nil {
		t.Error("verified map should be initialized")
	}
}

func TestGenerateID(t *testing.T) {
	id1 := GenerateID()
	id2 := GenerateID()

	if id1 == "" {
		t.Error("GenerateID should not return empty string")
	}
	if id2 == "" {
		t.Error("GenerateID should not return empty string")
	}
	if id1 == id2 {
		t.Error("GenerateID should return unique IDs")
	}
	if len(id1) != 22 {
		t.Errorf("GenerateID length = %d, want 22", len(id1))
	}
}

func TestCalculateSuspicion(t *testing.T) {
	tests := []struct {
		name      string
		userAgent string
		ip        string
		wantMin   int
		wantMax   int
	}{
		{
			name:      "normal browser",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
			ip:        "192.168.1.1",
			wantMin:   0,
			wantMax:   0,
		},
		{
			name:      "bot in user agent",
			userAgent: "Googlebot/2.1 (+http://www.google.com/bot.html)",
			ip:        "192.168.1.1",
			wantMin:   30,
			wantMax:   45, // May also hit no-mozilla check
		},
		{
			name:      "crawler",
			userAgent: "Mozilla/5.0 compatible; Crawler/1.0",
			ip:        "192.168.1.1",
			wantMin:   30,
			wantMax:   30,
		},
		{
			name:      "python requests",
			userAgent: "python-requests/2.26.0",
			ip:        "192.168.1.1",
			wantMin:   30,
			wantMax:   45,
		},
		{
			name:      "curl",
			userAgent: "curl/7.68.0",
			ip:        "192.168.1.1",
			wantMin:   30,
			wantMax:   45,
		},
		{
			name:      "empty user agent",
			userAgent: "",
			ip:        "192.168.1.1",
			wantMin:   25,
			wantMax:   40,
		},
		{
			name:      "short user agent",
			userAgent: "Bot",
			ip:        "192.168.1.1",
			wantMin:   55,
			wantMax:   70, // Bot keyword + short + no mozilla/chrome/safari
		},
		{
			name:      "very long user agent",
			userAgent: strings.Repeat("A", 350),
			ip:        "192.168.1.1",
			wantMin:   10,
			wantMax:   25,
		},
		{
			name:      "many semicolons",
			userAgent: strings.Repeat(";", 15) + "test",
			ip:        "192.168.1.1",
			wantMin:   10,
			wantMax:   25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := CalculateSuspicion(tt.userAgent, tt.ip)

			if score < tt.wantMin {
				t.Errorf("Suspicion score = %d, want >= %d", score, tt.wantMin)
			}
			if score > tt.wantMax {
				t.Errorf("Suspicion score = %d, want <= %d", score, tt.wantMax)
			}
			if score < 0 || score > 100 {
				t.Errorf("Suspicion score = %d, should be in range [0, 100]", score)
			}
		})
	}
}

func TestDetermineChallengeType(t *testing.T) {
	tests := []struct {
		suspicion int
		want      ChallengeType
	}{
		{0, ChallengeNone},
		{10, ChallengeNone},
		{29, ChallengeNone},
		{30, ChallengeText},
		{40, ChallengeText},
		{59, ChallengeText},
		{60, ChallengeClick},
		{70, ChallengeClick},
		{79, ChallengeClick},
		{80, ChallengeSlider},
		{90, ChallengeSlider},
		{100, ChallengeSlider},
	}

	for _, tt := range tests {
		t.Run(string(tt.want), func(t *testing.T) {
			got := DetermineChallengeType(tt.suspicion)
			if got != tt.want {
				t.Errorf("DetermineChallengeType(%d) = %v, want %v", tt.suspicion, got, tt.want)
			}
		})
	}
}

func TestStoreCreate(t *testing.T) {
	store := NewStore()

	ch := store.Create("192.168.1.1", "TestBot/1.0", 50, ChallengeText)

	if ch == nil {
		t.Fatal("Create returned nil")
	}
	if ch.ID == "" {
		t.Error("Challenge ID should not be empty")
	}
	if ch.Type != ChallengeText {
		t.Errorf("Challenge type = %v, want %v", ch.Type, ChallengeText)
	}
	if ch.IP != "192.168.1.1" {
		t.Errorf("Challenge IP = %s, want 192.168.1.1", ch.IP)
	}
	if ch.UserAgent != "TestBot/1.0" {
		t.Errorf("Challenge UserAgent = %s, want TestBot/1.0", ch.UserAgent)
	}
	if ch.Suspicion != 50 {
		t.Errorf("Challenge Suspicion = %d, want 50", ch.Suspicion)
	}
	if ch.Answer == "" {
		t.Error("Challenge Answer should not be empty")
	}
	if ch.ExpiresAt.Before(time.Now()) {
		t.Error("Challenge should not be expired on creation")
	}
}

func TestStoreGet(t *testing.T) {
	store := NewStore()

	created := store.Create("192.168.1.1", "Bot", 60, ChallengeClick)

	// Test Get existing challenge
	retrieved, ok := store.Get(created.ID)
	if !ok {
		t.Error("Get should return true for existing challenge")
	}
	if retrieved == nil {
		t.Fatal("Get returned nil for existing challenge")
	}
	if retrieved.ID != created.ID {
		t.Errorf("Retrieved ID = %s, want %s", retrieved.ID, created.ID)
	}

	// Test Get non-existent challenge
	_, ok = store.Get("nonexistent")
	if ok {
		t.Error("Get should return false for non-existent challenge")
	}
}

func TestStoreVerify(t *testing.T) {
	store := NewStore()

	tests := []struct {
		name         string
		challengeType ChallengeType
		testIP       string
		testAnswer   string
		wantSuccess  bool
	}{
		{
			name:         "text challenge correct",
			challengeType: ChallengeText,
			testIP:       "192.168.1.1",
			testAnswer:   "", // Will be set to correct answer
			wantSuccess:  true,
		},
		{
			name:         "text challenge wrong answer",
			challengeType: ChallengeText,
			testIP:       "192.168.1.1",
			testAnswer:   "wronganswer",
			wantSuccess:  false,
		},
		{
			name:         "click challenge correct",
			challengeType: ChallengeClick,
			testIP:       "192.168.1.1",
			testAnswer:   "", // Will be set
			wantSuccess:  true,
		},
		{
			name:         "slider challenge correct",
			challengeType: ChallengeSlider,
			testIP:       "192.168.1.1",
			testAnswer:   "", // Will be set
			wantSuccess:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := store.Create(tt.testIP, "Bot", 60, tt.challengeType)

			answer := tt.testAnswer
			if answer == "" && tt.wantSuccess {
				answer = ch.Answer
			}

			verified := store.Verify(ch.ID, answer, tt.testIP)

			if verified != tt.wantSuccess {
				t.Errorf("Verify = %v, want %v", verified, tt.wantSuccess)
			}

			if tt.wantSuccess && store.IsVerified(tt.testIP) == false {
				t.Error("IP should be verified after successful challenge")
			}
		})
	}
}

func TestStoreVerifyWrongIP(t *testing.T) {
	store := NewStore()

	ch := store.Create("192.168.1.1", "Bot", 60, ChallengeText)

	// Try to verify with different IP
	verified := store.Verify(ch.ID, ch.Answer, "192.168.1.2")

	if verified {
		t.Error("Should not verify with different IP")
	}
}

func TestStoreVerifyExpired(t *testing.T) {
	store := NewStore()

	ch := store.Create("192.168.1.1", "Bot", 60, ChallengeText)

	// Manually expire the challenge
	ch.ExpiresAt = time.Now().Add(-1 * time.Hour)

	verified := store.Verify(ch.ID, ch.Answer, "192.168.1.1")

	if verified {
		t.Error("Should not verify expired challenge")
	}
}

func TestStoreIsVerified(t *testing.T) {
	store := NewStore()

	ip := "192.168.1.1"

	// Should not be verified initially
	if store.IsVerified(ip) {
		t.Error("IP should not be verified initially")
	}

	// Create and verify a challenge
	ch := store.Create(ip, "Bot", 60, ChallengeText)
	store.Verify(ch.ID, ch.Answer, ip)

	// Should be verified now
	if !store.IsVerified(ip) {
		t.Error("IP should be verified after successful challenge")
	}

	// Different IP should not be verified
	if store.IsVerified("192.168.1.2") {
		t.Error("Different IP should not be verified")
	}
}

func TestStoreVerifyCaseInsensitiveText(t *testing.T) {
	store := NewStore()

	ch := store.Create("192.168.1.1", "Bot", 40, ChallengeText)

	// Test case insensitive verification
	upperAnswer := strings.ToUpper(ch.Answer)
	verified := store.Verify(ch.ID, upperAnswer, "192.168.1.1")

	if !verified {
		t.Error("Text challenge should be case insensitive")
	}
}

func TestStoreVerifyWithWhitespace(t *testing.T) {
	store := NewStore()

	ch := store.Create("192.168.1.1", "Bot", 40, ChallengeText)

	// Test with whitespace
	answerWithSpace := "  " + ch.Answer + "  "
	verified := store.Verify(ch.ID, answerWithSpace, "192.168.1.1")

	if !verified {
		t.Error("Text challenge should trim whitespace")
	}
}

func TestGenerateTextAnswer(t *testing.T) {
	answers := make(map[string]bool)

	// Generate multiple answers
	for i := 0; i < 100; i++ {
		answer := generateTextAnswer()
		if answer == "" {
			t.Error("generateTextAnswer should not return empty string")
		}
		answers[answer] = true
	}

	// Should generate at least a few different words
	if len(answers) < 2 {
		t.Error("generateTextAnswer should generate varied words")
	}
}

func TestGenerateClickAnswer(t *testing.T) {
	answers := make(map[string]bool)

	for i := 0; i < 50; i++ {
		answer := generateClickAnswer()
		if answer == "" {
			t.Error("generateClickAnswer should not return empty string")
		}

		// Verify format (comma-separated numbers)
		parts := strings.Split(answer, ",")
		for _, part := range parts {
			if len(part) != 1 {
				t.Errorf("Click answer part should be single digit, got %s", part)
			}
			if part < "0" || part > "8" {
				t.Errorf("Click answer should be in range 0-8, got %s", part)
			}
		}

		answers[answer] = true
	}

	// Should generate varied answers
	if len(answers) < 5 {
		t.Error("generateClickAnswer should generate varied answers")
	}
}

func TestGenerateSliderAnswer(t *testing.T) {
	answers := make(map[string]bool)

	for i := 0; i < 50; i++ {
		answer := generateSliderAnswer()
		if answer == "" {
			t.Error("generateSliderAnswer should not return empty string")
		}
		if len(answer) != 1 {
			t.Errorf("Slider answer length = %d, want 1", len(answer))
		}

		answers[answer] = true
	}

	// Should have some variety
	if len(answers) < 2 {
		t.Error("generateSliderAnswer should generate varied answers")
	}
}

func TestChallengeTypes(t *testing.T) {
	types := []ChallengeType{
		ChallengeNone,
		ChallengeText,
		ChallengeClick,
		ChallengeSlider,
	}

	for _, ctype := range types {
		if string(ctype) == "" {
			t.Errorf("Challenge type %v should have string representation", ctype)
		}
	}
}

func TestStoreVerifyRemovesChallengeOnSuccess(t *testing.T) {
	store := NewStore()

	ch := store.Create("192.168.1.1", "Bot", 60, ChallengeText)

	// Verify successfully
	verified := store.Verify(ch.ID, ch.Answer, "192.168.1.1")
	if !verified {
		t.Fatal("Verification should succeed")
	}

	// Challenge should be removed
	_, ok := store.Get(ch.ID)
	if ok {
		t.Error("Challenge should be removed after successful verification")
	}
}

func TestStoreMultipleChallenges(t *testing.T) {
	store := NewStore()

	ch1 := store.Create("192.168.1.1", "Bot1", 40, ChallengeText)
	ch2 := store.Create("192.168.1.2", "Bot2", 60, ChallengeClick)
	ch3 := store.Create("192.168.1.3", "Bot3", 80, ChallengeSlider)

	// All should exist
	if _, ok := store.Get(ch1.ID); !ok {
		t.Error("ch1 should exist")
	}
	if _, ok := store.Get(ch2.ID); !ok {
		t.Error("ch2 should exist")
	}
	if _, ok := store.Get(ch3.ID); !ok {
		t.Error("ch3 should exist")
	}

	// Verify one
	store.Verify(ch2.ID, ch2.Answer, "192.168.1.2")

	// ch2 should be removed, others should remain
	if _, ok := store.Get(ch1.ID); !ok {
		t.Error("ch1 should still exist")
	}
	if _, ok := store.Get(ch2.ID); ok {
		t.Error("ch2 should be removed")
	}
	if _, ok := store.Get(ch3.ID); !ok {
		t.Error("ch3 should still exist")
	}
}

func TestStoreIsVerifiedExpiration(t *testing.T) {
	store := NewStore()

	ip := "192.168.1.1"

	// Manually add verified IP with old timestamp
	store.mu.Lock()
	store.verified[ip] = time.Now().Add(-2 * time.Hour)
	store.mu.Unlock()

	// Should not be verified (expired)
	if store.IsVerified(ip) {
		t.Error("Old verification should be expired")
	}
}

func TestChallengeExpiration(t *testing.T) {
	store := NewStore()

	ch := store.Create("192.168.1.1", "Bot", 60, ChallengeText)

	// Should have expiration set
	if ch.ExpiresAt.IsZero() {
		t.Error("ExpiresAt should be set")
	}

	// Should expire in the future
	if ch.ExpiresAt.Before(time.Now()) {
		t.Error("Challenge should not be expired on creation")
	}

	// Should expire within reasonable time (5 minutes)
	if ch.ExpiresAt.After(time.Now().Add(6 * time.Minute)) {
		t.Error("Challenge expiration seems too far in future")
	}
}