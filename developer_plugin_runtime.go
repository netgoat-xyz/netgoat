package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"
	"netgoat.xyz/agent/internal/config"
	"netgoat.xyz/agent/internal/developerplugins"
	"netgoat.xyz/agent/internal/middleware"
)

// developerPluginRuntime owns the fixed middleware set selected during agent
// startup. It intentionally has no update method: remote catalog selections
// are restart-only and can never hot-load code into a serving process.
type developerPluginRuntime struct {
	registry *middleware.Registry
	active   []developerplugins.ResolvedInstallation
}

func newDeveloperPluginRuntime(cfg *config.Config) (*developerPluginRuntime, error) {
	plugins := config.PluginConfig{}
	if cfg != nil {
		plugins = cfg.Plugins.Clone()
	}
	resolved, err := developerplugins.NewBuiltinRegistry().ResolveAll(plugins)
	if err != nil {
		return nil, err
	}
	if len(resolved) == 0 {
		return &developerPluginRuntime{}, nil
	}

	registry := middleware.NewRegistry(
		middleware.CapabilityRequestRead,
		middleware.CapabilityRouteRead,
		middleware.CapabilityResponseWrite,
	)
	for _, selection := range resolved {
		plugin, err := selection.Descriptor.Factory(selection.Config)
		if err != nil {
			return nil, fmt.Errorf("create compiled plugin %q: %w", selection.Descriptor.PluginID, err)
		}
		if err := selection.Descriptor.VerifyPlugin(plugin); err != nil {
			return nil, err
		}
		if err := registry.Register(plugin, selection.Descriptor.GrantedCapabilities...); err != nil {
			return nil, fmt.Errorf("register compiled plugin %q: %w", selection.Descriptor.PluginID, err)
		}
	}
	if err := registry.Start(context.Background()); err != nil {
		return nil, fmt.Errorf("start compiled developer plugins: %w", err)
	}
	return &developerPluginRuntime{registry: registry, active: resolved}, nil
}

// Evaluate invokes the immutable startup registry after the proxy has resolved
// trusted client and route metadata. Returning false means the middleware
// wrote a blocking/error response, so callers must stop processing the request.
func (r *developerPluginRuntime) Evaluate(writer http.ResponseWriter, request *http.Request, metadata middleware.RequestMetadata) bool {
	if r == nil || r.registry == nil {
		return true
	}
	nextCalled := false
	r.registry.Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		nextCalled = true
	})).ServeHTTP(writer, middleware.WithRequestMetadata(request, metadata))
	return nextCalled
}

func (r *developerPluginRuntime) Close(ctx context.Context) error {
	if r == nil || r.registry == nil {
		return nil
	}
	return r.registry.Close(ctx)
}

func (r *developerPluginRuntime) ActiveCount() int {
	if r == nil {
		return 0
	}
	return len(r.active)
}

func validateDeveloperPluginSelection(plugins config.PluginConfig) error {
	_, err := developerplugins.NewBuiltinRegistry().ResolveAll(plugins)
	return err
}

func closeDeveloperPluginRuntime(runtime *developerPluginRuntime) {
	if runtime == nil {
		return
	}
	if err := runtime.Close(context.Background()); err != nil && !errors.Is(err, context.Canceled) {
		log.Error().Err(err).Msg("Graceful developer plugin shutdown failed")
	}
}
