// Package middleware provides the versioned extension contract used by
// trusted, in-process NetGoat middleware.
//
// It intentionally does not load Go plugins from disk: Go's plugin ABI is not
// portable and loading untrusted native code cannot be sandboxed. Applications
// register compiled-in factories explicitly and grant only the capabilities a
// plugin declares. Use dynamicrules for isolated administrator-authored rules.
package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// APIVersion identifies the stable v1 middleware contract.
const APIVersion = "netgoat.dev/middleware/v1"

// Capability is an explicitly granted permission a plugin may request.
type Capability string

const (
	CapabilityRequestRead   Capability = "request.read"
	CapabilityRouteRead     Capability = "route.read"
	CapabilityResponseWrite Capability = "response.write"
)

// Manifest describes a plugin and the API version it was compiled for.
type Manifest struct {
	Name         string
	Version      string
	APIVersion   string
	Capabilities []Capability
}

// Request is an immutable snapshot of the request data made available to a
// plugin. Header values are cloned before each invocation so a plugin cannot
// mutate the request forwarded to the upstream.
type Request struct {
	Method   string
	Host     string
	Path     string
	RawQuery string
	Header   http.Header
	ClientIP string
	RouteKey string
}

// RequestMetadata is supplied by the host application after trusted client-IP
// resolution and route selection. It is intentionally separate from the raw
// request so plugins receive it only when the corresponding capability is
// granted.
type RequestMetadata struct {
	ClientIP string
	RouteKey string
}

type requestMetadataKey struct{}

// WithRequestMetadata makes host-derived metadata available to middleware for
// one request. It returns a copy and never mutates the original request.
func WithRequestMetadata(request *http.Request, metadata RequestMetadata) *http.Request {
	if request == nil {
		return nil
	}
	return request.WithContext(context.WithValue(request.Context(), requestMetadataKey{}, metadata))
}

// Action is the result of evaluating a request in a plugin.
type Action string

const (
	ActionAllow Action = "allow"
	ActionBlock Action = "block"
)

// Decision is returned by Plugin.Handle. A blocking plugin can optionally
// provide a status, headers, and body when it holds response.write.
type Decision struct {
	Action  Action
	Reason  string
	Status  int
	Headers http.Header
	Body    []byte
}

// Plugin is the stable v1 lifecycle and request contract. Plugins are called
// in registration order. Start is called before serving, and Stop is called in
// reverse order during graceful shutdown.
type Plugin interface {
	Manifest() Manifest
	Start(context.Context) error
	Handle(context.Context, Request) (Decision, error)
	Stop(context.Context) error
}

// Factory creates a compiled-in plugin from a JSON configuration blob.
type Factory func(json.RawMessage) (Plugin, error)

// Spec is the serializable configuration used by Loader and Registry.
type Spec struct {
	Name                string          `json:"name" yaml:"name"`
	Enabled             bool            `json:"enabled" yaml:"enabled"`
	Config              json.RawMessage `json:"config" yaml:"config"`
	GrantedCapabilities []Capability    `json:"granted_capabilities" yaml:"granted_capabilities"`
}

// Loader holds the compiled-in plugin factories. It is safe for concurrent
// configuration loading, but registration is deliberately restricted to setup
// time to keep the extension surface auditable.
type Loader struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

func NewLoader() *Loader {
	return &Loader{factories: make(map[string]Factory)}
}

// RegisterFactory makes a trusted, compiled-in factory available by name.
func (l *Loader) RegisterFactory(name string, factory Factory) error {
	if l == nil {
		return errors.New("middleware loader is nil")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("middleware factory name is required")
	}
	if factory == nil {
		return fmt.Errorf("middleware factory %q is nil", name)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, exists := l.factories[name]; exists {
		return fmt.Errorf("middleware factory %q is already registered", name)
	}
	l.factories[name] = factory
	return nil
}

// Load constructs one plugin. Disabled specs are ignored by callers rather
// than returning a partially initialized plugin.
func (l *Loader) Load(spec Spec) (Plugin, error) {
	if l == nil {
		return nil, errors.New("middleware loader is nil")
	}
	name := strings.TrimSpace(spec.Name)
	l.mu.RLock()
	factory := l.factories[name]
	l.mu.RUnlock()
	if factory == nil {
		return nil, fmt.Errorf("middleware factory %q is not registered", name)
	}
	plugin, err := factory(spec.Config)
	if err != nil {
		return nil, fmt.Errorf("load middleware %q: %w", name, err)
	}
	if plugin == nil {
		return nil, fmt.Errorf("load middleware %q: factory returned nil", name)
	}
	return plugin, nil
}

type registeredPlugin struct {
	plugin   Plugin
	manifest Manifest
	grants   map[Capability]struct{}
}

type pluginSet struct {
	items []registeredPlugin
}

// Registry validates capability grants, owns plugin lifecycle, and turns a
// registered set into an HTTP handler. Its failure behavior is deliberately
// fail-closed: an extension error or panic returns HTTP 500 without invoking
// later plugins or the upstream handler.
type Registry struct {
	mu      sync.Mutex
	allowed map[Capability]struct{}
	plugins atomic.Pointer[pluginSet]
	started bool
	closed  bool
}

// NewRegistry creates a registry that may grant only capabilities listed in
// allowed. Supplying an empty list makes the registry safe by default.
func NewRegistry(allowed ...Capability) *Registry {
	permissions := make(map[Capability]struct{}, len(allowed))
	for _, capability := range allowed {
		permissions[capability] = struct{}{}
	}
	registry := &Registry{allowed: permissions}
	registry.plugins.Store(&pluginSet{})
	return registry
}

// Register adds a plugin before Start. Every capability declared in its
// manifest must also be explicitly granted and permitted by this registry.
func (r *Registry) Register(plugin Plugin, grants ...Capability) error {
	if r == nil {
		return errors.New("middleware registry is nil")
	}
	if plugin == nil {
		return errors.New("middleware plugin is nil")
	}
	manifest, err := validateManifest(plugin.Manifest())
	if err != nil {
		return err
	}
	granted := make(map[Capability]struct{}, len(grants))
	for _, capability := range grants {
		if _, known := validCapabilities()[capability]; !known {
			return fmt.Errorf("middleware %q grants unknown capability %q", manifest.Name, capability)
		}
		if _, allowed := r.allowed[capability]; !allowed {
			return fmt.Errorf("middleware %q is not allowed capability %q", manifest.Name, capability)
		}
		granted[capability] = struct{}{}
	}
	for _, capability := range manifest.Capabilities {
		if _, allowed := granted[capability]; !allowed {
			return fmt.Errorf("middleware %q requires ungranted capability %q", manifest.Name, capability)
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started {
		return errors.New("cannot register middleware after start")
	}
	if r.closed {
		return errors.New("cannot register middleware after close")
	}
	current := r.plugins.Load()
	next := &pluginSet{items: make([]registeredPlugin, 0, len(current.items)+1)}
	next.items = append(next.items, current.items...)
	for _, existing := range current.items {
		if existing.manifest.Name == manifest.Name {
			return fmt.Errorf("middleware %q is already registered", manifest.Name)
		}
	}
	next.items = append(next.items, registeredPlugin{plugin: plugin, manifest: manifest, grants: granted})
	r.plugins.Store(next)
	return nil
}

// LoadAndRegister loads and validates every enabled spec before startup. No
// plugin is started as part of this method; callers should discard the
// registry when configuration loading returns an error.
func (r *Registry) LoadAndRegister(loader *Loader, specs []Spec) error {
	for _, spec := range specs {
		if !spec.Enabled {
			continue
		}
		plugin, err := loader.Load(spec)
		if err != nil {
			return err
		}
		if err := r.Register(plugin, spec.GrantedCapabilities...); err != nil {
			return err
		}
	}
	return nil
}

// Start initializes plugins in registration order. A failed start shuts down
// already-started plugins before returning the error.
func (r *Registry) Start(ctx context.Context) error {
	if r == nil {
		return errors.New("middleware registry is nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return errors.New("cannot start closed middleware registry")
	}
	if r.started {
		return nil
	}
	items := r.plugins.Load().items
	started := make([]registeredPlugin, 0, len(items))
	for _, entry := range items {
		if err := callStart(ctx, entry); err != nil {
			for index := len(started) - 1; index >= 0; index-- {
				_ = callStop(ctx, started[index])
			}
			return err
		}
		started = append(started, entry)
	}
	r.started = true
	return nil
}

// Close stops initialized plugins in reverse order. It is safe to call more
// than once and returns joined shutdown errors when several plugins fail.
func (r *Registry) Close(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	if !r.started {
		return nil
	}
	items := r.plugins.Load().items
	errs := make([]error, 0)
	for index := len(items) - 1; index >= 0; index-- {
		if err := callStop(ctx, items[index]); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Handler evaluates the plugins before passing a request to next. It is safe
// to use after Start and remains race-free while requests are in flight.
func (r *Registry) Handler(next http.Handler) http.Handler {
	if next == nil {
		next = http.NotFoundHandler()
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if r == nil {
			next.ServeHTTP(writer, request)
			return
		}
		for _, entry := range r.plugins.Load().items {
			decision, err := callHandle(request.Context(), entry, requestView(request, entry.grants))
			if err != nil {
				http.Error(writer, "middleware unavailable", http.StatusInternalServerError)
				return
			}
			if err := applyDecision(writer, entry, decision); err != nil {
				http.Error(writer, "middleware rejected response", http.StatusInternalServerError)
				return
			}
			if decision.Action == ActionBlock {
				return
			}
		}
		next.ServeHTTP(writer, request)
	})
}

func validateManifest(manifest Manifest) (Manifest, error) {
	manifest.Name = strings.TrimSpace(manifest.Name)
	manifest.Version = strings.TrimSpace(manifest.Version)
	manifest.APIVersion = strings.TrimSpace(manifest.APIVersion)
	if manifest.Name == "" {
		return Manifest{}, errors.New("middleware manifest name is required")
	}
	if manifest.Version == "" {
		return Manifest{}, fmt.Errorf("middleware %q manifest version is required", manifest.Name)
	}
	if manifest.APIVersion != APIVersion {
		return Manifest{}, fmt.Errorf("middleware %q requires API %q, supported API is %q", manifest.Name, manifest.APIVersion, APIVersion)
	}
	known := validCapabilities()
	seen := make(map[Capability]struct{}, len(manifest.Capabilities))
	for _, capability := range manifest.Capabilities {
		if _, valid := known[capability]; !valid {
			return Manifest{}, fmt.Errorf("middleware %q declares unknown capability %q", manifest.Name, capability)
		}
		if _, duplicate := seen[capability]; duplicate {
			return Manifest{}, fmt.Errorf("middleware %q declares capability %q twice", manifest.Name, capability)
		}
		seen[capability] = struct{}{}
	}
	manifest.Capabilities = append([]Capability(nil), manifest.Capabilities...)
	sort.Slice(manifest.Capabilities, func(left, right int) bool { return manifest.Capabilities[left] < manifest.Capabilities[right] })
	return manifest, nil
}

func validCapabilities() map[Capability]struct{} {
	return map[Capability]struct{}{
		CapabilityRequestRead:   {},
		CapabilityRouteRead:     {},
		CapabilityResponseWrite: {},
	}
}

func requestView(request *http.Request, grants map[Capability]struct{}) Request {
	metadata, _ := request.Context().Value(requestMetadataKey{}).(RequestMetadata)
	view := Request{}
	if _, allowed := grants[CapabilityRequestRead]; allowed {
		view.Method = request.Method
		view.Host = request.Host
		view.Path = request.URL.Path
		view.RawQuery = request.URL.RawQuery
		view.Header = request.Header.Clone()
		view.ClientIP = metadata.ClientIP
	}
	if _, allowed := grants[CapabilityRouteRead]; allowed {
		view.RouteKey = metadata.RouteKey
	}
	return view
}

func applyDecision(writer http.ResponseWriter, entry registeredPlugin, decision Decision) error {
	if decision.Action == "" {
		decision.Action = ActionAllow
	}
	if decision.Action != ActionAllow && decision.Action != ActionBlock {
		return fmt.Errorf("middleware %q returned invalid action %q", entry.manifest.Name, decision.Action)
	}
	if decision.Action != ActionBlock {
		if decision.Status != 0 || len(decision.Headers) != 0 || len(decision.Body) != 0 {
			return fmt.Errorf("middleware %q supplied a response without blocking", entry.manifest.Name)
		}
		return nil
	}
	if decision.Status != 0 || len(decision.Headers) != 0 || len(decision.Body) != 0 {
		if _, granted := entry.grants[CapabilityResponseWrite]; !granted {
			return fmt.Errorf("middleware %q wrote a response without response.write", entry.manifest.Name)
		}
	}
	if decision.Status == 0 {
		decision.Status = http.StatusForbidden
	}
	if decision.Status < 400 || decision.Status > 599 {
		return fmt.Errorf("middleware %q returned invalid block status %d", entry.manifest.Name, decision.Status)
	}
	for key, values := range decision.Headers {
		writer.Header().Del(key)
		for _, value := range values {
			writer.Header().Add(key, value)
		}
	}
	writer.WriteHeader(decision.Status)
	if len(decision.Body) > 0 {
		_, _ = writer.Write(decision.Body)
	}
	return nil
}

func callStart(ctx context.Context, entry registeredPlugin) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("middleware %q panicked during start: %v", entry.manifest.Name, recovered)
		}
	}()
	if err := entry.plugin.Start(ctx); err != nil {
		return fmt.Errorf("start middleware %q: %w", entry.manifest.Name, err)
	}
	return nil
}

func callHandle(ctx context.Context, entry registeredPlugin, request Request) (decision Decision, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("middleware %q panicked while handling a request: %v", entry.manifest.Name, recovered)
		}
	}()
	decision, err = entry.plugin.Handle(ctx, request)
	if err != nil {
		return Decision{}, fmt.Errorf("run middleware %q: %w", entry.manifest.Name, err)
	}
	return decision, nil
}

func callStop(ctx context.Context, entry registeredPlugin) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("middleware %q panicked during stop: %v", entry.manifest.Name, recovered)
		}
	}()
	if err := entry.plugin.Stop(ctx); err != nil {
		return fmt.Errorf("stop middleware %q: %w", entry.manifest.Name, err)
	}
	return nil
}
