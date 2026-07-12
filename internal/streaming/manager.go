package streaming

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

type ConfigSnapshot struct {
	Version          int64                  `json:"version"`
	Timestamp        time.Time              `json:"timestamp"`
	Routes           map[string]RouteData   `json:"routes"`
	WAFRules         map[string]WAFRuleData `json:"waf_rules"`
	Users            []UserData             `json:"users"`
	UserDomains      []UserDomainData       `json:"user_domains"`
	ZeroTrustEnabled bool                   `json:"zero_trust_enabled"`
	AgentConfig      AgentConfigData        `json:"agent_config"`
}

type RouteTarget struct {
	URL         string `json:"url"`
	HealthCheck string `json:"health_check,omitempty"` // "http" or "tcp"
}

type RouteData struct {
	Type           string        `json:"type"`
	Target         string        `json:"target"` // legacy single target; use Targets when possible
	Targets        []RouteTarget `json:"targets,omitempty"`
	CertificatePEM string        `json:"certificate_pem,omitempty"`
	PrivateKeyPEM  string        `json:"private_key_pem,omitempty"`
}

// AllTargets returns configured upstreams, falling back to the legacy Target field.
func (r RouteData) AllTargets() []RouteTarget {
	if len(r.Targets) > 0 {
		return r.Targets
	}
	if r.Target != "" {
		return []RouteTarget{{URL: r.Target, HealthCheck: "http"}}
	}
	return nil
}

type WAFRuleData struct {
	Name       string `json:"name"`
	Expression string `json:"expression"`
	Action     string `json:"action"`
	Priority   int    `json:"priority"`
}

type UserData struct {
	ID               string `json:"id"`
	Username         string `json:"username"`
	Email            string `json:"email"`
	PasswordHash     string `json:"password_hash"`
	ZeroTrustEnabled bool   `json:"zero_trust_enabled"`
}

type UserDomainData struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	Domain    string `json:"domain"`
	TargetURL string `json:"target_url"`
	Active    bool   `json:"active"`
}

type AgentKeyMode string

const (
	AgentKeyIP     AgentKeyMode = "ip"
	AgentKeyHost   AgentKeyMode = "host"
	AgentKeyRoute  AgentKeyMode = "route"
	AgentKeyGlobal AgentKeyMode = "global"
)

type AgentCacheConfig struct {
	Enabled      bool `json:"enabled"`
	TTLSeconds   int  `json:"ttl_seconds"`
	MaxEntries   int  `json:"max_entries"`
	MaxBodyBytes int  `json:"max_body_bytes"`
}

type AgentRateLimitConfig struct {
	Enabled           bool         `json:"enabled"`
	RequestsPerMinute int          `json:"requests_per_minute"`
	Burst             int          `json:"burst"`
	Key               AgentKeyMode `json:"key"`
}

type AgentRequestQueueConfig struct {
	Enabled        bool `json:"enabled"`
	MaxConcurrent  int  `json:"max_concurrent"`
	MaxQueued      int  `json:"max_queued"`
	TimeoutSeconds int  `json:"timeout_seconds"`
}

type AgentBandwidthConfig struct {
	Enabled        bool         `json:"enabled"`
	BytesPerSecond int          `json:"bytes_per_second"`
	BurstBytes     int          `json:"burst_bytes"`
	Key            AgentKeyMode `json:"key"`
}

type AgentMetricsConfig struct {
	Enabled bool   `json:"enabled"`
	Path    string `json:"path"`
}

type AgentModelConfig struct {
	Enabled       bool    `json:"enabled"`
	Threshold     float64 `json:"threshold"`
	ModelPath     string  `json:"model_path"`
	ScalerPath    string  `json:"scaler_path"`
	PythonScript  string  `json:"python_script"`
	FeatureHeader string  `json:"feature_header"`
}

type AgentConfigData struct {
	Cache        AgentCacheConfig        `json:"cache"`
	RateLimit    AgentRateLimitConfig    `json:"rate_limit"`
	RequestQueue AgentRequestQueueConfig `json:"request_queue"`
	Bandwidth    AgentBandwidthConfig    `json:"bandwidth"`
	Metrics      AgentMetricsConfig      `json:"metrics"`
	KodaWaf      AgentModelConfig        `json:"koda_waf"`
	Koda2        AgentModelConfig        `json:"koda_2"`
	present      bool
}

func (c AgentConfigData) IsZero() bool {
	return !c.present && c == AgentConfigData{}
}

func (c *AgentConfigData) UnmarshalJSON(data []byte) error {
	type agentConfigData AgentConfigData
	var decoded agentConfigData
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*c = AgentConfigData(decoded)
	c.present = true
	return nil
}

type Message struct {
	Type      string          `json:"type"`
	Data      json.RawMessage `json:"data"`
	Version   int64           `json:"version"`
	Timestamp time.Time       `json:"timestamp"`
}

type Manager struct {
	mu           sync.RWMutex
	current      *ConfigSnapshot
	version      int64
	listeners    []chan *ConfigSnapshot
	ctx          context.Context
	cancel       context.CancelFunc
	recoveryFile string
	connected    bool
	lastError    error
}

func NewManager(recoveryFile string) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		current: &ConfigSnapshot{
			Version:     0,
			Timestamp:   time.Now(),
			Routes:      make(map[string]RouteData),
			WAFRules:    make(map[string]WAFRuleData),
			Users:       []UserData{},
			UserDomains: []UserDomainData{},
		},
		version:      0,
		listeners:    []chan *ConfigSnapshot{},
		ctx:          ctx,
		cancel:       cancel,
		recoveryFile: recoveryFile,
		connected:    false,
	}

	// Try to load snapshot from disk
	if err := m.loadFromDisk(); err != nil {
		log.Warn().Err(err).Str("file", recoveryFile).Msg("Could not load snapshot from disk, using defaults")
	} else {
		log.Info().Str("file", recoveryFile).Int64("version", m.current.Version).Msg("Loaded snapshot from disk")
	}

	return m
}

func (m *Manager) Subscribe() <-chan *ConfigSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	ch := make(chan *ConfigSnapshot, 10)
	ch <- m.current.copy()
	m.listeners = append(m.listeners, ch)
	return ch
}

// HandleMessage processes an incoming stream message and updates state.
func (m *Manager) HandleMessage(msg *Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if msg.Version <= m.version {
		log.Debug().Int64("msg_version", msg.Version).Int64("current_version", m.version).Msg("Ignoring stale message version")
		return nil // Don't error on stale versions, just skip
	}

	switch msg.Type {
	case "snapshot":
		var snap ConfigSnapshot
		if err := json.Unmarshal(msg.Data, &snap); err != nil {
			return err
		}
		m.current = &snap
		m.version = msg.Version
		m.connected = true
		m.lastError = nil

		log.Info().Int64("version", msg.Version).Int("routes", len(snap.Routes)).Int("waf_rules", len(snap.WAFRules)).Int("users", len(snap.Users)).Msg("Config snapshot updated")
	default:
		return errors.New("unknown message type: " + msg.Type)
	}

	m.current.Version = msg.Version
	m.current.Timestamp = msg.Timestamp

	// Save to disk for fault tolerance
	if err := m.saveToDisk(); err != nil {
		log.Error().Err(err).Msg("Failed to persist snapshot to disk")
	}

	// Broadcast to listeners
	snap := m.current.copy()
	for _, ch := range m.listeners {
		select {
		case ch <- snap:
		default:
		}
	}

	return nil
}

// GetSnapshot returns current config snapshot.
func (m *Manager) GetSnapshot() *ConfigSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current.copy()
}

// Close shuts down the manager.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cancel()
	for _, ch := range m.listeners {
		close(ch)
	}
	m.listeners = nil
}

func (s *ConfigSnapshot) copy() *ConfigSnapshot {
	routes := make(map[string]RouteData, len(s.Routes))
	for k, v := range s.Routes {
		targets := make([]RouteTarget, len(v.Targets))
		copy(targets, v.Targets)
		routes[k] = RouteData{
			Type:           v.Type,
			Target:         v.Target,
			Targets:        targets,
			CertificatePEM: v.CertificatePEM,
			PrivateKeyPEM:  v.PrivateKeyPEM,
		}
	}
	rules := make(map[string]WAFRuleData, len(s.WAFRules))
	for k, v := range s.WAFRules {
		rules[k] = v
	}
	users := make([]UserData, len(s.Users))
	copy(users, s.Users)
	userDomains := make([]UserDomainData, len(s.UserDomains))
	copy(userDomains, s.UserDomains)

	return &ConfigSnapshot{
		Version:          s.Version,
		Timestamp:        s.Timestamp,
		Routes:           routes,
		WAFRules:         rules,
		Users:            users,
		UserDomains:      userDomains,
		ZeroTrustEnabled: s.ZeroTrustEnabled,
		AgentConfig:      s.AgentConfig,
	}
}

// saveToDisk persists the current config snapshot to disk
func (m *Manager) saveToDisk() error {
	if m.recoveryFile == "" {
		return nil
	}

	data, err := json.MarshalIndent(m.current, "", "  ")
	if err != nil {
		return err
	}

	// Write to temp file first, then rename for atomicity
	tmpFile := m.recoveryFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return err
	}

	if err := os.Rename(tmpFile, m.recoveryFile); err != nil {
		os.Remove(tmpFile)
		return err
	}

	log.Debug().Str("file", m.recoveryFile).Int64("version", m.current.Version).Msg("Saved snapshot to disk")
	return nil
}

// loadFromDisk loads the config snapshot from disk
func (m *Manager) loadFromDisk() error {
	if m.recoveryFile == "" {
		return errors.New("no recovery file configured")
	}

	data, err := os.ReadFile(m.recoveryFile)
	if err != nil {
		return err
	}

	var snap ConfigSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}

	// Initialize maps if nil
	if snap.Routes == nil {
		snap.Routes = make(map[string]RouteData)
	}
	if snap.WAFRules == nil {
		snap.WAFRules = make(map[string]WAFRuleData)
	}
	if snap.Users == nil {
		snap.Users = []UserData{}
	}
	if snap.UserDomains == nil {
		snap.UserDomains = []UserDomainData{}
	}

	m.current = &snap
	m.version = snap.Version

	return nil
}

// SetConnectionStatus updates the connection status
func (m *Manager) SetConnectionStatus(connected bool, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = connected
	m.lastError = err
}

// GetConnectionStatus returns the current connection status
func (m *Manager) GetConnectionStatus() (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connected, m.lastError
}
