// Package policy defines the route-scoped traffic policy contract shared by
// local configuration, streamed snapshots, and the request path.
package policy

import (
	"fmt"
	"strings"
)

// KeyMode determines which request attribute identifies a bandwidth bucket.
// Route policies always namespace the resulting key by route, so an IP on one
// route cannot consume another route's quota.
type KeyMode string

const (
	KeyIP     KeyMode = "ip"
	KeyHost   KeyMode = "host"
	KeyRoute  KeyMode = "route"
	KeyGlobal KeyMode = "global"
)

// RoutePolicy contains optional overrides. A nil section inherits the process
// default. Individual nil fields inherit the corresponding default, while an
// explicit enabled: false disables that feature for the route.
type RoutePolicy struct {
	Cache     *CacheOverride     `json:"cache,omitempty" yaml:"cache,omitempty"`
	Bandwidth *BandwidthOverride `json:"bandwidth,omitempty" yaml:"bandwidth,omitempty"`
}

// Clone returns an independent copy that is safe to retain in immutable route
// snapshots.
func (p RoutePolicy) Clone() RoutePolicy {
	clone := RoutePolicy{}
	if p.Cache != nil {
		value := *p.Cache
		value.Enabled = cloneBool(p.Cache.Enabled)
		value.TTLSeconds = cloneInt(p.Cache.TTLSeconds)
		value.MaxEntries = cloneInt(p.Cache.MaxEntries)
		value.MaxBodyBytes = cloneInt(p.Cache.MaxBodyBytes)
		clone.Cache = &value
	}
	if p.Bandwidth != nil {
		value := *p.Bandwidth
		value.Enabled = cloneBool(p.Bandwidth.Enabled)
		value.BytesPerSecond = cloneInt(p.Bandwidth.BytesPerSecond)
		value.BurstBytes = cloneInt(p.Bandwidth.BurstBytes)
		value.Key = cloneKey(p.Bandwidth.Key)
		clone.Bandwidth = &value
	}
	return clone
}

func cloneBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneKey(value *KeyMode) *KeyMode {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

// CacheOverride applies limits only to a specific route.
type CacheOverride struct {
	Enabled      *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	TTLSeconds   *int  `json:"ttl_seconds,omitempty" yaml:"ttl_seconds,omitempty"`
	MaxEntries   *int  `json:"max_entries,omitempty" yaml:"max_entries,omitempty"`
	MaxBodyBytes *int  `json:"max_body_bytes,omitempty" yaml:"max_body_bytes,omitempty"`
}

// BandwidthOverride applies upload and download shaping only to a specific
// route.
type BandwidthOverride struct {
	Enabled        *bool    `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	BytesPerSecond *int     `json:"bytes_per_second,omitempty" yaml:"bytes_per_second,omitempty"`
	BurstBytes     *int     `json:"burst_bytes,omitempty" yaml:"burst_bytes,omitempty"`
	Key            *KeyMode `json:"key,omitempty" yaml:"key,omitempty"`
}

// CacheSettings is a fully resolved cache policy.
type CacheSettings struct {
	Enabled      bool
	TTLSeconds   int
	MaxEntries   int
	MaxBodyBytes int
}

// BandwidthSettings is a fully resolved bandwidth policy.
type BandwidthSettings struct {
	Enabled        bool
	BytesPerSecond int
	BurstBytes     int
	Key            KeyMode
}

func DefaultCacheSettings() CacheSettings {
	return CacheSettings{TTLSeconds: 60, MaxEntries: 1024, MaxBodyBytes: 1 << 20}
}

func DefaultBandwidthSettings() BandwidthSettings {
	return BandwidthSettings{BytesPerSecond: 1 << 20, BurstBytes: 1 << 20, Key: KeyIP}
}

// ResolveCache applies an override to the supplied global defaults.
func ResolveCache(global CacheSettings, override *CacheOverride) (CacheSettings, error) {
	resolved := normalizeCache(global)
	if override == nil {
		return resolved, nil
	}
	if override.Enabled != nil {
		resolved.Enabled = *override.Enabled
	}
	if override.TTLSeconds != nil {
		resolved.TTLSeconds = *override.TTLSeconds
	}
	if override.MaxEntries != nil {
		resolved.MaxEntries = *override.MaxEntries
	}
	if override.MaxBodyBytes != nil {
		resolved.MaxBodyBytes = *override.MaxBodyBytes
	}
	if err := validateCache(resolved); err != nil {
		return CacheSettings{}, err
	}
	return resolved, nil
}

// ResolveBandwidth applies an override to the supplied global defaults.
func ResolveBandwidth(global BandwidthSettings, override *BandwidthOverride) (BandwidthSettings, error) {
	resolved := normalizeBandwidth(global)
	if override == nil {
		return resolved, nil
	}
	if override.Enabled != nil {
		resolved.Enabled = *override.Enabled
	}
	if override.BytesPerSecond != nil {
		resolved.BytesPerSecond = *override.BytesPerSecond
	}
	if override.BurstBytes != nil {
		resolved.BurstBytes = *override.BurstBytes
	}
	if override.Key != nil {
		resolved.Key = *override.Key
	}
	if err := validateBandwidth(resolved); err != nil {
		return BandwidthSettings{}, err
	}
	return resolved, nil
}

// Validate ensures route-level input is safe to persist before it reaches the
// hot request path.
func (p RoutePolicy) Validate() error {
	if _, err := ResolveCache(DefaultCacheSettings(), p.Cache); err != nil {
		return fmt.Errorf("cache policy: %w", err)
	}
	if _, err := ResolveBandwidth(DefaultBandwidthSettings(), p.Bandwidth); err != nil {
		return fmt.Errorf("bandwidth policy: %w", err)
	}
	return nil
}

func normalizeCache(settings CacheSettings) CacheSettings {
	defaults := DefaultCacheSettings()
	if settings.TTLSeconds <= 0 {
		settings.TTLSeconds = defaults.TTLSeconds
	}
	if settings.MaxEntries <= 0 {
		settings.MaxEntries = defaults.MaxEntries
	}
	if settings.MaxBodyBytes <= 0 {
		settings.MaxBodyBytes = defaults.MaxBodyBytes
	}
	return settings
}

func normalizeBandwidth(settings BandwidthSettings) BandwidthSettings {
	defaults := DefaultBandwidthSettings()
	if settings.BytesPerSecond <= 0 {
		settings.BytesPerSecond = defaults.BytesPerSecond
	}
	if settings.BurstBytes <= 0 {
		settings.BurstBytes = settings.BytesPerSecond
	}
	if settings.Key == "" {
		settings.Key = defaults.Key
	}
	return settings
}

func validateCache(settings CacheSettings) error {
	if settings.TTLSeconds < 1 || settings.TTLSeconds > 86400 {
		return fmt.Errorf("ttl_seconds must be between 1 and 86400")
	}
	if settings.MaxEntries < 1 || settings.MaxEntries > 100000 {
		return fmt.Errorf("max_entries must be between 1 and 100000")
	}
	if settings.MaxBodyBytes < 1024 || settings.MaxBodyBytes > 100*1024*1024 {
		return fmt.Errorf("max_body_bytes must be between 1024 and 104857600")
	}
	return nil
}

func validateBandwidth(settings BandwidthSettings) error {
	if settings.BytesPerSecond < 1024 || settings.BytesPerSecond > 10*1024*1024*1024 {
		return fmt.Errorf("bytes_per_second must be between 1024 and 10737418240")
	}
	if settings.BurstBytes < 1024 || settings.BurstBytes > 10*1024*1024*1024 {
		return fmt.Errorf("burst_bytes must be between 1024 and 10737418240")
	}
	if !ValidKeyMode(settings.Key) {
		return fmt.Errorf("key must be ip, host, route, or global")
	}
	return nil
}

// ValidKeyMode reports whether a configuration key selector is supported.
func ValidKeyMode(value KeyMode) bool {
	switch KeyMode(strings.ToLower(strings.TrimSpace(string(value)))) {
	case KeyIP, KeyHost, KeyRoute, KeyGlobal:
		return true
	default:
		return false
	}
}
