package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestRegistryAppliesCapabilitiesAndBlockResponse(t *testing.T) {
	var received Request
	plugin := &testPlugin{
		manifest: Manifest{
			Name:         "blocker",
			Version:      "1.0.0",
			APIVersion:   APIVersion,
			Capabilities: []Capability{CapabilityRequestRead, CapabilityRouteRead, CapabilityResponseWrite},
		},
		handle: func(_ context.Context, request Request) (Decision, error) {
			received = request
			request.Header.Set("X-Mutated", "no")
			return Decision{
				Action: ActionBlock,
				Status: http.StatusUnavailableForLegalReasons,
				Headers: http.Header{
					"X-Middleware": {"blocker"},
				},
				Body: []byte("blocked"),
			}, nil
		},
	}
	registry := NewRegistry(CapabilityRequestRead, CapabilityRouteRead, CapabilityResponseWrite)
	if err := registry.Register(plugin, CapabilityRequestRead, CapabilityRouteRead, CapabilityResponseWrite); err != nil {
		t.Fatalf("Register(): %v", err)
	}
	if err := registry.Start(context.Background()); err != nil {
		t.Fatalf("Start(): %v", err)
	}
	defer registry.Close(context.Background())

	nextCalled := false
	handler := registry.Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { nextCalled = true }))
	request := httptest.NewRequest(http.MethodGet, "http://api.example.test/v1/items?limit=5", nil)
	request.Header.Set("X-Input", "yes")
	request = WithRequestMetadata(request, RequestMetadata{ClientIP: "203.0.113.1", RouteKey: "api.example.test"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnavailableForLegalReasons {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnavailableForLegalReasons)
	}
	if response.Body.String() != "blocked" || response.Header().Get("X-Middleware") != "blocker" {
		t.Fatalf("unexpected response: status=%d body=%q headers=%v", response.Code, response.Body.String(), response.Header())
	}
	if nextCalled {
		t.Fatal("next handler was called after a block")
	}
	if received.Method != http.MethodGet || received.Host != "api.example.test" || received.Path != "/v1/items" || received.RawQuery != "limit=5" || received.ClientIP != "203.0.113.1" || received.RouteKey != "api.example.test" {
		t.Fatalf("request view = %#v", received)
	}
	if request.Header.Get("X-Mutated") != "" {
		t.Fatal("plugin mutated the original request headers")
	}
}

func TestRegistryDoesNotExposeUngrantedRequestData(t *testing.T) {
	var received Request
	plugin := &testPlugin{
		manifest: Manifest{Name: "route-only", Version: "1.0.0", APIVersion: APIVersion, Capabilities: []Capability{CapabilityRouteRead}},
		handle: func(_ context.Context, request Request) (Decision, error) {
			received = request
			return Decision{Action: ActionAllow}, nil
		},
	}
	registry := NewRegistry(CapabilityRouteRead)
	if err := registry.Register(plugin, CapabilityRouteRead); err != nil {
		t.Fatalf("Register(): %v", err)
	}
	handler := registry.Handler(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusNoContent) }))
	request := httptest.NewRequest(http.MethodPost, "http://api.example.test/private?q=1", nil)
	request.Header.Set("Authorization", "secret")
	request = WithRequestMetadata(request, RequestMetadata{ClientIP: "203.0.113.1", RouteKey: "route-a"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if received.RouteKey != "route-a" || received.Method != "" || received.Host != "" || received.Path != "" || received.RawQuery != "" || received.ClientIP != "" || len(received.Header) != 0 {
		t.Fatalf("request view exposed ungranted data: %#v", received)
	}
}

func TestRegistryRejectsInvalidOrUngrantedCapabilities(t *testing.T) {
	plugin := &testPlugin{manifest: Manifest{Name: "needs-route", Version: "1.0.0", APIVersion: APIVersion, Capabilities: []Capability{CapabilityRouteRead}}}
	registry := NewRegistry(CapabilityRequestRead)
	if err := registry.Register(plugin, CapabilityRouteRead); err == nil {
		t.Fatal("Register() succeeded with a capability denied by registry policy")
	}

	registry = NewRegistry(CapabilityRouteRead)
	if err := registry.Register(plugin); err == nil {
		t.Fatal("Register() succeeded without a declared capability grant")
	}

	badAPI := &testPlugin{manifest: Manifest{Name: "old", Version: "1.0.0", APIVersion: "netgoat.dev/middleware/v0"}}
	if err := registry.Register(badAPI); err == nil {
		t.Fatal("Register() succeeded with an incompatible API version")
	}
}

func TestRegistryContainsPluginFailureAndPanic(t *testing.T) {
	for name, handler := range map[string]func(context.Context, Request) (Decision, error){
		"error": func(context.Context, Request) (Decision, error) { return Decision{}, errors.New("boom") },
		"panic": func(context.Context, Request) (Decision, error) { panic("boom") },
	} {
		t.Run(name, func(t *testing.T) {
			plugin := &testPlugin{manifest: Manifest{Name: name, Version: "1.0.0", APIVersion: APIVersion}, handle: handler}
			registry := NewRegistry()
			if err := registry.Register(plugin); err != nil {
				t.Fatalf("Register(): %v", err)
			}
			nextCalled := false
			response := httptest.NewRecorder()
			registry.Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { nextCalled = true })).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "http://example.test", nil))
			if response.Code != http.StatusInternalServerError || nextCalled {
				t.Fatalf("failure handling status=%d nextCalled=%t", response.Code, nextCalled)
			}
		})
	}
}

func TestRegistryLifecycleOrderAndRollback(t *testing.T) {
	events := []string{}
	first := &testPlugin{manifest: Manifest{Name: "first", Version: "1.0.0", APIVersion: APIVersion}, events: &events}
	second := &testPlugin{manifest: Manifest{Name: "second", Version: "1.0.0", APIVersion: APIVersion}, events: &events, startErr: errors.New("unavailable")}
	registry := NewRegistry()
	for _, plugin := range []Plugin{first, second} {
		if err := registry.Register(plugin); err != nil {
			t.Fatalf("Register(): %v", err)
		}
	}
	if err := registry.Start(context.Background()); err == nil {
		t.Fatal("Start() succeeded despite plugin failure")
	}
	if want := []string{"start:first", "start:second", "stop:first"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}

	events = nil
	second.startErr = nil
	if err := registry.Start(context.Background()); err != nil {
		t.Fatalf("Start() after recovery: %v", err)
	}
	if err := registry.Close(context.Background()); err != nil {
		t.Fatalf("Close(): %v", err)
	}
	if want := []string{"start:first", "start:second", "stop:second", "stop:first"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestLoaderLoadsCompiledInFactory(t *testing.T) {
	loader := NewLoader()
	if err := loader.RegisterFactory("example", func(raw json.RawMessage) (Plugin, error) {
		if string(raw) != `{"mode":"safe"}` {
			return nil, errors.New("unexpected config")
		}
		return &testPlugin{manifest: Manifest{Name: "example", Version: "1.0.0", APIVersion: APIVersion}}, nil
	}); err != nil {
		t.Fatalf("RegisterFactory(): %v", err)
	}
	registry := NewRegistry()
	if err := registry.LoadAndRegister(loader, []Spec{{Name: "example", Enabled: true, Config: json.RawMessage(`{"mode":"safe"}`)}}); err != nil {
		t.Fatalf("LoadAndRegister(): %v", err)
	}
	if err := registry.LoadAndRegister(loader, []Spec{{Name: "missing", Enabled: true}}); err == nil {
		t.Fatal("LoadAndRegister() succeeded for unknown factory")
	}
}

type testPlugin struct {
	manifest Manifest
	handle   func(context.Context, Request) (Decision, error)
	events   *[]string
	startErr error
}

func (plugin *testPlugin) Manifest() Manifest {
	return plugin.manifest
}

func (plugin *testPlugin) Start(context.Context) error {
	if plugin.events != nil {
		*plugin.events = append(*plugin.events, "start:"+plugin.manifest.Name)
	}
	return plugin.startErr
}

func (plugin *testPlugin) Handle(ctx context.Context, request Request) (Decision, error) {
	if plugin.handle == nil {
		return Decision{Action: ActionAllow}, nil
	}
	return plugin.handle(ctx, request)
}

func (plugin *testPlugin) Stop(context.Context) error {
	if plugin.events != nil {
		*plugin.events = append(*plugin.events, "stop:"+plugin.manifest.Name)
	}
	return nil
}
