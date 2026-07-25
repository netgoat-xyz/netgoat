package developerplugins

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"netgoat.xyz/agent/internal/config"
	"netgoat.xyz/agent/internal/middleware"
)

func TestBuiltinRegistryResolvesOnlyExactCatalogSelection(t *testing.T) {
	registry := NewBuiltinRegistry()
	installation := config.PluginInstallation{
		PluginID:   BuiltinNoopPluginID,
		FactoryID:  BuiltinNoopFactoryID,
		Version:    BuiltinNoopVersion,
		SHA256:     BuiltinNoopSHA256,
		APIVersion: middleware.APIVersion,
	}

	resolved, err := registry.Resolve(installation)
	if err != nil {
		t.Fatalf("Resolve() exact built-in selection: %v", err)
	}
	if resolved.Descriptor.PluginID != BuiltinNoopPluginID || string(resolved.Config) != "{}" {
		t.Fatalf("resolved installation = %+v", resolved)
	}

	for name, mutate := range map[string]func(*config.PluginInstallation){
		"package":     func(item *config.PluginInstallation) { item.PluginID = "netgoat.example/other" },
		"factory":     func(item *config.PluginInstallation) { item.FactoryID = "builtin.other" },
		"version":     func(item *config.PluginInstallation) { item.Version = "2.0.0" },
		"digest":      func(item *config.PluginInstallation) { item.SHA256 = strings.Repeat("a", 64) },
		"api version": func(item *config.PluginInstallation) { item.APIVersion = "netgoat.dev/middleware/v2" },
		"capabilities": func(item *config.PluginInstallation) {
			item.GrantedCapabilities = []string{string(middleware.CapabilityRequestRead)}
		},
	} {
		t.Run(name, func(t *testing.T) {
			candidate := installation
			mutate(&candidate)
			if _, err := registry.Resolve(candidate); err == nil {
				t.Fatal("Resolve() accepted a catalog claim that differs from compiled metadata")
			}
		})
	}
}

func TestRegistryCanonicalizesConfigButNeverRunsFactoryDuringResolution(t *testing.T) {
	factoryCalls := 0
	registry, err := NewRegistry(Descriptor{
		PluginID:   "example.test/metadata-only",
		FactoryID:  "example.metadata-only",
		Version:    "1.0.0",
		SHA256:     strings.Repeat("b", 64),
		APIVersion: middleware.APIVersion,
		GrantedCapabilities: []middleware.Capability{
			middleware.CapabilityRouteRead,
			middleware.CapabilityRequestRead,
		},
		Factory: func(json.RawMessage) (middleware.Plugin, error) {
			factoryCalls++
			return testPlugin{}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewRegistry(): %v", err)
	}

	resolved, err := registry.Resolve(config.PluginInstallation{
		PluginID:            "example.test/metadata-only",
		FactoryID:           "example.metadata-only",
		Version:             "1.0.0",
		SHA256:              strings.Repeat("b", 64),
		APIVersion:          middleware.APIVersion,
		GrantedCapabilities: []string{string(middleware.CapabilityRequestRead), string(middleware.CapabilityRouteRead)},
		Config:              map[string]any{"mode": "observe"},
	})
	if err != nil {
		t.Fatalf("Resolve(): %v", err)
	}
	if factoryCalls != 0 {
		t.Fatalf("factory was invoked while resolving metadata: %d calls", factoryCalls)
	}
	if string(resolved.Config) != `{"mode":"observe"}` {
		t.Fatalf("canonical config = %s", resolved.Config)
	}
	if got := resolved.Descriptor.GrantedCapabilities; len(got) != 2 || got[0] != middleware.CapabilityRequestRead || got[1] != middleware.CapabilityRouteRead {
		t.Fatalf("descriptor capabilities were not normalized: %v", got)
	}
}

func TestRegistryRejectsUnsafeOrUnboundedSelections(t *testing.T) {
	registry := NewBuiltinRegistry()
	base := config.PluginInstallation{
		PluginID:   BuiltinNoopPluginID,
		FactoryID:  BuiltinNoopFactoryID,
		Version:    BuiltinNoopVersion,
		SHA256:     BuiltinNoopSHA256,
		APIVersion: middleware.APIVersion,
	}

	for name, mutate := range map[string]func(*config.PluginInstallation){
		"uppercase digest": func(item *config.PluginInstallation) { item.SHA256 = strings.ToUpper(item.SHA256) },
		"short digest":     func(item *config.PluginInstallation) { item.SHA256 = "abc" },
		"whitespace id":    func(item *config.PluginInstallation) { item.PluginID = " " + item.PluginID },
		"unknown grant":    func(item *config.PluginInstallation) { item.GrantedCapabilities = []string{"filesystem.read"} },
		"duplicate grant": func(item *config.PluginInstallation) {
			item.GrantedCapabilities = []string{string(middleware.CapabilityRequestRead), string(middleware.CapabilityRequestRead)}
		},
		"oversized config": func(item *config.PluginInstallation) {
			item.Config = map[string]any{"data": strings.Repeat("x", MaxConfigBytes)}
		},
	} {
		t.Run(name, func(t *testing.T) {
			candidate := base
			mutate(&candidate)
			if _, err := registry.Resolve(candidate); err == nil {
				t.Fatal("Resolve() accepted an unsafe or unbounded installation")
			}
		})
	}

	installations := make([]config.PluginInstallation, MaxInstallations+1)
	for index := range installations {
		installations[index] = base
	}
	if _, err := registry.ResolveAll(config.PluginConfig{Installations: installations}); err == nil {
		t.Fatal("ResolveAll() accepted too many installations")
	}
	if _, err := registry.ResolveAll(config.PluginConfig{Installations: []config.PluginInstallation{base, base}}); err == nil {
		t.Fatal("ResolveAll() accepted a duplicate plugin package")
	}
}

func TestDescriptorVerifyPluginRejectsMismatchedCompiledManifest(t *testing.T) {
	descriptor := Descriptor{
		PluginID:   "example.test/expected",
		FactoryID:  "example.expected",
		Version:    "1.0.0",
		SHA256:     strings.Repeat("c", 64),
		APIVersion: middleware.APIVersion,
		Factory: func(json.RawMessage) (middleware.Plugin, error) {
			return testPlugin{}, nil
		},
	}
	if err := descriptor.VerifyPlugin(testPlugin{}); err == nil {
		t.Fatal("VerifyPlugin() accepted a factory result with a mismatched manifest")
	}
}

type testPlugin struct{}

func (testPlugin) Manifest() middleware.Manifest {
	return middleware.Manifest{Name: "test", Version: "1.0.0", APIVersion: middleware.APIVersion}
}

func (testPlugin) Start(context.Context) error { return nil }

func (testPlugin) Handle(context.Context, middleware.Request) (middleware.Decision, error) {
	return middleware.Decision{Action: middleware.ActionAllow}, nil
}

func (testPlugin) Stop(context.Context) error { return nil }
