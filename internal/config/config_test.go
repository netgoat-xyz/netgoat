package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yml")

	configContent := `
debug_logs: true
debug_overlay: true
honeypot: true
auth:
  enabled: true
  session_secret: "supersecret"
ssl:
  enabled: true
  cert_file: "/path/to/cert.pem"
  key_file: "/path/to/key.pem"
  port: ":8443"
custom_error_page: "/path/to/error.html"
anomaly:
  enabled: true
  threshold: 0.75
  model_path: "ai/model.keras"
  scaler_path: "ai/scaler.pkl"
  python_script: "ai/server.py"
  feature_header: "X-Features"
error_pages:
  domain:
    example.com: "/path/to/example-error.html"
    test.com: "/path/to/test-error.html"
  path:
    /api: "/path/to/api-error.html"
    /admin: "/path/to/admin-error.html"
cache:
  enabled: true
  ttl_seconds: 120
  max_entries: 2048
  max_body_bytes: 2097152
api:
  url: "https://api.example.com"
  key: "api_key_12345"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Test basic boolean flags
	if !cfg.DebugLogs {
		t.Error("DebugLogs should be true")
	}
	if !cfg.DebugOverlay {
		t.Error("DebugOverlay should be true")
	}
	if !cfg.Honeypot {
		t.Error("Honeypot should be true")
	}

	// Test auth config
	if !cfg.Auth.Enabled {
		t.Error("Auth.Enabled should be true")
	}
	if cfg.Auth.SessionSecret != "supersecret" {
		t.Errorf("Auth.SessionSecret = %s, want supersecret", cfg.Auth.SessionSecret)
	}

	// Test SSL config
	if !cfg.SSL.Enabled {
		t.Error("SSL.Enabled should be true")
	}
	if cfg.SSL.CertFile != "/path/to/cert.pem" {
		t.Errorf("SSL.CertFile = %s, want /path/to/cert.pem", cfg.SSL.CertFile)
	}
	if cfg.SSL.KeyFile != "/path/to/key.pem" {
		t.Errorf("SSL.KeyFile = %s, want /path/to/key.pem", cfg.SSL.KeyFile)
	}
	if cfg.SSL.Port != ":8443" {
		t.Errorf("SSL.Port = %s, want :8443", cfg.SSL.Port)
	}

	// Test custom error page
	if cfg.CustomErrorPage != "/path/to/error.html" {
		t.Errorf("CustomErrorPage = %s, want /path/to/error.html", cfg.CustomErrorPage)
	}

	// Test anomaly config
	if !cfg.Anomaly.Enabled {
		t.Error("Anomaly.Enabled should be true")
	}
	if cfg.Anomaly.Threshold != 0.75 {
		t.Errorf("Anomaly.Threshold = %f, want 0.75", cfg.Anomaly.Threshold)
	}
	if cfg.Anomaly.ModelPath != "ai/model.keras" {
		t.Errorf("Anomaly.ModelPath = %s, want ai/model.keras", cfg.Anomaly.ModelPath)
	}
	if cfg.Anomaly.ScalerPath != "ai/scaler.pkl" {
		t.Errorf("Anomaly.ScalerPath = %s, want ai/scaler.pkl", cfg.Anomaly.ScalerPath)
	}
	if cfg.Anomaly.PythonScript != "ai/server.py" {
		t.Errorf("Anomaly.PythonScript = %s, want ai/server.py", cfg.Anomaly.PythonScript)
	}
	if cfg.Anomaly.FeatureHeader != "X-Features" {
		t.Errorf("Anomaly.FeatureHeader = %s, want X-Features", cfg.Anomaly.FeatureHeader)
	}

	// Test error pages config
	if len(cfg.ErrorPages.Domain) != 2 {
		t.Errorf("ErrorPages.Domain length = %d, want 2", len(cfg.ErrorPages.Domain))
	}
	if cfg.ErrorPages.Domain["example.com"] != "/path/to/example-error.html" {
		t.Errorf("ErrorPages.Domain[example.com] = %s", cfg.ErrorPages.Domain["example.com"])
	}
	if cfg.ErrorPages.Domain["test.com"] != "/path/to/test-error.html" {
		t.Errorf("ErrorPages.Domain[test.com] = %s", cfg.ErrorPages.Domain["test.com"])
	}

	if len(cfg.ErrorPages.Path) != 2 {
		t.Errorf("ErrorPages.Path length = %d, want 2", len(cfg.ErrorPages.Path))
	}
	if cfg.ErrorPages.Path["/api"] != "/path/to/api-error.html" {
		t.Errorf("ErrorPages.Path[/api] = %s", cfg.ErrorPages.Path["/api"])
	}
	if cfg.ErrorPages.Path["/admin"] != "/path/to/admin-error.html" {
		t.Errorf("ErrorPages.Path[/admin] = %s", cfg.ErrorPages.Path["/admin"])
	}

	// Test cache config
	if !cfg.Cache.Enabled {
		t.Error("Cache.Enabled should be true")
	}
	if cfg.Cache.TTLSeconds != 120 {
		t.Errorf("Cache.TTLSeconds = %d, want 120", cfg.Cache.TTLSeconds)
	}
	if cfg.Cache.MaxEntries != 2048 {
		t.Errorf("Cache.MaxEntries = %d, want 2048", cfg.Cache.MaxEntries)
	}
	if cfg.Cache.MaxBodyBytes != 2097152 {
		t.Errorf("Cache.MaxBodyBytes = %d, want 2097152", cfg.Cache.MaxBodyBytes)
	}

	// Test API config
	if cfg.API.URL != "https://api.example.com" {
		t.Errorf("API.URL = %s, want https://api.example.com", cfg.API.URL)
	}
	if cfg.API.Key != "api_key_12345" {
		t.Errorf("API.Key = %s, want api_key_12345", cfg.API.Key)
	}
}

func TestLoadMinimalConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yml")

	configContent := `
debug_logs: false
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Should load with defaults
	if cfg.DebugLogs {
		t.Error("DebugLogs should be false")
	}
	if cfg.Auth.Enabled {
		t.Error("Auth.Enabled should be false (default)")
	}
	if cfg.Cache.Enabled {
		t.Error("Cache.Enabled should be false (default)")
	}
}

func TestLoadEmptyConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yml")

	err := os.WriteFile(configFile, []byte(""), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Should load with all defaults
	if cfg == nil {
		t.Fatal("Config should not be nil")
	}
	if cfg.DebugLogs {
		t.Error("DebugLogs should be false by default")
	}
}

func TestLoadNonExistentFile(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yml")
	if err == nil {
		t.Error("Load should fail for nonexistent file")
	}
	if cfg != nil {
		t.Error("Config should be nil for failed load")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yml")

	invalidContent := `
debug_logs: true
  invalid_indent: bad
    more_bad: indent
[invalid: yaml}
`

	err := os.WriteFile(configFile, []byte(invalidContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	_, err = Load(configFile)
	if err == nil {
		t.Error("Load should fail for invalid YAML")
	}
}

func TestLoadWithTypeMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yml")

	// Boolean field with string value
	configContent := `
debug_logs: "not a boolean"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	_, err = Load(configFile)
	if err == nil {
		t.Error("Load should fail for type mismatch")
	}
}

func TestLoadNestedStructures(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yml")

	configContent := `
error_pages:
  domain:
    example.com: "/error1.html"
  path:
    /api: "/error2.html"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.ErrorPages.Domain == nil {
		t.Fatal("ErrorPages.Domain should not be nil")
	}
	if cfg.ErrorPages.Path == nil {
		t.Fatal("ErrorPages.Path should not be nil")
	}

	if len(cfg.ErrorPages.Domain) != 1 {
		t.Errorf("ErrorPages.Domain length = %d, want 1", len(cfg.ErrorPages.Domain))
	}
	if len(cfg.ErrorPages.Path) != 1 {
		t.Errorf("ErrorPages.Path length = %d, want 1", len(cfg.ErrorPages.Path))
	}
}

func TestLoadWithNumericValues(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yml")

	configContent := `
anomaly:
  threshold: 0.85
cache:
  ttl_seconds: 300
  max_entries: 5000
  max_body_bytes: 10485760
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Anomaly.Threshold != 0.85 {
		t.Errorf("Anomaly.Threshold = %f, want 0.85", cfg.Anomaly.Threshold)
	}
	if cfg.Cache.TTLSeconds != 300 {
		t.Errorf("Cache.TTLSeconds = %d, want 300", cfg.Cache.TTLSeconds)
	}
	if cfg.Cache.MaxEntries != 5000 {
		t.Errorf("Cache.MaxEntries = %d, want 5000", cfg.Cache.MaxEntries)
	}
	if cfg.Cache.MaxBodyBytes != 10485760 {
		t.Errorf("Cache.MaxBodyBytes = %d, want 10485760", cfg.Cache.MaxBodyBytes)
	}
}

func TestLoadWithZeroValues(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yml")

	configContent := `
anomaly:
  threshold: 0.0
cache:
  ttl_seconds: 0
  max_entries: 0
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Zero values should be preserved
	if cfg.Anomaly.Threshold != 0.0 {
		t.Errorf("Anomaly.Threshold = %f, want 0.0", cfg.Anomaly.Threshold)
	}
	if cfg.Cache.TTLSeconds != 0 {
		t.Errorf("Cache.TTLSeconds = %d, want 0", cfg.Cache.TTLSeconds)
	}
	if cfg.Cache.MaxEntries != 0 {
		t.Errorf("Cache.MaxEntries = %d, want 0", cfg.Cache.MaxEntries)
	}
}

func TestLoadWithEmptyStrings(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yml")

	configContent := `
auth:
  session_secret: ""
ssl:
  cert_file: ""
api:
  url: ""
  key: ""
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Empty strings should be preserved
	if cfg.Auth.SessionSecret != "" {
		t.Errorf("Auth.SessionSecret = %q, want empty string", cfg.Auth.SessionSecret)
	}
	if cfg.SSL.CertFile != "" {
		t.Errorf("SSL.CertFile = %q, want empty string", cfg.SSL.CertFile)
	}
	if cfg.API.URL != "" {
		t.Errorf("API.URL = %q, want empty string", cfg.API.URL)
	}
}

func TestConfigStruct(t *testing.T) {
	cfg := &Config{}

	// Test that struct can be instantiated
	if cfg == nil {
		t.Fatal("Config struct should be instantiable")
	}

	// Test default values
	if cfg.DebugLogs {
		t.Error("DebugLogs should default to false")
	}
	if cfg.Auth.Enabled {
		t.Error("Auth.Enabled should default to false")
	}
	if cfg.SSL.Enabled {
		t.Error("SSL.Enabled should default to false")
	}
}

func TestLoadWithSpecialCharacters(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yml")

	configContent := `
auth:
  session_secret: "p@$$w0rd!#$%^&*()"
api:
  key: "key-with-dashes_and_underscores123"
custom_error_page: "/path/with spaces/error.html"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Auth.SessionSecret != "p@$$w0rd!#$%^&*()" {
		t.Errorf("Auth.SessionSecret not preserved correctly")
	}
	if cfg.API.Key != "key-with-dashes_and_underscores123" {
		t.Errorf("API.Key not preserved correctly")
	}
	if cfg.CustomErrorPage != "/path/with spaces/error.html" {
		t.Errorf("CustomErrorPage not preserved correctly")
	}
}