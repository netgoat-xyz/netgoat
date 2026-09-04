package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"netgoat.xyz/agent/internal/policy"
)

type Config struct {
	DebugLogs    bool `yaml:"debug_logs"`
	DebugOverlay bool `yaml:"debug_overlay"`
	Honeypot     bool `yaml:"honeypot"`
	// TrustedProxies contains only socket peers that may supply Forwarded or
	// X-Forwarded-For client address chains. Empty is the secure default.
	TrustedProxies []string `yaml:"trusted_proxies"`
	// Listen is the plaintext HTTP bind address used when SSL is disabled.
	// An empty value defaults to ":8080". Public (non-loopback) plaintext
	// binds are rejected unless AllowInsecurePublicHTTP is set.
	Listen string `yaml:"listen"`
	// AllowInsecurePublicHTTP is the explicit operator opt-in for plaintext
	// HTTP on a non-loopback address. It is never implied by other settings.
	AllowInsecurePublicHTTP bool `yaml:"allow_insecure_public_http"`
	Auth                    struct {
		Enabled       bool   `yaml:"enabled"`
		SessionSecret string `yaml:"session_secret"`
	} `yaml:"auth"`
	// BotAuth is the pinned Web Bot Auth skip lane. Empty pinned_directories
	// means the verifier is present but never skips; unsigned agents pay PoW.
	// Operator pins should be https directory URLs.
	BotAuth BotAuthConfig `yaml:"bot_auth"`
	SSL     struct {
		Enabled  bool   `yaml:"enabled"`
		CertFile string `yaml:"cert_file"`
		KeyFile  string `yaml:"key_file"`
		Port     string `yaml:"port"`
		ACME     struct {
			// Enabled opts into automatic ACME issuance and renewal. It requires
			// an explicit domain allow-list, accepted CA terms, and the
			// NETGOAT_ACME_CACHE_KEY environment variable.
			Enabled      bool     `yaml:"enabled"`
			AcceptTOS    bool     `yaml:"accept_tos"`
			Email        string   `yaml:"email"`
			Domains      []string `yaml:"domains"`
			CacheDir     string   `yaml:"cache_dir"`
			DirectoryURL string   `yaml:"directory_url"`
			HTTPPort     string   `yaml:"http_port"`
		} `yaml:"acme"`
	} `yaml:"ssl"`
	DynamicRules struct {
		// Enabled activates the bounded TypeScript/JavaScript rule runtime.
		// Rules are administrator-managed source code and are evaluated before
		// the WAF; evaluation failures block the request.
		Enabled                  bool          `yaml:"enabled"`
		Rules                    []DynamicRule `yaml:"rules"`
		MaxRules                 int           `yaml:"max_rules"`
		MaxSourceBytes           int           `yaml:"max_source_bytes"`
		MaxCompiledBytes         int           `yaml:"max_compiled_bytes"`
		MaxInputBytes            int           `yaml:"max_input_bytes"`
		MaxResultBytes           int           `yaml:"max_result_bytes"`
		MaxExecutionMilliseconds int           `yaml:"max_execution_milliseconds"`
	} `yaml:"dynamic_rules"`
	// Plugins contains catalog selections for middleware that is already
	// compiled into this agent binary. It is intentionally restart-only: the
	// agent never downloads, evaluates, or hot-loads plugin code or artifacts.
	Plugins PluginConfig `yaml:"plugins"`
	// Cloudflare contains opt-in integrations. Access assertions are verified
	// before the agent's HTTP handlers run, while reconciliation is performed
	// once at startup from declarative records. API credentials are deliberately
	// not serializable and must be supplied through CLOUDFLARE_API_TOKEN.
	Cloudflare struct {
		Access         CloudflareAccessConfig         `yaml:"access"`
		Reconciliation CloudflareReconciliationConfig `yaml:"reconciliation"`
	} `yaml:"cloudflare"`
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
		URL                      string `yaml:"url"`
		Key                      string `yaml:"key"`
		PollIntervalSeconds      int    `yaml:"poll_interval"`
		ConnectionTimeoutSeconds int    `yaml:"connection_timeout"`
		MaxRetryIntervalSeconds  int    `yaml:"max_retry_interval"`
	} `yaml:"api"`

	Health struct {
		Enabled         *bool  `yaml:"enabled"`
		IntervalSeconds int    `yaml:"interval_seconds"`
		TimeoutSeconds  int    `yaml:"timeout_seconds"`
		Path            string `yaml:"path"`
	} `yaml:"health"`

	Telemetry struct {
		Enabled         bool   `yaml:"enabled"`
		Endpoint        string `yaml:"endpoint"`
		IngestKey       string `yaml:"ingest_key"`
		IntervalSeconds int    `yaml:"interval_seconds"`
	} `yaml:"telemetry"`

	// Database controls the SQLite primary/standby files. Empty values retain
	// the historical defaults used by DatabasePath and DatabaseStandbyPath.
	Database struct {
		Path                  string `yaml:"path"`
		StandbyPath           string `yaml:"standby_path"`
		BackupIntervalSeconds int    `yaml:"backup_interval_seconds"`
	} `yaml:"database"`

	// Routes are local fallback routes keyed by domain, wildcard/regex pattern,
	// or path prefix. Streamed routes with the same key take precedence.
	Routes map[string]Route `yaml:"routes"`
}

// BotAuthConfig is the YAML-safe Web Bot Auth allowlist. Directory URLs are
// operator-pinned https JWKS locations; the agent never fetches an arbitrary
// Signature-Agent URL. The PoW HMAC key is not stored here: set
// NETGOAT_CHALLENGE_SECRET or DiamondKey in the environment.
type BotAuthConfig struct {
	Enabled           bool     `yaml:"enabled"`
	PinnedDirectories []string `yaml:"pinned_directories"`
	JWKSCacheSeconds  int      `yaml:"jwks_cache_seconds"`
}

type Route struct {
	Type           string             `yaml:"type"`
	Target         string             `yaml:"target"`
	Targets        []RouteTarget      `yaml:"targets"`
	CertificatePEM string             `yaml:"certificate_pem"`
	PrivateKeyPEM  string             `yaml:"private_key_pem"`
	Policy         policy.RoutePolicy `yaml:"policy"`
	Active         *bool              `yaml:"active"`
}

// DynamicRule is one ordered JavaScript or TypeScript source unit. An omitted
// enabled flag defaults to true so operators can disable a rule explicitly
// without changing its source.
type DynamicRule struct {
	Name     string `yaml:"name"`
	Language string `yaml:"language"`
	Source   string `yaml:"source"`
	Enabled  *bool  `yaml:"enabled"`
}

// PluginConfig is the serializable catalog selection document. A control-plane
// snapshot uses an explicit plugins_configured flag to distinguish a requested
// empty selection from an older control plane that does not publish plugins.
//
// Installations are metadata claims, not executable packages. The agent only
// activates a selection after it exactly matches a descriptor compiled into
// the running binary.
type PluginConfig struct {
	Installations []PluginInstallation `json:"installations" yaml:"installations"`
}

// PluginInstallation selects one catalog release that may be compiled into
// the agent. Config is a JSON/YAML object passed only to the matched built-in
// factory; it is never interpreted as source code by the catalog runtime.
type PluginInstallation struct {
	PluginID            string         `json:"plugin_id" yaml:"plugin_id"`
	FactoryID           string         `json:"factory_id" yaml:"factory_id"`
	Version             string         `json:"version" yaml:"version"`
	SHA256              string         `json:"sha256" yaml:"sha256"`
	APIVersion          string         `json:"api_version" yaml:"api_version"`
	GrantedCapabilities []string       `json:"granted_capabilities" yaml:"granted_capabilities"`
	Config              map[string]any `json:"config" yaml:"config"`
}

// Clone returns an independently mutable catalog selection. Snapshot copies
// use it so a receiver cannot mutate the manager's remembered configuration.
func (p PluginConfig) Clone() PluginConfig {
	cloned := PluginConfig{Installations: make([]PluginInstallation, len(p.Installations))}
	for index, installation := range p.Installations {
		cloned.Installations[index] = PluginInstallation{
			PluginID:            installation.PluginID,
			FactoryID:           installation.FactoryID,
			Version:             installation.Version,
			SHA256:              installation.SHA256,
			APIVersion:          installation.APIVersion,
			GrantedCapabilities: append([]string(nil), installation.GrantedCapabilities...),
			Config:              clonePluginConfigMap(installation.Config),
		}
	}
	return cloned
}

func clonePluginConfigMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	cloned := make(map[string]any, len(input))
	for key, value := range input {
		cloned[key] = clonePluginConfigValue(value)
	}
	return cloned
}

func clonePluginConfigValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return clonePluginConfigMap(typed)
	case []any:
		cloned := make([]any, len(typed))
		for index, item := range typed {
			cloned[index] = clonePluginConfigValue(item)
		}
		return cloned
	default:
		return value
	}
}

// CloudflareAccessConfig is the YAML-safe representation of a Cloudflare
// Access JWT verifier. The numeric durations are kept in seconds so config
// files remain portable and easy to review.
type CloudflareAccessConfig struct {
	Enabled             bool     `yaml:"enabled"`
	Issuer              string   `yaml:"issuer"`
	Audience            []string `yaml:"audience"`
	JWKSURL             string   `yaml:"jwks_url"`
	Header              string   `yaml:"header"`
	Cookie              string   `yaml:"cookie"`
	JWKSCacheSeconds    int      `yaml:"jwks_cache_seconds"`
	ClockSkewSeconds    int      `yaml:"clock_skew_seconds"`
	FetchTimeoutSeconds int      `yaml:"fetch_timeout_seconds"`
}

// CloudflareReconciliationConfig describes one bounded startup reconciliation
// run. A missing dry_run value is intentionally treated as true by the agent;
// API tokens are read only from CLOUDFLARE_API_TOKEN.
type CloudflareReconciliationConfig struct {
	Enabled               bool                  `yaml:"enabled"`
	DryRun                *bool                 `yaml:"dry_run"`
	AccountID             string                `yaml:"account_id"`
	RequestTimeoutSeconds int                   `yaml:"request_timeout_seconds"`
	DNSRecords            []CloudflareDNSRecord `yaml:"dns_records"`
	Tunnels               []CloudflareTunnel    `yaml:"tunnels"`
}

// CloudflareDNSRecord creates a record when RecordID is omitted and updates
// it when RecordID is present. Delete is intentionally a separate explicit
// action and requires RecordID. Record is passed as a bounded JSON object to
// the Cloudflare DNS API so supported record fields remain forward-compatible.
type CloudflareDNSRecord struct {
	ZoneID   string         `yaml:"zone_id"`
	RecordID string         `yaml:"record_id"`
	Delete   bool           `yaml:"delete"`
	Record   map[string]any `yaml:"record"`
}

// CloudflareTunnel creates a tunnel when TunnelID is omitted and updates one
// when it is set. Delete must be explicit and requires TunnelID. Tunnel is a
// bounded JSON object accepted by Cloudflare's tunnel endpoint.
type CloudflareTunnel struct {
	TunnelID string         `yaml:"tunnel_id"`
	Delete   bool           `yaml:"delete"`
	Tunnel   map[string]any `yaml:"tunnel"`
}

// IsEnabled treats an omitted flag as enabled.
func (r DynamicRule) IsEnabled() bool {
	return r.Enabled == nil || *r.Enabled
}

type RouteTarget struct {
	URL         string `yaml:"url"`
	HealthCheck string `yaml:"health_check"`
}

// IsActive treats an omitted active flag as enabled.
func (r Route) IsActive() bool {
	return r.Active == nil || *r.Active
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
