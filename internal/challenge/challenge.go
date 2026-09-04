package challenge

import (
	"container/list"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"
	"sync"
	"time"
)

type ChallengeType string

const (
	ChallengeNone ChallengeType = "none"
	ChallengePoW  ChallengeType = "pow"
)

const (
	challengeIDBytes         = 16
	challengeIDLength        = 22
	defaultMaxChallenges     = 4096
	defaultMaxVerified       = 4096
	defaultMaxFailedAttempts = 5
	defaultChallengeTTL      = 2 * time.Minute
	defaultVerificationTTL   = 15 * time.Minute
	maxAnswerBytes           = 256
	maxStoredBindingBytes    = 256
	maxStoredSubjectBytes    = 128
)

// Challenge is a session-bound proof-of-work puzzle. Agents solve the Wire
// JSON without a DOM; browsers use the same JSON in a small inline worker.
type Challenge struct {
	ID         string
	Type       ChallengeType
	Nonce      string
	Difficulty int
	Expires    int64
	MAC        string
	CreatedAt  time.Time
	ExpiresAt  time.Time
}

// Wire is the agent-facing PoW document.
type Wire struct {
	Nonce      string `json:"nonce"`
	Difficulty int    `json:"difficulty"`
	Expires    int64  `json:"expires"`
	MAC        string `json:"mac"`
}

func (c *Challenge) Wire() Wire {
	if c == nil {
		return Wire{}
	}
	return Wire{
		Nonce:      c.Nonce,
		Difficulty: c.Difficulty,
		Expires:    c.Expires,
		MAC:        c.MAC,
	}
}

type bindingKey [sha256.Size]byte

type challengeEntry struct {
	challenge      Challenge
	binding        bindingKey
	sessionID      string
	expiresUnix    int64
	difficulty     int
	commitment     []byte
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
	secret            []byte
	botAuth           *BotAuthVerifier
	maxChallenges     int
	maxVerified       int
	maxFailedAttempts int
	challengeTTL      time.Duration
	verificationTTL   time.Duration
	baseDifficulty    int
	maxDifficulty     int
	mismatchBump      int
}

// Option configures a Store.
type Option func(*storeConfig)

func WithSecret(secret []byte) Option {
	return func(config *storeConfig) {
		config.secret = append([]byte(nil), secret...)
	}
}

func WithBotAuth(verifier *BotAuthVerifier) Option {
	return func(config *storeConfig) {
		config.botAuth = verifier
	}
}

func WithNow(now func() time.Time) Option {
	return func(config *storeConfig) {
		config.now = now
	}
}

func WithTTLs(challengeTTL, verificationTTL time.Duration) Option {
	return func(config *storeConfig) {
		config.challengeTTL = challengeTTL
		config.verificationTTL = verificationTTL
	}
}

func WithDifficulty(base, max, mismatchBump int) Option {
	return func(config *storeConfig) {
		config.baseDifficulty = base
		config.maxDifficulty = max
		config.mismatchBump = mismatchBump
	}
}

type Store struct {
	mu sync.RWMutex

	// The order lists keep the oldest expiration at the front. Inserts and
	// verification refreshes happen under mu so cleanup stays amortized O(1).
	challenges     map[string]*challengeEntry
	challengeOrder list.List
	verified       map[bindingKey]*verifiedEntry
	verifiedOrder  list.List
	recentIssues   []time.Time

	now               func() time.Time
	secret            []byte
	botAuth           *BotAuthVerifier
	maxChallenges     int
	maxVerified       int
	maxFailedAttempts int
	challengeTTL      time.Duration
	verificationTTL   time.Duration
	baseDifficulty    int
	maxDifficulty     int
	mismatchBump      int
}

func NewStore(opts ...Option) *Store {
	config := storeConfig{}
	for _, opt := range opts {
		opt(&config)
	}
	return newStore(config)
}

func newStore(config storeConfig) *Store {
	if config.now == nil {
		config.now = time.Now
	}
	if len(config.secret) == 0 {
		secret, _ := ResolveSecret()
		config.secret = secret
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
	if config.baseDifficulty <= 0 {
		config.baseDifficulty = defaultBaseDifficulty
	}
	if config.maxDifficulty <= 0 {
		config.maxDifficulty = defaultMaxDifficulty
	}
	if config.maxDifficulty < config.baseDifficulty {
		config.maxDifficulty = config.baseDifficulty
	}
	if config.mismatchBump <= 0 {
		config.mismatchBump = defaultMismatchBump
	}

	return &Store{
		challenges:        make(map[string]*challengeEntry),
		verified:          make(map[bindingKey]*verifiedEntry),
		now:               config.now,
		secret:            append([]byte(nil), config.secret...),
		botAuth:           config.botAuth,
		maxChallenges:     config.maxChallenges,
		maxVerified:       config.maxVerified,
		maxFailedAttempts: config.maxFailedAttempts,
		challengeTTL:      config.challengeTTL,
		verificationTTL:   config.verificationTTL,
		baseDifficulty:    config.baseDifficulty,
		maxDifficulty:     config.maxDifficulty,
		mismatchBump:      config.mismatchBump,
	}
}

func GenerateID() string {
	var b [challengeIDBytes]byte
	readRandom(b[:])
	return base64.RawURLEncoding.EncodeToString(b[:])
}

// SkipRequest is the pinned Web Bot Auth skip lane. Unpinned, unknown, or
// missing signatures return false and must fail open to PoW with no
// suspicion bump.
func (s *Store) SkipRequest(r *http.Request) bool {
	if s == nil || s.botAuth == nil {
		return false
	}
	return s.botAuth.Skip(r)
}

// Create issues a session-bound PoW puzzle. It returns nil when TLS was not
// terminated here (pass-through / HTTP-only / Cloudflare in front).
func (s *Store) Create(binding SessionBinding) *Challenge {
	if !binding.usable() {
		return nil
	}

	entry := &challengeEntry{
		binding:   makeBindingKey(binding.Key()),
		sessionID: boundedClone(binding.SessionID, maxStoredBindingBytes),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.cleanupExpiredLocked(now)
	for len(s.challenges) >= s.maxChallenges {
		s.removeOldestChallengeLocked()
	}

	difficulty := s.difficulty(binding)
	expiresAt := now.Add(s.challengeTTL)
	expiresUnix := expiresAt.Unix()
	nonce := GenerateID()
	for {
		if _, exists := s.challenges[nonce]; !exists {
			break
		}
		nonce = GenerateID()
	}
	commitment := computeCommitment(s.secret, entry.sessionID, nonce, difficulty, expiresUnix)

	entry.challenge = Challenge{
		ID:         nonce,
		Type:       ChallengePoW,
		Nonce:      nonce,
		Difficulty: difficulty,
		Expires:    expiresUnix,
		MAC:        encodeMAC(commitment),
		CreatedAt:  now,
		ExpiresAt:  expiresAt,
	}
	entry.expiresUnix = expiresUnix
	entry.difficulty = difficulty
	entry.commitment = commitment
	entry.order = s.challengeOrder.PushBack(entry)
	s.challenges[nonce] = entry
	s.noteIssueLocked(now)

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

// Verify checks a PoW solution bound to the caller's session. The nonce is
// burned on success or after maxFailedAttempts. Binding is never an IP.
func (s *Store) Verify(binding SessionBinding, nonce, counter string) bool {
	if !binding.usable() {
		return false
	}
	key := makeBindingKey(binding.Key())

	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.cleanupExpiredLocked(now)

	if len(nonce) != challengeIDLength {
		return false
	}
	entry, ok := s.challenges[nonce]
	if !ok || entry.binding != key {
		return false
	}
	if now.Unix() >= entry.expiresUnix {
		s.removeChallengeLocked(entry)
		return false
	}
	if len(counter) > maxAnswerBytes {
		s.recordFailureLocked(entry)
		return false
	}
	parsed, ok := parseCounter(counter)
	if !ok {
		s.recordFailureLocked(entry)
		return false
	}

	// Recompute the commitment with the stored expires_unix. A truncated
	// HMAC that omitted expiry cannot match, which is the Altcha replay hole.
	recomputed := computeCommitment(s.secret, entry.sessionID, entry.challenge.Nonce, entry.difficulty, entry.expiresUnix)
	if !hmac.Equal(recomputed, entry.commitment) {
		s.removeChallengeLocked(entry)
		return false
	}
	digest := powDigest(recomputed, parsed)
	if leadingZeroBits(digest) < entry.difficulty {
		s.recordFailureLocked(entry)
		return false
	}

	s.removeChallengeLocked(entry)
	s.markVerifiedLocked(key, now)
	return true
}

func (s *Store) IsVerified(binding SessionBinding) bool {
	if !binding.usable() {
		return false
	}
	key := makeBindingKey(binding.Key())
	s.mu.RLock()
	now := s.now()
	entry, ok := s.verified[key]
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

	entry, ok = s.verified[key]
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
		if now.Before(entry.challenge.ExpiresAt) && now.Unix() < entry.expiresUnix {
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
