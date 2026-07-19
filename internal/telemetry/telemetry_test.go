package telemetry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestClientDeliversAuthenticatedLifecycleWithoutBlockingStart(t *testing.T) {
	t.Setenv("TELEMETRY_ENDPOINT", "")
	t.Setenv("TELEMETRY_INGEST_KEY", "")
	events := make(chan Payload, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Telemetry-Key"); got != "shared-secret" {
			t.Errorf("X-Telemetry-Key = %q", got)
		}
		var payload Payload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		events <- payload
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(Config{
		Enabled: true, Endpoint: server.URL, IngestKey: "shared-secret",
		DataDir: t.TempDir(), Interval: time.Hour,
		StatsFunc: func() AppStats { return AppStats{Requests: 7} },
	})
	started := time.Now()
	client.Start()
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("Start blocked for %s", elapsed)
	}

	startup := waitForEvent(t, events)
	if startup.EventType != "startup" || startup.InstanceID == "" || startup.App == nil || startup.App.Requests != 7 {
		t.Fatalf("unexpected startup payload: %+v", startup)
	}
	client.Stop()
	client.Stop()
	if shutdown := waitForEvent(t, events); shutdown.EventType != "shutdown" {
		t.Fatalf("last event = %q, want shutdown", shutdown.EventType)
	}
}

func TestClientPersistsPrivateValidatedID(t *testing.T) {
	dir := t.TempDir()
	client := NewClient(Config{DataDir: dir})
	id, err := client.loadOrCreateID()
	if err != nil {
		t.Fatal(err)
	}
	if !validUUID(id) {
		t.Fatalf("invalid generated UUID %q", id)
	}
	path := filepath.Join(dir, idFilename)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("ID permissions = %o, want 600", got)
	}

	if err := os.WriteFile(path, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := client.loadOrCreateID(); err == nil {
		t.Fatal("corrupt stored ID was accepted")
	}
}

func TestValidateEndpoint(t *testing.T) {
	for _, endpoint := range []string{"", "ftp://example.test/api", "/api", "http://user:pass@example.test/api"} {
		if err := validateEndpoint(endpoint); err == nil {
			t.Errorf("validateEndpoint(%q) succeeded", endpoint)
		}
	}
	if err := validateEndpoint("https://telemetry.example.test/api"); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentStopIsSafe(t *testing.T) {
	client := NewClient(Config{})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); client.Stop() }()
	}
	wg.Wait()
}

func waitForEvent(t *testing.T, events <-chan Payload) Payload {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for telemetry event")
		return Payload{}
	}
}
