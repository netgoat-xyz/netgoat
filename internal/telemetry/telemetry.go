package telemetry

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	defaultEndpoint = "http://127.0.0.1:9091/api"
	defaultInterval = 5 * time.Minute
	idFilename      = ".telemetry-id"
	maxResponseBody = 4 << 10
)

type Config struct {
	Enabled    bool
	Endpoint   string
	IngestKey  string
	DataDir    string
	Interval   time.Duration
	StatsFunc  func() AppStats
	HTTPClient *http.Client
}

type AppStats struct {
	Routes           int               `json:"routes"`
	Users            int               `json:"users"`
	Requests         uint64            `json:"requests"`
	Blocked          uint64            `json:"blocked"`
	ProxyErrors      uint64            `json:"proxy_errors"`
	TotalErrors      uint64            `json:"total_errors"`
	AvgLatency       float64           `json:"avg_latency_ms"`
	StatusCodes      map[string]uint64 `json:"status_codes,omitempty"`
	BlockReasons     map[string]uint64 `json:"block_reasons,omitempty"`
	ErrorStatusCodes map[string]uint64 `json:"error_status_codes,omitempty"`
	RecentErrors     []ErrorInfo       `json:"recent_errors,omitempty"`
}

type ErrorInfo struct {
	Kind     string    `json:"kind"`
	Message  string    `json:"message"`
	Count    uint64    `json:"count"`
	LastSeen time.Time `json:"last_seen"`
}

type SysInfo struct {
	CPUModel    string `json:"cpu_model"`
	CPUCores    int    `json:"cpu_cores"`
	RAMTotalMB  int64  `json:"ram_total_mb"`
	DiskTotalMB int64  `json:"disk_total_mb"`
	DiskFreeMB  int64  `json:"disk_free_mb"`
	Kernel      string `json:"kernel"`
}

type Payload struct {
	InstanceID  string    `json:"instance_id"`
	Hostname    string    `json:"hostname"`
	GoVersion   string    `json:"go_version"`
	OS          string    `json:"os"`
	Arch        string    `json:"arch"`
	Kernel      string    `json:"kernel"`
	CPUModel    string    `json:"cpu_model"`
	CPUCores    int       `json:"cpu_cores"`
	RAMTotalMB  int64     `json:"ram_total_mb"`
	DiskTotalMB int64     `json:"disk_total_mb"`
	DiskFreeMB  int64     `json:"disk_free_mb"`
	Uptime      string    `json:"uptime"`
	EventType   string    `json:"event_type"`
	Timestamp   time.Time `json:"timestamp"`
	App         *AppStats `json:"app,omitempty"`
}

type Client struct {
	cfg        Config
	endpoint   string
	ingestKey  string
	instanceID string
	startedAt  time.Time
	sysInfo    SysInfo
	client     *http.Client
	done       chan struct{}
	startOnce  sync.Once
	stopOnce   sync.Once
	wg         sync.WaitGroup
	active     atomic.Bool
}

func NewClient(cfg Config) *Client {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	if value := strings.TrimSpace(os.Getenv("TELEMETRY_ENDPOINT")); value != "" {
		endpoint = value
	}
	ingestKey := cfg.IngestKey
	if value := os.Getenv("TELEMETRY_INGEST_KEY"); value != "" {
		ingestKey = value
	}
	if cfg.Interval <= 0 {
		cfg.Interval = defaultInterval
	}
	if cfg.DataDir == "" {
		cfg.DataDir = "database"
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	return &Client{
		cfg:       cfg,
		endpoint:  endpoint,
		ingestKey: ingestKey,
		startedAt: time.Now(),
		client:    client,
		done:      make(chan struct{}),
	}
}

// Start launches telemetry delivery without delaying proxy startup.
func (t *Client) Start() {
	if t == nil || !t.cfg.Enabled {
		return
	}
	t.startOnce.Do(func() {
		if err := validateEndpoint(t.endpoint); err != nil {
			log.Warn().Err(err).Msg("Telemetry disabled because its endpoint is invalid")
			return
		}
		id, err := t.loadOrCreateID()
		if err != nil {
			log.Warn().Err(err).Msg("Telemetry disabled because its instance ID is unavailable")
			return
		}
		t.instanceID = id
		t.sysInfo = collectSysInfo(t.cfg.DataDir)
		t.active.Store(true)
		t.wg.Add(1)
		go t.run()
		log.Info().Str("endpoint", t.endpoint).Msg("Opt-in telemetry started")
	})
}

func (t *Client) run() {
	defer t.wg.Done()
	defer t.active.Store(false)
	t.sendEvent("startup")
	ticker := time.NewTicker(t.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			t.sendEvent("heartbeat")
		case <-t.done:
			t.sendEvent("shutdown")
			return
		}
	}
}

// Stop is safe to call more than once.
func (t *Client) Stop() {
	if t == nil {
		return
	}
	t.stopOnce.Do(func() { close(t.done) })
	t.wg.Wait()
	if t.cfg.Enabled {
		log.Info().Msg("Telemetry stopped")
	}
}

func (t *Client) sendEvent(eventType string) {
	hostname, _ := os.Hostname()
	payload := Payload{
		InstanceID: t.instanceID, Hostname: hostname, GoVersion: runtime.Version(),
		OS: runtime.GOOS, Arch: runtime.GOARCH, Kernel: t.sysInfo.Kernel,
		CPUModel: t.sysInfo.CPUModel, CPUCores: t.sysInfo.CPUCores,
		RAMTotalMB: t.sysInfo.RAMTotalMB, DiskTotalMB: t.sysInfo.DiskTotalMB,
		DiskFreeMB: t.sysInfo.DiskFreeMB, Uptime: time.Since(t.startedAt).Round(time.Second).String(),
		EventType: eventType, Timestamp: time.Now(),
	}
	if t.cfg.StatsFunc != nil {
		stats := t.cfg.StatsFunc()
		payload.App = &stats
	}
	body, err := json.Marshal(payload)
	if err != nil {
		log.Debug().Err(err).Msg("Telemetry marshal failed")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.endpoint, bytes.NewReader(body))
	if err != nil {
		log.Debug().Err(err).Msg("Telemetry request creation failed")
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if t.ingestKey != "" {
		req.Header.Set("X-Telemetry-Key", t.ingestKey)
	}
	resp, err := t.client.Do(req)
	if err != nil {
		log.Debug().Err(err).Msg("Telemetry delivery failed")
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBody))
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		log.Debug().Int("status", resp.StatusCode).Msg("Telemetry endpoint rejected event")
	}
}

func validateEndpoint(endpoint string) error {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return err
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return errors.New("endpoint must be an absolute HTTP or HTTPS URL")
	}
	if parsed.User != nil {
		return errors.New("endpoint must not contain URL credentials")
	}
	return nil
}

func (t *Client) loadOrCreateID() (string, error) {
	path := filepath.Join(t.cfg.DataDir, idFilename)
	data, err := os.ReadFile(path)
	if err == nil {
		id := strings.TrimSpace(string(data))
		if !validUUID(id) {
			return "", errors.New("stored telemetry ID is invalid")
		}
		if err := os.Chmod(path, 0600); err != nil {
			return "", fmt.Errorf("secure telemetry id: %w", err)
		}
		return id, nil
	}
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("read telemetry id: %w", err)
	}
	id, err := newUUID()
	if err != nil {
		return "", fmt.Errorf("generate telemetry id: %w", err)
	}
	if err := os.MkdirAll(t.cfg.DataDir, 0700); err != nil {
		return "", fmt.Errorf("create data dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(id+"\n"), 0600); err != nil {
		return "", fmt.Errorf("write telemetry id: %w", err)
	}
	return id, nil
}

func validUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	for index, char := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			continue
		}
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
			return false
		}
	}
	return true
}

func newUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func collectSysInfo(dataDir string) SysInfo {
	info := SysInfo{CPUCores: runtime.NumCPU()}
	if b, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "model name") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					info.CPUModel = strings.TrimSpace(parts[1])
				}
				break
			}
		}
	}
	if b, err := os.ReadFile("/proc/meminfo"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "MemTotal:") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					_, _ = fmt.Sscanf(parts[1], "%d", &info.RAMTotalMB)
					info.RAMTotalMB /= 1024
				}
				break
			}
		}
	}
	if b, err := os.ReadFile("/proc/version"); err == nil {
		parts := strings.SplitN(string(b), " ", 4)
		if len(parts) >= 3 {
			info.Kernel = parts[2]
		}
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dataDir, &stat); err == nil {
		blockSize := int64(stat.Bsize)
		info.DiskTotalMB = (int64(stat.Blocks) * blockSize) / (1024 * 1024)
		info.DiskFreeMB = (int64(stat.Bavail) * blockSize) / (1024 * 1024)
	}
	return info
}
