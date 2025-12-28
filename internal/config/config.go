package config

import (
	"io/ioutil"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DebugLogs bool `yaml:"debug_logs"`
	Honeypot  bool `yaml:"honeypot"`
	Auth      struct {
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

	// AI-based anomaly detection (Hugging Face Inference API)
	Anomaly struct {
		Enabled   bool    `yaml:"enabled"`
		Threshold float64 `yaml:"threshold"`
		Model     string  `yaml:"model"`
		// Optional: If empty, will read from env HUGGINGFACE_TOKEN or HUGGINGFACEHUB_API_TOKEN
		HuggingFaceToken string `yaml:"huggingface_token"`
		// Optional header name to read CSV features from (defaults to X-GoatAI-Features)
		FeatureHeader string `yaml:"feature_header"`
	} `yaml:"anomaly"`

	// Optional per-domain and per-path error pages. Values are file paths.
	// If both domain and path match, path takes precedence by longest prefix.
	ErrorPages struct {
		Domain map[string]string `yaml:"domain"`
		Path   map[string]string `yaml:"path"`
	} `yaml:"error_pages"`
}

func Load(path string) (*Config, error) {
	var config Config
	configFile, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(configFile, &config); err != nil {
		return nil, err
	}
	return &config, nil
}
