package streaming

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

// ConfigSnapshot holds the current routes and WAF rules state.
type ConfigSnapshot struct {
	Version   int64                  `json:"version"`
	Timestamp time.Time              `json:"timestamp"`
	Routes    map[string]string      `json:"routes"`
	WAFRules  map[string]WAFRuleData `json:"waf_rules"`
}

// WAFRuleData represents a WAF rule.
type WAFRuleData struct {
	Name       string `json:"name"`
	Expression string `json:"expression"`
	Action     string `json:"action"`
	Priority   int    `json:"priority"`
}

// Message represents an update message from the API.
type Message struct {
	Type      string          `json:"type"`
	Data      json.RawMessage `json:"data"`
	Version   int64           `json:"version"`
	Timestamp time.Time       `json:"timestamp"`
}

// Manager handles config streaming and state management.
type Manager struct {
	mu        sync.RWMutex
	current   *ConfigSnapshot
	version   int64
	listeners []chan *ConfigSnapshot
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewManager creates a streaming manager.
func NewManager(recoveryFile string) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		current: &ConfigSnapshot{
			Version:   0,
			Timestamp: time.Now(),
			Routes:    make(map[string]string),
			WAFRules:  make(map[string]WAFRuleData),
		},
		version:   0,
		listeners: []chan *ConfigSnapshot{},
		ctx:       ctx,
		cancel:    cancel,
	}
	return m
}

// Subscribe returns a channel that receives config updates.
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
		return errors.New("stale message version")
	}

	switch msg.Type {
	case "snapshot":
		var snap ConfigSnapshot
		if err := json.Unmarshal(msg.Data, &snap); err != nil {
			return err
		}
		m.current = &snap
		m.version = msg.Version
	default:
		return errors.New("unknown message type: " + msg.Type)
	}

	m.current.Version = msg.Version
	m.current.Timestamp = msg.Timestamp

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
	routes := make(map[string]string, len(s.Routes))
	for k, v := range s.Routes {
		routes[k] = v
	}
	rules := make(map[string]WAFRuleData, len(s.WAFRules))
	for k, v := range s.WAFRules {
		rules[k] = v
	}
	return &ConfigSnapshot{
		Version:   s.Version,
		Timestamp: s.Timestamp,
		Routes:    routes,
		WAFRules:  rules,
	}
}
