package challenge

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"sync"
	"time"
)

// ChallengeType defines the type of challenge to present
type ChallengeType string

const (
	ChallengeNone   ChallengeType = "none"
	ChallengeText   ChallengeType = "text"
	ChallengeClick  ChallengeType = "click"
	ChallengeSlider ChallengeType = "slider"
)

// Challenge represents an active challenge
type Challenge struct {
	ID        string
	Type      ChallengeType
	Answer    string // Expected answer
	CreatedAt time.Time
	ExpiresAt time.Time
	IP        string
	UserAgent string
	Suspicion int // 0-100 suspicion score
}

// Store manages active challenges
type Store struct {
	mu         sync.RWMutex
	challenges map[string]*Challenge
	verified   map[string]time.Time // IP -> last verified time
}

func NewStore() *Store {
	s := &Store{
		challenges: make(map[string]*Challenge),
		verified:   make(map[string]time.Time),
	}
	go s.cleanup()
	return s
}

// cleanup removes expired challenges every minute
func (s *Store) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for id, ch := range s.challenges {
			if now.After(ch.ExpiresAt) {
				delete(s.challenges, id)
			}
		}
		// Clean verified entries older than 1 hour
		for ip, t := range s.verified {
			if now.Sub(t) > 1*time.Hour {
				delete(s.verified, ip)
			}
		}
		s.mu.Unlock()
	}
}

// GenerateID creates a unique challenge ID
func GenerateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)[:22]
}

// CalculateSuspicion scores the request based on user-agent and behavior
func CalculateSuspicion(userAgent, ip string) int {
	score := 0
	ua := strings.ToLower(userAgent)

	// Known bot signatures
	bots := []string{"bot", "crawler", "spider", "scraper", "curl", "wget", "python", "go-http-client", "java"}
	for _, b := range bots {
		if strings.Contains(ua, b) {
			score += 30
			break
		}
	}

	// Suspicious patterns
	if userAgent == "" || len(userAgent) < 10 {
		score += 25
	}
	if !strings.Contains(ua, "mozilla") && !strings.Contains(ua, "chrome") && !strings.Contains(ua, "safari") {
		score += 15
	}
	if strings.Count(ua, ";") > 10 || len(ua) > 300 {
		score += 10
	}

	// Cap at 100
	if score > 100 {
		score = 100
	}
	return score
}

// DetermineChallengeType picks challenge based on suspicion level
func DetermineChallengeType(suspicion int) ChallengeType {
	if suspicion < 30 {
		return ChallengeNone
	} else if suspicion < 60 {
		return ChallengeText
	} else if suspicion < 80 {
		return ChallengeClick
	}
	return ChallengeSlider
}

// Create generates and stores a new challenge
func (s *Store) Create(ip, userAgent string, suspicion int, challengeType ChallengeType) *Challenge {
	ch := &Challenge{
		ID:        GenerateID(),
		Type:      challengeType,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(5 * time.Minute),
		IP:        ip,
		UserAgent: userAgent,
		Suspicion: suspicion,
	}

	// Generate challenge-specific answer
	switch challengeType {
	case ChallengeText:
		ch.Answer = generateTextAnswer()
	case ChallengeClick:
		ch.Answer = generateClickAnswer()
	case ChallengeSlider:
		ch.Answer = generateSliderAnswer()
	}

	s.mu.Lock()
	s.challenges[ch.ID] = ch
	s.mu.Unlock()

	return ch
}

// Get retrieves a challenge by ID
func (s *Store) Get(id string) (*Challenge, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ch, ok := s.challenges[id]
	return ch, ok
}

// Verify checks if the answer is correct and marks IP as verified
func (s *Store) Verify(id, answer, ip string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch, ok := s.challenges[id]
	if !ok || time.Now().After(ch.ExpiresAt) {
		return false
	}

	// Check IP match
	if ch.IP != ip {
		return false
	}

	// Verify answer
	correct := false
	switch ch.Type {
	case ChallengeText:
		correct = strings.EqualFold(strings.TrimSpace(answer), strings.TrimSpace(ch.Answer))
	case ChallengeClick, ChallengeSlider:
		correct = answer == ch.Answer
	}

	if correct {
		delete(s.challenges, id)
		s.verified[ip] = time.Now()
		return true
	}

	return false
}

// IsVerified checks if an IP recently passed a challenge
func (s *Store) IsVerified(ip string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.verified[ip]
	if !ok {
		return false
	}
	return time.Now().Sub(t) < 1*time.Hour
}

// Helper functions to generate challenge answers
func generateTextAnswer() string {
	words := []string{"sunrise", "mountain", "ocean", "forest", "desert", "river", "cloud", "thunder"}
	b := make([]byte, 1)
	rand.Read(b)
	return words[int(b[0])%len(words)]
}

func generateClickAnswer() string {
	// Returns a comma-separated list of correct box indices (0-8)
	b := make([]byte, 3)
	rand.Read(b)
	indices := []int{int(b[0]) % 9, int(b[1]) % 9, int(b[2]) % 9}
	// Make unique
	seen := make(map[int]bool)
	result := []string{}
	for _, idx := range indices {
		if !seen[idx] {
			seen[idx] = true
			result = append(result, string(rune('0'+idx)))
		}
	}
	return strings.Join(result, ",")
}

func generateSliderAnswer() string {
	// Returns expected slider position (0-100)
	b := make([]byte, 1)
	rand.Read(b)
	pos := 20 + (int(b[0]) % 60) // Range 20-80
	return string(rune('0' + pos/10))
}
