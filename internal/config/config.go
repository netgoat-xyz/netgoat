package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DebugLogs    bool `yaml:"debug_logs"`
	DebugOverlay bool `yaml:"debug_overlay"`
	Honeypot     bool `yaml:"honeypot"`
	Auth         struct {
		Enabled       bool   `yaml:"enabled"`
		SessionSecret string `yaml:"session_secret"`
	} `yaml:"auth"`
	SSL struct {
		Enabled  bool   `yaml:"enabled"`
		CertFile string `yaml:"cert_file"`
		KeyFile  string `yaml:"key_file"`
		Port     string `yaml:"port"`
	} `yaml:"ssl"`
	// Path to a static HTML file to serve for errors (e.g., 403/404/500)
	CustomErrorPage string `yaml:"custom_error_page"`

	// AI-based anomaly detection (local Keras model + sklearn scaler)
	Anomaly struct {
		Enabled       bool    `yaml:"enabled"`
		Threshold     float64 `yaml:"threshold"`
		ModelPath     string  `yaml:"model_path"`     // Path to goatai.keras
		ScalerPath    string  `yaml:"scaler_path"`    // Path to scaler.pkl
		PythonScript  string  `yaml:"python_script"`  // Path to model_server.py
		FeatureHeader string  `yaml:"feature_header"` // Header name to read CSV from
	} `yaml:"anomaly"`

	// Koda-Waf: ML-enhanced WAF attack classification model.
	KodaWaf struct {
		Enabled       bool    `yaml:"enabled"`
		Threshold     float64 `yaml:"threshold"`
		ModelPath     string  `yaml:"model_path"`    // Path to smart_waf_model.pkl
		ScalerPath    string  `yaml:"scaler_path"`   // Path to model_features.pkl
		PythonScript  string  `yaml:"python_script"` // Path to koda_waf_server.py
		FeatureHeader string  `yaml:"feature_header"`
	} `yaml:"koda_waf"`

	// Koda-2: next-generation anomaly detection model.
	Koda2 struct {
		Enabled       bool    `yaml:"enabled"`
		Threshold     float64 `yaml:"threshold"`
		ModelPath     string  `yaml:"model_path"`    // Path to koda2.keras
		ScalerPath    string  `yaml:"scaler_path"`   // Path to koda2_scaler.pkl
		PythonScript  string  `yaml:"python_script"` // Path to koda2_server.py
		FeatureHeader string  `yaml:"feature_header"`
	} `yaml:"koda_2"`

	// Optional per-domain and per-path error pages. Values are file paths.
	// If both domain and path match, path takes precedence by longest prefix.
	ErrorPages struct {
		Domain map[string]string `yaml:"domain"`
		Path   map[string]string `yaml:"path"`
	} `yaml:"error_pages"`

	Cache struct {
		Enabled      bool `yaml:"enabled"`
		TTLSeconds   int  `yaml:"ttl_seconds"`
		MaxEntries   int  `yaml:"max_entries"`
		MaxBodyBytes int  `yaml:"max_body_bytes"`
	} `yaml:"cache"`

	RateLimit struct {
		Enabled           bool   `yaml:"enabled"`
		RequestsPerMinute int    `yaml:"requests_per_minute"`
		Burst             int    `yaml:"burst"`
		Key               string `yaml:"key"`
	} `yaml:"rate_limit"`

	RequestQueue struct {
		Enabled        bool `yaml:"enabled"`
		MaxConcurrent  int  `yaml:"max_concurrent"`
		MaxQueued      int  `yaml:"max_queued"`
		TimeoutSeconds int  `yaml:"timeout_seconds"`
	} `yaml:"request_queue"`

	Bandwidth struct {
		Enabled        bool   `yaml:"enabled"`
		BytesPerSecond int    `yaml:"bytes_per_second"`
		BurstBytes     int    `yaml:"burst_bytes"`
		Key            string `yaml:"key"`
	} `yaml:"bandwidth"`

	Metrics struct {
		Enabled bool   `yaml:"enabled"`
		Path    string `yaml:"path"`
	} `yaml:"metrics"`

	API struct {
		URL string `yaml:"url"`
		Key string `yaml:"key"`
	} `yaml:"api"`

	Health struct {
		Enabled         *bool  `yaml:"enabled"`
		IntervalSeconds int    `yaml:"interval_seconds"`
		TimeoutSeconds  int    `yaml:"timeout_seconds"`
		Path            string `yaml:"path"`
	} `yaml:"health"`

	// Local SQLite persistence and automatic data failover.
	Database struct {
		Path                  string `yaml:"path"`
		StandbyPath           string `yaml:"standby_path"`
		BackupIntervalSeconds int    `yaml:"backup_interval_seconds"`
	} `yaml:"database"`
}

// HealthChecksEnabled reports whether upstream health probes should run.
// Probes default to enabled when the config field is omitted.
func (c *Config) HealthChecksEnabled() bool {
	if c == nil || c.Health.Enabled == nil {
		return true
	}
	return *c.Health.Enabled
}

// DatabasePath returns the primary SQLite path (default ./database/proxy.db).
func (c *Config) DatabasePath() string {
	if c != nil && strings.TrimSpace(c.Database.Path) != "" {
		return c.Database.Path
	}
	return "./database/proxy.db"
}

// DatabaseStandbyPath returns the hot-standby SQLite path.
// Defaults to <primary without extension>.standby.db.
func (c *Config) DatabaseStandbyPath() string {
	if c != nil && strings.TrimSpace(c.Database.StandbyPath) != "" {
		return c.Database.StandbyPath
	}
	primary := c.DatabasePath()
	ext := filepath.Ext(primary)
	if ext == "" {
		return primary + ".standby.db"
	}
	return strings.TrimSuffix(primary, ext) + ".standby" + ext
}

// DatabaseBackupIntervalSeconds returns how often to refresh the standby copy.
// Zero means periodic backups are disabled (snapshot-triggered backups still run).
func (c *Config) DatabaseBackupIntervalSeconds() int {
	if c == nil || c.Database.BackupIntervalSeconds <= 0 {
		return 0
	}
	return c.Database.BackupIntervalSeconds
}

func Load(path string) (*Config, error) {
	var config Config
	configFile, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(configFile, &config); err != nil {
		return nil, err
	}
	return &config, nil
}
