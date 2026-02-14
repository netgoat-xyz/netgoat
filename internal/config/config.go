package config

import (
	"os"

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

	API struct {
		URL string `yaml:"url"`
		Key string `yaml:"key"`
	} `yaml:"api"`
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
