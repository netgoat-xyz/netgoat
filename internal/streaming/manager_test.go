package streaming

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestManagerPersistsSensitiveSnapshotPrivately(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot.json")
	mgr := NewManager(path)
	defer mgr.Close()

	snapshot := ConfigSnapshot{
		RoutesConfigured: true,
		Routes: map[string]RouteData{
			"private.example.test": {
				Type:          "domain",
				Target:        "http://127.0.0.1:9000",
				PrivateKeyPEM: "private-key-material",
			},
		},
		WAFRules:    map[string]WAFRuleData{},
		Users:       []UserData{},
		UserDomains: []UserDomainData{},
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.HandleMessage(&Message{Type: "snapshot", Version: 1, Timestamp: time.Now(), Data: data}); err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	// Windows does not expose POSIX mode bits through os.FileInfo; ACL policy
	// is deployment-specific there. Unix-like platforms must retain the
	// owner-only invariant enforced by saveToDisk.
	if runtime.GOOS != "windows" {
		if got := info.Mode().Perm(); got != 0600 {
			t.Fatalf("snapshot mode = %o, want 600", got)
		}
	}
	loaded := mgr.GetSnapshot()
	if !loaded.RoutesConfigured {
		t.Fatal("route presence metadata was not preserved")
	}
}
