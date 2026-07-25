// Package developerplugins resolves catalog selections to middleware that is
// already compiled into the NetGoat agent binary.
//
// It is deliberately not a package manager. The registry never downloads a
// release, opens an artifact, evaluates source, or registers a factory from
// configuration. A selection is accepted only when every immutable catalog
// claim matches a descriptor that the binary registered during startup.
package developerplugins

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"netgoat.xyz/agent/internal/config"
	"netgoat.xyz/agent/internal/middleware"
)

const (
	// MaxInstallations caps the startup work and prevents a control-plane
	// snapshot from turning a catalog selection into unbounded memory use.
	MaxInstallations = 64
	// MaxIdentifierBytes bounds every immutable catalog identity field.
	MaxIdentifierBytes = 128
	// MaxConfigBytes bounds the JSON object passed to one compiled factory.
	MaxConfigBytes = 16 << 10
	// MaxTotalConfigBytes bounds all factory configuration in a selection.
	MaxTotalConfigBytes = 256 << 10
)

// Descriptor is immutable metadata for a trusted middleware implementation
// compiled into this binary. SHA256 is the catalog release fingerprint baked
// into the build; it is not a digest of a remotely fetched artifact.
type Descriptor struct {
	PluginID            string
	FactoryID           string
	Version             string
	SHA256              string
	APIVersion          string
	GrantedCapabilities []middleware.Capability
	Factory             middleware.Factory
}

// ResolvedInstallation is a validated local binding between a catalog
// selection and a compiled descriptor. Config is canonical JSON for the
// descriptor's factory and never contains a loaded artifact or source unit.
type ResolvedInstallation struct {
	Descriptor Descriptor
	Config     json.RawMessage
}

// Registry is an immutable lookup table built from the agent's compiled-in
// descriptors. It is safe for concurrent resolution after construction.
type Registry struct {
	descriptors map[string]Descriptor
}

// NewRegistry constructs a descriptor registry. It rejects duplicate release
// identities and descriptors that do not specify a compiled factory.
func NewRegistry(descriptors ...Descriptor) (*Registry, error) {
	registry := &Registry{descriptors: make(map[string]Descriptor, len(descriptors))}
	for _, descriptor := range descriptors {
		normalized, err := normalizeDescriptor(descriptor)
		if err != nil {
			return nil, err
		}
		key := descriptorKey(normalized.PluginID, normalized.FactoryID, normalized.Version, normalized.SHA256, normalized.APIVersion)
		if _, exists := registry.descriptors[key]; exists {
			return nil, fmt.Errorf("duplicate compiled plugin descriptor %q", normalized.PluginID)
		}
		registry.descriptors[key] = normalized
	}
	return registry, nil
}

// Resolve verifies one catalog installation against immutable metadata baked
// into this binary. It never makes network or filesystem calls.
func (r *Registry) Resolve(installation config.PluginInstallation) (ResolvedInstallation, error) {
	if r == nil {
		return ResolvedInstallation{}, errors.New("developer plugin registry is nil")
	}
	normalized, factoryConfig, err := normalizeInstallation(installation)
	if err != nil {
		return ResolvedInstallation{}, err
	}
	key := descriptorKey(normalized.PluginID, normalized.FactoryID, normalized.Version, normalized.SHA256, normalized.APIVersion)
	descriptor, exists := r.descriptors[key]
	if !exists {
		return ResolvedInstallation{}, fmt.Errorf("plugin %q release is not compiled into this agent", normalized.PluginID)
	}
	grantedCapabilities, err := normalizeStringCapabilities(normalized.GrantedCapabilities)
	if err != nil {
		return ResolvedInstallation{}, err
	}
	if !sameCapabilities(descriptor.GrantedCapabilities, grantedCapabilities) {
		return ResolvedInstallation{}, fmt.Errorf("plugin %q capability grant does not exactly match the compiled descriptor", normalized.PluginID)
	}
	return ResolvedInstallation{Descriptor: descriptor, Config: factoryConfig}, nil
}

// VerifyPlugin proves that a factory returned the middleware implementation
// described by a resolved catalog release. Calling code must perform this
// check before registering the plugin with the request path.
func (d Descriptor) VerifyPlugin(plugin middleware.Plugin) error {
	if plugin == nil {
		return fmt.Errorf("compiled plugin %q factory returned nil", d.PluginID)
	}
	manifest := plugin.Manifest()
	if manifest.Name != d.PluginID {
		return fmt.Errorf("compiled plugin %q returned manifest name %q", d.PluginID, manifest.Name)
	}
	if manifest.Version != d.Version {
		return fmt.Errorf("compiled plugin %q returned manifest version %q", d.PluginID, manifest.Version)
	}
	if manifest.APIVersion != d.APIVersion {
		return fmt.Errorf("compiled plugin %q returned API version %q", d.PluginID, manifest.APIVersion)
	}
	capabilities, err := normalizeCapabilities(manifest.Capabilities)
	if err != nil {
		return fmt.Errorf("compiled plugin %q manifest: %w", d.PluginID, err)
	}
	if !sameCapabilities(d.GrantedCapabilities, capabilities) {
		return fmt.Errorf("compiled plugin %q manifest capabilities do not match its descriptor", d.PluginID)
	}
	return nil
}

// ResolveAll validates a bounded restart-time selection. A duplicate plugin
// package is rejected even if it names a different release so the startup
// middleware order remains deterministic and auditable.
func (r *Registry) ResolveAll(plugins config.PluginConfig) ([]ResolvedInstallation, error) {
	if len(plugins.Installations) > MaxInstallations {
		return nil, fmt.Errorf("plugin installations exceed maximum of %d", MaxInstallations)
	}
	resolved := make([]ResolvedInstallation, 0, len(plugins.Installations))
	seenPluginIDs := make(map[string]struct{}, len(plugins.Installations))
	totalConfigBytes := 0
	for index, installation := range plugins.Installations {
		item, err := r.Resolve(installation)
		if err != nil {
			return nil, fmt.Errorf("plugin installation %d: %w", index, err)
		}
		if _, duplicate := seenPluginIDs[item.Descriptor.PluginID]; duplicate {
			return nil, fmt.Errorf("plugin %q is selected more than once", item.Descriptor.PluginID)
		}
		seenPluginIDs[item.Descriptor.PluginID] = struct{}{}
		totalConfigBytes += len(item.Config)
		if totalConfigBytes > MaxTotalConfigBytes {
			return nil, fmt.Errorf("plugin configuration exceeds total maximum of %d bytes", MaxTotalConfigBytes)
		}
		resolved = append(resolved, item)
	}
	return resolved, nil
}

func normalizeDescriptor(descriptor Descriptor) (Descriptor, error) {
	if err := validateIdentity(descriptor.PluginID, "plugin_id"); err != nil {
		return Descriptor{}, err
	}
	if err := validateIdentity(descriptor.FactoryID, "factory_id"); err != nil {
		return Descriptor{}, err
	}
	if err := validateIdentity(descriptor.Version, "version"); err != nil {
		return Descriptor{}, err
	}
	if err := validateSHA256(descriptor.SHA256); err != nil {
		return Descriptor{}, err
	}
	if err := validateIdentity(descriptor.APIVersion, "api_version"); err != nil {
		return Descriptor{}, err
	}
	if descriptor.Factory == nil {
		return Descriptor{}, fmt.Errorf("compiled plugin %q has no factory", descriptor.PluginID)
	}
	capabilities, err := normalizeCapabilities(descriptor.GrantedCapabilities)
	if err != nil {
		return Descriptor{}, fmt.Errorf("compiled plugin %q: %w", descriptor.PluginID, err)
	}
	descriptor.GrantedCapabilities = capabilities
	return descriptor, nil
}

func normalizeInstallation(installation config.PluginInstallation) (config.PluginInstallation, json.RawMessage, error) {
	if err := validateIdentity(installation.PluginID, "plugin_id"); err != nil {
		return config.PluginInstallation{}, nil, err
	}
	if err := validateIdentity(installation.FactoryID, "factory_id"); err != nil {
		return config.PluginInstallation{}, nil, err
	}
	if err := validateIdentity(installation.Version, "version"); err != nil {
		return config.PluginInstallation{}, nil, err
	}
	if err := validateSHA256(installation.SHA256); err != nil {
		return config.PluginInstallation{}, nil, err
	}
	if err := validateIdentity(installation.APIVersion, "api_version"); err != nil {
		return config.PluginInstallation{}, nil, err
	}
	capabilities, err := normalizeStringCapabilities(installation.GrantedCapabilities)
	if err != nil {
		return config.PluginInstallation{}, nil, err
	}
	factoryConfig, err := canonicalConfig(installation.Config)
	if err != nil {
		return config.PluginInstallation{}, nil, err
	}
	installation.GrantedCapabilities = make([]string, len(capabilities))
	for index, capability := range capabilities {
		installation.GrantedCapabilities[index] = string(capability)
	}
	return installation, factoryConfig, nil
}

func validateIdentity(value, field string) error {
	if value == "" {
		return fmt.Errorf("plugin %s is required", field)
	}
	if len(value) > MaxIdentifierBytes {
		return fmt.Errorf("plugin %s exceeds %d bytes", field, MaxIdentifierBytes)
	}
	if strings.TrimSpace(value) != value {
		return fmt.Errorf("plugin %s must not have leading or trailing whitespace", field)
	}
	for _, runeValue := range value {
		if unicode.IsControl(runeValue) {
			return fmt.Errorf("plugin %s contains a control character", field)
		}
	}
	return nil
}

func validateSHA256(value string) error {
	if len(value) != 64 {
		return errors.New("plugin sha256 must be 64 lowercase hexadecimal characters")
	}
	for _, runeValue := range value {
		if !(runeValue >= '0' && runeValue <= '9' || runeValue >= 'a' && runeValue <= 'f') {
			return errors.New("plugin sha256 must be 64 lowercase hexadecimal characters")
		}
	}
	return nil
}

func normalizeStringCapabilities(capabilities []string) ([]middleware.Capability, error) {
	converted := make([]middleware.Capability, len(capabilities))
	for index, capability := range capabilities {
		converted[index] = middleware.Capability(capability)
	}
	return normalizeCapabilities(converted)
}

func normalizeCapabilities(capabilities []middleware.Capability) ([]middleware.Capability, error) {
	known := map[middleware.Capability]struct{}{
		middleware.CapabilityRequestRead:   {},
		middleware.CapabilityRouteRead:     {},
		middleware.CapabilityResponseWrite: {},
	}
	normalized := append([]middleware.Capability(nil), capabilities...)
	seen := make(map[middleware.Capability]struct{}, len(normalized))
	for _, capability := range normalized {
		if strings.TrimSpace(string(capability)) != string(capability) {
			return nil, fmt.Errorf("plugin capability %q has surrounding whitespace", capability)
		}
		if _, exists := known[capability]; !exists {
			return nil, fmt.Errorf("plugin capability %q is not supported", capability)
		}
		if _, duplicate := seen[capability]; duplicate {
			return nil, fmt.Errorf("plugin capability %q is declared more than once", capability)
		}
		seen[capability] = struct{}{}
	}
	sort.Slice(normalized, func(left, right int) bool { return normalized[left] < normalized[right] })
	return normalized, nil
}

func sameCapabilities(left, right []middleware.Capability) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func canonicalConfig(input map[string]any) (json.RawMessage, error) {
	if input == nil {
		return json.RawMessage(`{}`), nil
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("plugin config must be a JSON object: %w", err)
	}
	if len(encoded) > MaxConfigBytes {
		return nil, fmt.Errorf("plugin config exceeds maximum of %d bytes", MaxConfigBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var verified map[string]any
	if err := decoder.Decode(&verified); err != nil {
		return nil, fmt.Errorf("plugin config must be a JSON object: %w", err)
	}
	if verified == nil {
		return nil, errors.New("plugin config must be a JSON object")
	}
	return json.RawMessage(encoded), nil
}

func descriptorKey(pluginID, factoryID, version, sha256, apiVersion string) string {
	return strings.Join([]string{pluginID, factoryID, version, sha256, apiVersion}, "\x00")
}

// BuiltinNoopPluginID identifies a harmless compiled-in reference plugin. It
// proves the catalog path end-to-end without granting request data or changing
// proxy behavior. It is disabled unless explicitly selected in configuration.
const BuiltinNoopPluginID = "netgoat/noop"

const BuiltinNoopFactoryID = "builtin.noop"
const BuiltinNoopVersion = "1.0.0"

// BuiltinNoopSHA256 is the immutable compiled descriptor fingerprint for the
// reference plugin. Control planes must send this descriptor digest in
// selection.sha256; a manifest/audit digest is deliberately not interchangeable.
const BuiltinNoopSHA256 = "22d6f9898bf456265c2549c38ef185c11de1da1eeee944bb8e97470ed438d2c0"

// NewBuiltinRegistry returns the fixed registry shipped by this build. Future
// trusted extensions must add their descriptor and factory in Go source.
func NewBuiltinRegistry() *Registry {
	registry, err := NewRegistry(Descriptor{
		PluginID:   BuiltinNoopPluginID,
		FactoryID:  BuiltinNoopFactoryID,
		Version:    BuiltinNoopVersion,
		SHA256:     BuiltinNoopSHA256,
		APIVersion: middleware.APIVersion,
		Factory:    newBuiltinNoop,
	})
	if err != nil {
		panic(fmt.Sprintf("invalid built-in plugin registry: %v", err))
	}
	return registry
}

func newBuiltinNoop(raw json.RawMessage) (middleware.Plugin, error) {
	var options map[string]any
	if err := json.Unmarshal(raw, &options); err != nil {
		return nil, fmt.Errorf("decode built-in noop config: %w", err)
	}
	if len(options) != 0 {
		return nil, errors.New("built-in noop plugin does not accept configuration")
	}
	return builtinNoop{}, nil
}

type builtinNoop struct{}

func (builtinNoop) Manifest() middleware.Manifest {
	return middleware.Manifest{
		Name:       BuiltinNoopPluginID,
		Version:    BuiltinNoopVersion,
		APIVersion: middleware.APIVersion,
	}
}

func (builtinNoop) Start(context.Context) error { return nil }

func (builtinNoop) Handle(context.Context, middleware.Request) (middleware.Decision, error) {
	return middleware.Decision{Action: middleware.ActionAllow}, nil
}

func (builtinNoop) Stop(context.Context) error { return nil }
