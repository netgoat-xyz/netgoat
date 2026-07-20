package challenge

import (
	"container/list"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"sync"
	"time"
)

type ChallengeType string

const (
	ChallengeNone   ChallengeType = "none"
	ChallengeText   ChallengeType = "text"
	ChallengeClick  ChallengeType = "click"
	ChallengeSlider ChallengeType = "slider"
)

const (
	challengeIDBytes            = 16
	challengeIDLength           = 22
	defaultMaxChallenges        = 4096
	defaultMaxVerified          = 4096
	defaultMaxFailedAttempts    = 5
	defaultChallengeTTL         = 5 * time.Minute
	defaultVerificationTTL      = time.Hour
	maxAnswerBytes              = 256
	maxStoredBindingBytes       = 256
	maxStoredChallengeTypeBytes = 32
	maxStoredUserAgentBytes     = 512
)

type Challenge struct {
	ID        string
	Type      ChallengeType
	Answer    string
	CreatedAt time.Time
	ExpiresAt time.Time
	IP        string
	UserAgent string
	Suspicion int
}

type bindingKey [sha256.Size]byte

type challengeEntry struct {
	challenge      Challenge
	binding        bindingKey
	failedAttempts int
	order          *list.Element
}

type verifiedEntry struct {
	binding   bindingKey
	expiresAt time.Time
	order     *list.Element
}

type storeConfig struct {
	now               func() time.Time
	maxChallenges     int
	maxVerified       int
	maxFailedAttempts int
	challengeTTL      time.Duration
	verificationTTL   time.Duration
}

type Store struct {
	mu sync.RWMutex

	// The order lists keep the oldest expiration at the front. Inserts and
	// verification refreshes happen under mu so cleanup stays amortized O(1).
	challenges     map[string]*challengeEntry
	challengeOrder list.List
	verified       map[bindingKey]*verifiedEntry
	verifiedOrder  list.List

	now               func() time.Time
	maxChallenges     int
	maxVerified       int
	maxFailedAttempts int
	challengeTTL      time.Duration
	verificationTTL   time.Duration
}

func NewStore() *Store {
	// Cleanup is deliberately request-driven: Store has no Close method, so a
	// background ticker here would leak one goroutine for every Store instance.
	return newStore(storeConfig{})
}

func newStore(config storeConfig) *Store {
	if config.now == nil {
		config.now = time.Now
	}
	if config.maxChallenges <= 0 {
		config.maxChallenges = defaultMaxChallenges
	}
	if config.maxVerified <= 0 {
		config.maxVerified = defaultMaxVerified
	}
	if config.maxFailedAttempts <= 0 {
		config.maxFailedAttempts = defaultMaxFailedAttempts
	}
	if config.challengeTTL <= 0 {
		config.challengeTTL = defaultChallengeTTL
	}
	if config.verificationTTL <= 0 {
		config.verificationTTL = defaultVerificationTTL
	}

	return &Store{
		challenges:        make(map[string]*challengeEntry),
		verified:          make(map[bindingKey]*verifiedEntry),
		now:               config.now,
		maxChallenges:     config.maxChallenges,
		maxVerified:       config.maxVerified,
		maxFailedAttempts: config.maxFailedAttempts,
		challengeTTL:      config.challengeTTL,
		verificationTTL:   config.verificationTTL,
	}
}

func GenerateID() string {
	var b [challengeIDBytes]byte
	readRandom(b[:])
	return base64.RawURLEncoding.EncodeToString(b[:])
}

func CalculateSuspicion(userAgent, ip string) int {
	score := 0
	ua := strings.ToLower(userAgent)

	bots := []string{"bot", "crawler", "spider", "scraper", "curl", "wget", "python", "go-http-client", "java"}
	for _, b := range bots {
		if strings.Contains(ua, b) {
			score += 30
			break
		}
	}

	if userAgent == "" || len(userAgent) < 10 {
		score += 25
	}
	if !strings.Contains(ua, "mozilla") && !strings.Contains(ua, "chrome") && !strings.Contains(ua, "safari") {
		score += 15
	}
	if strings.Count(ua, ";") > 10 || len(ua) > 300 {
		score += 10
	}

	if score > 100 {
		score = 100
	}
	return score
}

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

func (s *Store) Create(ip, userAgent string, suspicion int, challengeType ChallengeType) *Challenge {
	challenge := Challenge{
		ID:        GenerateID(),
		Type:      ChallengeType(boundedClone(string(challengeType), maxStoredChallengeTypeBytes)),
		IP:        boundedClone(ip, maxStoredBindingBytes),
		UserAgent: boundedClone(userAgent, maxStoredUserAgentBytes),
		Suspicion: suspicion,
	}

	switch challengeType {
	case ChallengeText:
		challenge.Answer = generateTextAnswer()
	case ChallengeClick:
		challenge.Answer = generateClickAnswer()
	case ChallengeSlider:
		challenge.Answer = generateSliderAnswer()
	}

	entry := &challengeEntry{
		challenge: challenge,
		binding:   makeBindingKey(ip),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	entry.challenge.CreatedAt = now
	entry.challenge.ExpiresAt = now.Add(s.challengeTTL)
	s.cleanupExpiredLocked(now)
	for len(s.challenges) >= s.maxChallenges {
		s.removeOldestChallengeLocked()
	}
	for {
		if _, exists := s.challenges[entry.challenge.ID]; !exists {
			break
		}
		entry.challenge.ID = GenerateID()
	}
	entry.order = s.challengeOrder.PushBack(entry)
	s.challenges[entry.challenge.ID] = entry

	return cloneChallenge(entry.challenge)
}

func (s *Store) Get(id string) (*Challenge, bool) {
	if len(id) != challengeIDLength {
		return nil, false
	}
	s.mu.RLock()
	now := s.now()
	entry, ok := s.challenges[id]
	if ok && now.Before(entry.challenge.ExpiresAt) {
		challenge := cloneChallenge(entry.challenge)
		s.mu.RUnlock()
		return challenge, true
	}
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredLocked(now)
	entry, ok = s.challenges[id]
	if !ok {
		return nil, false
	}
	return cloneChallenge(entry.challenge), true
}

func (s *Store) Verify(id, answer, ip string) bool {
	binding := makeBindingKey(ip)

	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.cleanupExpiredLocked(now)

	if len(id) != challengeIDLength {
		return false
	}
	entry, ok := s.challenges[id]
	if !ok || entry.binding != binding {
		return false
	}
	if len(answer) > maxAnswerBytes {
		s.recordFailureLocked(entry)
		return false
	}

	correct := false
	switch entry.challenge.Type {
	case ChallengeText:
		correct = strings.EqualFold(strings.TrimSpace(answer), strings.TrimSpace(entry.challenge.Answer))
	case ChallengeClick, ChallengeSlider:
		correct = answer == entry.challenge.Answer
	}

	if !correct {
		s.recordFailureLocked(entry)
		return false
	}

	s.removeChallengeLocked(entry)
	s.markVerifiedLocked(binding, now)
	return true
}

func (s *Store) IsVerified(ip string) bool {
	binding := makeBindingKey(ip)
	s.mu.RLock()
	now := s.now()
	entry, ok := s.verified[binding]
	if ok && now.Before(entry.expiresAt) {
		s.mu.RUnlock()
		return true
	}
	s.mu.RUnlock()
	if !ok {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredLocked(now)

	entry, ok = s.verified[binding]
	return ok && now.Before(entry.expiresAt)
}

func (s *Store) recordFailureLocked(entry *challengeEntry) {
	entry.failedAttempts++
	if entry.failedAttempts >= s.maxFailedAttempts {
		s.removeChallengeLocked(entry)
	}
}

func (s *Store) markVerifiedLocked(binding bindingKey, now time.Time) {
	expiresAt := now.Add(s.verificationTTL)
	if entry, ok := s.verified[binding]; ok {
		entry.expiresAt = expiresAt
		s.verifiedOrder.MoveToBack(entry.order)
		return
	}

	for len(s.verified) >= s.maxVerified {
		s.removeOldestVerifiedLocked()
	}
	entry := &verifiedEntry{binding: binding, expiresAt: expiresAt}
	entry.order = s.verifiedOrder.PushBack(entry)
	s.verified[binding] = entry
}

func (s *Store) cleanupExpiredLocked(now time.Time) {
	for {
		oldest := s.challengeOrder.Front()
		if oldest == nil {
			break
		}
		entry := oldest.Value.(*challengeEntry)
		if now.Before(entry.challenge.ExpiresAt) {
			break
		}
		s.removeChallengeLocked(entry)
	}

	for {
		oldest := s.verifiedOrder.Front()
		if oldest == nil {
			break
		}
		entry := oldest.Value.(*verifiedEntry)
		if now.Before(entry.expiresAt) {
			break
		}
		s.removeVerifiedLocked(entry)
	}
}

func (s *Store) removeOldestChallengeLocked() {
	if oldest := s.challengeOrder.Front(); oldest != nil {
		s.removeChallengeLocked(oldest.Value.(*challengeEntry))
	}
}

func (s *Store) removeChallengeLocked(entry *challengeEntry) {
	current, ok := s.challenges[entry.challenge.ID]
	if !ok || current != entry {
		return
	}
	delete(s.challenges, entry.challenge.ID)
	if entry.order != nil {
		s.challengeOrder.Remove(entry.order)
		entry.order = nil
	}
}

func (s *Store) removeOldestVerifiedLocked() {
	if oldest := s.verifiedOrder.Front(); oldest != nil {
		s.removeVerifiedLocked(oldest.Value.(*verifiedEntry))
	}
}

func (s *Store) removeVerifiedLocked(entry *verifiedEntry) {
	current, ok := s.verified[entry.binding]
	if !ok || current != entry {
		return
	}
	delete(s.verified, entry.binding)
	if entry.order != nil {
		s.verifiedOrder.Remove(entry.order)
		entry.order = nil
	}
}

func makeBindingKey(binding string) bindingKey {
	return sha256.Sum256([]byte(binding))
}

func cloneChallenge(challenge Challenge) *Challenge {
	clone := challenge
	return &clone
}

func boundedClone(value string, limit int) string {
	if len(value) > limit {
		value = value[:limit]
	}
	return strings.Clone(value)
}

func readRandom(buffer []byte) {
	if _, err := rand.Read(buffer); err != nil {
		panic("challenge: secure randomness unavailable: " + err.Error())
	}
}

func generateTextAnswer() string {
	words := []string{"sunrise", "mountain", "ocean", "forest", "desert", "river", "cloud", "thunder"}
	var b [1]byte
	readRandom(b[:])
	return words[int(b[0])%len(words)]
}

func generateClickAnswer() string {
	var b [3]byte
	readRandom(b[:])
	var seen [9]bool
	for _, value := range b {
		idx := int(value) % len(seen)
		seen[idx] = true
	}
	result := make([]string, 0, len(b))
	for idx, selected := range seen {
		if selected {
			result = append(result, string(rune('0'+idx)))
		}
	}
	return strings.Join(result, ",")
}

func generateSliderAnswer() string {
	var b [1]byte
	readRandom(b[:])
	pos := 20 + (int(b[0]) % 60)
	return string(rune('0' + pos/10))
}
