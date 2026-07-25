package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"netgoat.xyz/agent/internal/config"
	"netgoat.xyz/agent/internal/developerplugins"
	"netgoat.xyz/agent/internal/middleware"
)

func TestDeveloperPluginRuntimeActivatesExactBuiltinSelection(t *testing.T) {
	runtime, err := newDeveloperPluginRuntime(&config.Config{Plugins: config.PluginConfig{
		Installations: []config.PluginInstallation{builtinNoopInstallation()},
	}})
	if err != nil {
		t.Fatalf("newDeveloperPluginRuntime(): %v", err)
	}
	defer closeDeveloperPluginRuntime(runtime)
	if runtime.ActiveCount() != 1 {
		t.Fatalf("active plugin count = %d, want 1", runtime.ActiveCount())
	}

	request := httptest.NewRequest(http.MethodGet, "http://api.example.test/private", nil)
	response := httptest.NewRecorder()
	if allowed := runtime.Evaluate(response, request, middleware.RequestMetadata{
		ClientIP: "203.0.113.42",
		RouteKey: "api.example.test",
	}); !allowed {
		t.Fatal("harmless built-in plugin unexpectedly stopped the request")
	}
	if response.Code != http.StatusOK {
		t.Fatalf("response status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestDeveloperPluginRuntimeRejectsMismatchedSelection(t *testing.T) {
	installation := builtinNoopInstallation()
	installation.SHA256 = strings.Repeat("a", 64)
	if _, err := newDeveloperPluginRuntime(&config.Config{Plugins: config.PluginConfig{
		Installations: []config.PluginInstallation{installation},
	}}); err == nil {
		t.Fatal("newDeveloperPluginRuntime() accepted an uncompiled catalog release")
	}
}

func TestApplyPluginConfigCopiesRestartTimeSelection(t *testing.T) {
	cfg := &config.Config{}
	plugins := config.PluginConfig{Installations: []config.PluginInstallation{{
		PluginID: "example.test/metadata",
		Config:   map[string]any{"nested": map[string]any{"mode": "observe"}},
	}}}
	applyPluginConfigToConfig(cfg, plugins)
	plugins.Installations[0].Config["nested"].(map[string]any)["mode"] = "mutated"
	if got := cfg.Plugins.Installations[0].Config["nested"].(map[string]any)["mode"]; got != "observe" {
		t.Fatalf("restart-time plugin selection was not cloned: %+v", cfg.Plugins)
	}
}

func builtinNoopInstallation() config.PluginInstallation {
	return config.PluginInstallation{
		PluginID:   developerplugins.BuiltinNoopPluginID,
		FactoryID:  developerplugins.BuiltinNoopFactoryID,
		Version:    developerplugins.BuiltinNoopVersion,
		SHA256:     developerplugins.BuiltinNoopSHA256,
		APIVersion: middleware.APIVersion,
	}
}
