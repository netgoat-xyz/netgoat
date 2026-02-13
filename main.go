package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"netgoat.xyz/agent/internal/anomaly"
	"netgoat.xyz/agent/internal/auth"
	"netgoat.xyz/agent/internal/cache"
	"netgoat.xyz/agent/internal/challenge"
	"netgoat.xyz/agent/internal/config"
	"netgoat.xyz/agent/internal/database"
	"netgoat.xyz/agent/internal/debugoverlay"
	"netgoat.xyz/agent/internal/honeypot"
	"netgoat.xyz/agent/internal/streaming"
	"netgoat.xyz/agent/internal/waf"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	cfg, err := config.Load("config.yml")
	if err != nil {
		log.Warn().Err(err).Msg("Could not read config.yml, using defaults")
		cfg = &config.Config{}
	} else {
		log.Info().Bool("debug_logs", cfg.DebugLogs).Bool("honeypot", cfg.Honeypot).Bool("auth_enabled", cfg.Auth.Enabled).Msg("Loaded configuration")
	}

	if err := os.MkdirAll("./database", 0755); err != nil {
		log.Fatal().Err(err).Msg("Failed to create database directory")
	}

	db, err := database.Init("./database/proxy.db")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize database")
	}
	defer db.Close()

	streamMgr := streaming.NewManager("./database/config-snapshot.json")
	defer streamMgr.Close()

	log.Info().Msg("Applying initial configuration from snapshot")
	initialSnap := streamMgr.GetSnapshot()
	applySnapshotToDB(db, initialSnap)

	apiURL := os.Getenv("API_STREAM_URL")
	if apiURL == "" && cfg.API.URL != "" {
		apiURL = cfg.API.URL
	}

	if apiURL != "" {
		apiKey := resolveAPIKey(cfg)
		if apiKey == "" {
			log.Warn().Msg("API_STREAM_URL set but no API_STREAM_KEY/API_KEY provided; external updates will likely be unauthorized")
		}
		go connectToAPIStream(streamMgr, apiURL, apiKey)
	} else {
		log.Info().Msg("No API_STREAM_URL configured, running in offline mode with local configuration")
	}

	// Subscribe to config updates and apply them
	go applyConfigUpdates(db, streamMgr)

	pages := buildErrorPageStore(cfg)

	// Initialize response cache
	var cacheStore *cache.Store
	if cfg.Cache.Enabled {
		ttl := time.Duration(ifZeroInt(cfg.Cache.TTLSeconds, 60)) * time.Second
		cacheStore = cache.NewStore(ttl, ifZeroInt(cfg.Cache.MaxEntries, 1024), ifZeroInt(cfg.Cache.MaxBodyBytes, 1<<20))
		log.Info().Dur("ttl", ttl).Int("max_entries", ifZeroInt(cfg.Cache.MaxEntries, 1024)).Int("max_body_bytes", ifZeroInt(cfg.Cache.MaxBodyBytes, 1<<20)).Msg("Response cache enabled")
	}

	var detector *anomaly.LocalDetector
	featureHeader := "X-GoatAI-Features"
	if cfg.Anomaly.FeatureHeader != "" {
		featureHeader = cfg.Anomaly.FeatureHeader
	}
	if cfg.Anomaly.Enabled {
		var err error
		detector, err = anomaly.NewLocalDetector(anomaly.LocalSettings{
			Enabled:      cfg.Anomaly.Enabled,
			Threshold:    ifZero(cfg.Anomaly.Threshold, 0.7),
			ModelPath:    ifEmpty(cfg.Anomaly.ModelPath, "ai/goatai.keras"),
			ScalerPath:   ifEmpty(cfg.Anomaly.ScalerPath, "ai/scaler.pkl"),
			PythonScript: ifEmpty(cfg.Anomaly.PythonScript, "ai/model_server.py"),
		})
		if err != nil {
			log.Warn().Err(err).Msg("Failed to initialize local anomaly detector")
			detector = nil
		} else {
			defer detector.Close()
			log.Info().Bool("enabled", true).Str("model", ifEmpty(cfg.Anomaly.ModelPath, "ai/goatai.keras")).Float64("threshold", ifZero(cfg.Anomaly.Threshold, 0.7)).Msg("Anomaly detection configured")
		}
	}

	// Initialize challenge store for dynamic error pages
	challengeStore := challenge.NewStore()
	log.Info().Msg("Challenge system initialized")

	// Challenge verification endpoint
	http.HandleFunc("/__netgoat/verify", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		r.ParseForm()
		challengeID := r.FormValue("challenge_id")
		answer := r.FormValue("answer")
		ip := getClientIP(r)

		if challengeStore.Verify(challengeID, answer, ip) {
			log.Info().Str("ip", ip).Str("challenge_id", challengeID).Msg("Challenge verified successfully")
			http.Redirect(w, r, r.Header.Get("Referer"), http.StatusFound)
		} else {
			log.Warn().Str("ip", ip).Str("challenge_id", challengeID).Msg("Challenge verification failed")
			http.Error(w, "Verification failed. Please try again.", http.StatusForbidden)
		}
	})

	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		auth.HandleLogin(w, r, db)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		// Initialize debug analysis info
		analysisInfo := &debugoverlay.AnalysisInfo{
			RequestID:      fmt.Sprintf("%d", time.Now().UnixNano()),
			Timestamp:      startTime,
			ClientIP:       getClientIP(r),
			Host:           r.Host,
			Path:           r.URL.Path,
			Method:         r.Method,
			RequestAllowed: true,
			AIEnabled:      detector != nil,
			AIThreshold:    ifZero(cfg.Anomaly.Threshold, 0.7),
		}

		if cfg.Auth.Enabled {
			authResult := auth.Check(r, db)
			if !authResult.Authenticated {
				if strings.Contains(r.Header.Get("Accept"), "application/json") {
					writeError(w, pages, challengeStore, r, http.StatusUnauthorized, "Unauthorized")
				} else {
					http.Redirect(w, r, "/login", http.StatusFound)
				}
				return
			}
			// Check if user requires zero-trust challenge
			if authResult.ZeroTrustReq {
				// TODO: Implement zero-trust challenge
				log.Debug().Str("user", authResult.Username).Msg("User requires zero-trust challenge")
			}
		}

		if cfg.Honeypot {
			if honeypot.Check(w, r) {
				log.Warn().Str("ip", r.RemoteAddr).Str("path", r.URL.Path).Msg("Honeypot triggered")
				return
			}
		}

		if detector != nil {
			csv := r.Header.Get(featureHeader)
			if csv == "" {
				csv = r.URL.Query().Get("goatai")
			}
			if csv != "" {
				analysisInfo.AIChecked = true
				aiStart := time.Now()
				ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
				label, score, derr := detector.PredictCSV(ctx, csv)
				cancel()
				analysisInfo.AIProcessingMs = time.Since(aiStart).Milliseconds()

				if derr != nil {
					log.Warn().Err(derr).Msg("Local anomaly detection error")
					analysisInfo.AIError = derr.Error()
				} else {
					analysisInfo.AILabel = label
					analysisInfo.AIScore = score
					log.Info().Str("label", label).Float64("score", score).Msg("Local anomaly prediction")
					if detector.IsAnomalous(label, score) {
						analysisInfo.AIBlocked = true
						analysisInfo.RequestAllowed = false
						analysisInfo.BlockReason = fmt.Sprintf("AI detected high-risk: %s (%.1f%%)", label, score*100)
						log.Warn().Str("label", label).Float64("score", score).Str("ip", r.RemoteAddr).Str("path", r.URL.Path).Msg("Blocked by local anomaly detector")
						writeError(w, pages, challengeStore, r, http.StatusForbidden, "Forbidden")
						return
					}
				}
			}
		}

		analysisInfo.WAFChecked = true
		block, ruleName := waf.Check(db, r, cfg.DebugLogs)
		if block {
			analysisInfo.WAFBlocked = true
			analysisInfo.WAFRuleName = ruleName
			analysisInfo.RequestAllowed = false
			analysisInfo.BlockReason = fmt.Sprintf("WAF rule triggered: %s", ruleName)
			log.Warn().Str("rule", ruleName).Str("ip", r.RemoteAddr).Str("host", r.Host).Msg("Request blocked by WAF")
			writeError(w, pages, challengeStore, r, http.StatusForbidden, "Forbidden")
			return
		}

		// Extract domain from Host header
		host := r.Host
		if idx := strings.LastIndex(host, ":"); idx > 0 {
			host = host[:idx] // Remove port
		}
		log.Debug().Str("host", host).Str("method", r.Method).Str("path", r.URL.Path).Msg("Processing request")

		// Try domain-based routing first, then path-based
		targetStr, err := database.GetTarget(db, host, r.URL.Path)
		if err != nil {
			log.Warn().Err(err).Str("host", host).Str("path", r.URL.Path).Msg("No route found for domain or path")
			writeError(w, pages, challengeStore, r, http.StatusNotFound, "No route found")
			return
		}
		if targetStr == "" {
			log.Warn().Str("host", host).Str("path", r.URL.Path).Msg("Route lookup returned empty target")
			writeError(w, pages, challengeStore, r, http.StatusNotFound, "No route found")
			return
		}

		log.Info().Str("host", host).Str("path", r.URL.Path).Str("target", targetStr).Str("method", r.Method).Msg("Route resolved")

		analysisInfo.TargetURL = targetStr

		targetURL, err := url.Parse(targetStr)
		if err != nil {
			log.Error().Err(err).Str("target", targetStr).Str("host", host).Msg("Invalid target URL in DB")
			writeError(w, pages, challengeStore, r, http.StatusInternalServerError, "Internal Server Error")
			return
		}

		if r.Header.Get("Upgrade") == "websocket" {
			log.Info().Str("client", r.RemoteAddr).Str("host", host).Msg("WebSocket upgrade detected")
		}

		// Cache lookup for safe methods
		isCacheable := cacheStore != nil && r.Method == http.MethodGet && r.Header.Get("Upgrade") == ""
		cacheKey := ""
		if isCacheable {
			cacheKey = cache.CacheKey(r)
			if ent := cacheStore.Get(cacheKey); ent != nil {
				analysisInfo.CacheHit = true
				for k, vals := range ent.Header() {
					for _, v := range vals {
						w.Header().Add(k, v)
					}
				}
				w.Header().Set("X-Cache", "HIT")

				// Inject debug overlay into cached response if enabled
				body := ent.Body()
				if cfg.DebugOverlay && strings.Contains(ent.Header().Get("Content-Type"), "text/html") {
					body = debugoverlay.InjectOverlay(body, analysisInfo)
				}

				w.WriteHeader(ent.Status())
				_, _ = w.Write(body)
				return
			}
		}

		proxy := httputil.NewSingleHostReverseProxy(targetURL)

		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			req.Host = targetURL.Host
		}

		proxy.ModifyResponse = func(res *http.Response) error {
			// Inject debug overlay for HTML responses if enabled
			if cfg.DebugOverlay && res.Header.Get("Content-Type") != "" && strings.Contains(res.Header.Get("Content-Type"), "text/html") {
				body, err := io.ReadAll(res.Body)
				if err != nil {
					return err
				}
				res.Body.Close()

				// Inject the overlay
				modifiedBody := debugoverlay.InjectOverlay(body, analysisInfo)

				// Update content length
				res.ContentLength = int64(len(modifiedBody))
				res.Header.Set("Content-Length", fmt.Sprintf("%d", len(modifiedBody)))

				res.Body = io.NopCloser(bytes.NewReader(modifiedBody))
			}

			// Handle caching
			if !isCacheable || cacheStore == nil {
				return nil
			}
			if res.StatusCode != http.StatusOK {
				return nil
			}

			// For cacheable responses, read and cache
			if !cfg.DebugOverlay || !strings.Contains(res.Header.Get("Content-Type"), "text/html") {
				body, err := io.ReadAll(res.Body)
				if err != nil {
					return err
				}
				res.Body.Close()
				res.Body = io.NopCloser(bytes.NewReader(body))
				cacheStore.Set(cacheKey, res.StatusCode, res.Header, body)
				res.Header.Set("X-Cache", "MISS")
			}

			return nil
		}

		proxy.ServeHTTP(w, r)
	})

	if cfg.SSL.Enabled {
		port := cfg.SSL.Port
		if port == "" {
			port = ":8443"
		}
		if err := http.ListenAndServeTLS(port, cfg.SSL.CertFile, cfg.SSL.KeyFile, nil); err != nil {
			log.Fatal().Err(err).Msg("Server failed")
		}
	} else {
		port := ":8080"
		log.Info().Str("port", port).Msg("Reverse proxy listening (HTTP)")
		if err := http.ListenAndServe(port, nil); err != nil {
			log.Fatal().Err(err).Msg("Server failed")
		}
	}
}

type errorPageStore struct {
	def    []byte
	byHost map[string][]byte
	byPath map[string][]byte
}

func buildErrorPageStore(cfg *config.Config) *errorPageStore {
	s := &errorPageStore{byHost: map[string][]byte{}, byPath: map[string][]byte{}}
	if cfg.CustomErrorPage != "" {
		if b, err := os.ReadFile(cfg.CustomErrorPage); err == nil {
			s.def = b
			log.Info().Str("path", cfg.CustomErrorPage).Int("bytes", len(b)).Msg("Loaded default error page")
		} else if !errors.Is(err, fs.ErrNotExist) {
			log.Warn().Err(err).Str("path", cfg.CustomErrorPage).Msg("Failed to read default error page")
		}
	}
	for host, p := range cfg.ErrorPages.Domain {
		if p == "" {
			continue
		}
		if b, err := os.ReadFile(p); err == nil {
			s.byHost[strings.ToLower(host)] = b
			log.Info().Str("host", host).Str("path", p).Msg("Loaded host error page")
		} else {
			log.Warn().Err(err).Str("host", host).Str("path", p).Msg("Failed to read host error page")
		}
	}
	for prefix, p := range cfg.ErrorPages.Path {
		if p == "" {
			continue
		}
		if b, err := os.ReadFile(p); err == nil {
			s.byPath[prefix] = b
			log.Info().Str("prefix", prefix).Str("path", p).Msg("Loaded path error page")
		} else {
			log.Warn().Err(err).Str("prefix", prefix).Str("path", p).Msg("Failed to read path error page")
		}
	}
	return s
}

func (s *errorPageStore) pick(r *http.Request) []byte {
	if s == nil {
		return nil
	}
	if b, ok := s.byHost[strings.ToLower(r.Host)]; ok && len(b) > 0 {
		return b
	}
	bestLen := -1
	var chosen []byte
	for prefix, b := range s.byPath {
		if strings.HasPrefix(r.URL.Path, prefix) {
			if l := len(prefix); l > bestLen && len(b) > 0 {
				bestLen = l
				chosen = b
			}
		}
	}
	if chosen != nil {
		return chosen
	}
	return s.def
}

func writeError(w http.ResponseWriter, pages *errorPageStore, store *challenge.Store, r *http.Request, status int, fallback string) {
	ip := getClientIP(r)
	userAgent := r.UserAgent()

	// Check if IP is already verified (passed a challenge recently)
	if store.IsVerified(ip) {
		// Serve static error page for verified users
		if p := pages.pick(r); len(p) > 0 && isHTML(p) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(status)
			_, _ = w.Write(p)
			return
		}
		http.Error(w, fallback, status)
		return
	}

	// Calculate suspicion and create challenge
	suspicion := challenge.CalculateSuspicion(userAgent, ip)
	challengeType := challenge.DetermineChallengeType(suspicion)

	log.Info().Str("ip", ip).Str("user_agent", userAgent).Int("suspicion", suspicion).Str("challenge_type", string(challengeType)).Msg("Generating dynamic error page")

	var ch *challenge.Challenge
	if challengeType != challenge.ChallengeNone {
		ch = store.Create(ip, userAgent, suspicion, challengeType)
	}

	// Render dynamic error page with challenge
	dynamicHTML := challenge.RenderDynamicErrorPage(ch, status, fallback)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(dynamicHTML))
}

func isHTML(b []byte) bool {
	trimmed := strings.TrimSpace(strings.ToLower(string(b)))
	return strings.HasPrefix(trimmed, "<") && (strings.Contains(trimmed, "<html") || strings.Contains(trimmed, "<body"))
}

func ifEmpty(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
func ifZero(f, def float64) float64 {
	if f == 0 {
		return def
	}
	return f
}

func ifZeroInt(v int, def int) int {
	if v == 0 {
		return def
	}
	return v
}

func resolveAPIKey(cfg *config.Config) string {
	if k := os.Getenv("API_STREAM_KEY"); k != "" {
		return k
	}
	if k := os.Getenv("API_KEY"); k != "" {
		return k
	}
	if cfg != nil && cfg.API.Key != "" {
		return cfg.API.Key
	}
	return ""
}

func getClientIP(r *http.Request) string {
	// Try X-Forwarded-For first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.Index(xff, ","); idx > 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	// Fall back to RemoteAddr
	if idx := strings.LastIndex(r.RemoteAddr, ":"); idx > 0 {
		return r.RemoteAddr[:idx]
	}
	return r.RemoteAddr
}

// connectToAPIStream connects to the external API WebSocket for config streaming.
func connectToAPIStream(mgr *streaming.Manager, apiURL, apiKey string) {
	log.Info().Str("url", apiURL).Msg("Starting config stream connection to external API")
	retryInterval := 5 * time.Second
	maxRetryInterval := 2 * time.Minute
	consecutiveFailures := 0

	for {
		log.Debug().Int("consecutive_failures", consecutiveFailures).Dur("retry_interval", retryInterval).Msg("Attempting API connection")

		err := connectWebSocket(mgr, apiURL, apiKey)

		if err != nil {
			consecutiveFailures++
			mgr.SetConnectionStatus(false, err)
			log.Warn().Err(err).Str("api_url", apiURL).Dur("retry_in", retryInterval).Int("failures", consecutiveFailures).Msg("Stream connection failed, will retry")
		} else {
			consecutiveFailures = 0
			retryInterval = 5 * time.Second
			mgr.SetConnectionStatus(true, nil)
			log.Info().Msg("Stream connection established successfully")
		}

		time.Sleep(retryInterval)
		if retryInterval < maxRetryInterval {
			retryInterval *= 2
			if retryInterval > maxRetryInterval {
				retryInterval = maxRetryInterval
			}
		}
	}
}

func connectWebSocket(mgr *streaming.Manager, apiURL, apiKey string) error {
	wsURL := strings.Replace(apiURL, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	if !strings.HasSuffix(wsURL, "/stream") {
		wsURL = strings.TrimSuffix(wsURL, "/") + "/stream"
	}

	log.Info().Str("url", wsURL).Msg("Attempting to connect to WebSocket stream")

	host := strings.TrimPrefix(wsURL, "ws://")
	host = strings.TrimPrefix(host, "wss://")
	if idx := strings.Index(host, "/"); idx > 0 {
		host = host[:idx]
	}

	log.Debug().Str("host", host).Msg("Testing TCP connection to host")
	conn, err := net.Dial("tcp", host)
	if err != nil {
		log.Error().Err(err).Str("host", host).Msg("TCP connection failed, falling back to HTTP polling")
		return pollAPIStream(mgr, apiURL, apiKey)
	}
	defer conn.Close()
	log.Debug().Str("host", host).Msg("TCP connection successful")

	return pollAPIStream(mgr, apiURL, apiKey)
}

// pollAPIStream uses HTTP polling as a WebSocket fallback
func pollAPIStream(mgr *streaming.Manager, apiURL, apiKey string) error {
	snapshotURL := strings.TrimSuffix(apiURL, "/") + "/snapshot"
	lastVersion := int64(-1)
	log.Info().Str("url", snapshotURL).Msg("Starting API polling connection")

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Track consecutive failures for circuit breaker pattern
	consecutiveFailures := 0
	maxConsecutiveFailures := 12 // 1 minute of failures before backing off more

	for range ticker.C {
		log.Debug().Str("url", snapshotURL).Msg("Polling snapshot...")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, snapshotURL, nil)
		if err != nil {
			cancel()
			log.Warn().Err(err).Msg("Failed to build snapshot request")
			consecutiveFailures++
			mgr.SetConnectionStatus(false, err)
			continue
		}

		if apiKey != "" {
			req.Header.Set("X-API-Key", apiKey)
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}

		resp, err := http.DefaultClient.Do(req)
		cancel()

		if err != nil {
			consecutiveFailures++
			mgr.SetConnectionStatus(false, err)

			if consecutiveFailures >= maxConsecutiveFailures {
				log.Warn().Err(err).Str("url", snapshotURL).Int("failures", consecutiveFailures).Msg("Multiple polling failures detected - continuing with cached config")
			} else {
				log.Debug().Err(err).Str("url", snapshotURL).Int("failures", consecutiveFailures).Msg("Polling failed")
			}
			continue
		}

		log.Debug().Int("status", resp.StatusCode).Msg("Poll response received")

		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			resp.Body.Close()
			consecutiveFailures++
			authErr := errors.New("unauthorized: check API key")
			mgr.SetConnectionStatus(false, authErr)

			if consecutiveFailures == 1 || consecutiveFailures%10 == 0 {
				log.Warn().Int("status", resp.StatusCode).Int("failures", consecutiveFailures).Msg("Snapshot request unauthorized; check API key")
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			consecutiveFailures++
			statusErr := fmt.Errorf("unexpected status: %d", resp.StatusCode)
			mgr.SetConnectionStatus(false, statusErr)
			log.Debug().Int("status", resp.StatusCode).Msg("Non-200 status code")
			continue
		}

		var snapshot struct {
			Version          int64                            `json:"version"`
			Routes           map[string]streaming.RouteData   `json:"routes"`
			WAFRules         map[string]streaming.WAFRuleData `json:"waf_rules"`
			Users            []streaming.UserData             `json:"users"`
			UserDomains      []streaming.UserDomainData       `json:"user_domains"`
			ZeroTrustEnabled bool                             `json:"zero_trust_enabled"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
			resp.Body.Close()
			consecutiveFailures++
			mgr.SetConnectionStatus(false, err)
			log.Warn().Err(err).Msg("Failed to decode snapshot JSON")
			continue
		}
		resp.Body.Close()

		// Successfully got a snapshot
		mgr.SetConnectionStatus(true, nil)

		if consecutiveFailures > 0 {
			log.Info().Int("previous_failures", consecutiveFailures).Msg("Connection recovered")
			consecutiveFailures = 0
		}

		if snapshot.Version > lastVersion {
			log.Info().Int64("new_version", snapshot.Version).Int64("last_version", lastVersion).Int("routes", len(snapshot.Routes)).Msg("New config version detected")
			lastVersion = snapshot.Version
			msg := &streaming.Message{
				Type:      "snapshot",
				Version:   snapshot.Version,
				Timestamp: time.Now(),
			}
			if data, err := json.Marshal(snapshot); err == nil {
				msg.Data = data
				if err := mgr.HandleMessage(msg); err != nil {
					log.Error().Err(err).Msg("Failed to handle message")
				} else {
					log.Info().Int64("version", snapshot.Version).Int("routes", len(snapshot.Routes)).Msg("Applied new config from API")
				}
			}
		} else {
			log.Debug().Int64("version", snapshot.Version).Msg("Config version unchanged")
		}
	}

	return nil
}

// streamFromAPI establishes a connection to the API streaming endpoint and handles NDJSON updates.
func streamFromAPI(mgr *streaming.Manager, apiURL string) error {
	resp, err := http.Get(apiURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("non-200 response from API")
	}

	decoder := json.NewDecoder(resp.Body)
	for decoder.More() {
		var msg streaming.Message
		if err := decoder.Decode(&msg); err != nil {
			return err
		}
		mgr.HandleMessage(&msg)
	}
	return nil
}

// applyConfigUpdates subscribes to config changes and applies them to the database.
func applyConfigUpdates(db *sql.DB, mgr *streaming.Manager) {
	ch := mgr.Subscribe()
	log.Info().Msg("Config update subscriber started")

	for snap := range ch {
		if snap == nil {
			log.Warn().Msg("Received nil snapshot")
			continue
		}
		applySnapshotToDB(db, snap)
	}
}

// applySnapshotToDB applies a config snapshot to the database
func applySnapshotToDB(db *sql.DB, snap *streaming.ConfigSnapshot) {
	log.Info().Int("route_count", len(snap.Routes)).Int("waf_rule_count", len(snap.WAFRules)).Msg("Processing config snapshot")

	// Apply routes - support both domain and path based
	routesApplied := 0
	routesFailed := 0
	for routeKey, route := range snap.Routes {
		log.Debug().Str("route_key", routeKey).Str("target", route.Target).Str("type", route.Type).Msg("Updating route")

		routeType := strings.ToLower(strings.TrimSpace(route.Type))
		if routeType == "" {
			routeType = "domain"
		}

		var domainVal interface{}
		var pathVal interface{}
		switch routeType {
		case "path":
			domainVal = nil
			pathVal = routeKey
		case "domain":
			domainVal = routeKey
			pathVal = nil
		default:
			log.Warn().Str("route_key", routeKey).Str("type", route.Type).Msg("Unknown route type; skipping")
			routesFailed++
			continue
		}

		_, err := db.Exec(
			`INSERT INTO routes (route_type, domain, path_prefix, target_url, certificate_pem, private_key_pem, active) VALUES (?, ?, ?, ?, ?, ?, 1)
				 ON CONFLICT(route_type, domain, path_prefix) DO UPDATE SET target_url=excluded.target_url, certificate_pem=excluded.certificate_pem, private_key_pem=excluded.private_key_pem, updated_at=CURRENT_TIMESTAMP`,
			routeType, domainVal, pathVal, route.Target, route.CertificatePEM, route.PrivateKeyPEM)
		if err != nil {
			log.Error().Err(err).Str("route", routeKey).Str("target", route.Target).Str("type", routeType).Msg("Failed to update route")
			routesFailed++
		} else {
			log.Info().Str("route", routeKey).Str("target", route.Target).Str("type", routeType).Msg("Route updated")
			routesApplied++
		}
	}

	// Apply WAF rules
	rulesApplied := 0
	rulesFailed := 0
	for _, rule := range snap.WAFRules {
		log.Debug().Str("name", rule.Name).Str("expression", rule.Expression).Msg("Updating WAF rule")
		_, err := db.Exec(
			`INSERT INTO waf_rules (name, expression, action, priority) VALUES (?, ?, ?, ?)
				 ON CONFLICT(name) DO UPDATE SET expression=excluded.expression, action=excluded.action, priority=excluded.priority`,
			rule.Name, rule.Expression, rule.Action, rule.Priority)
		if err != nil {
			log.Error().Err(err).Str("name", rule.Name).Msg("Failed to update WAF rule")
			rulesFailed++
		} else {
			log.Info().Str("name", rule.Name).Msg("WAF rule updated")
			rulesApplied++
		}
	}

	// Apply Users
	usersApplied := 0
	usersFailed := 0
	for _, user := range snap.Users {
		_, err := db.Exec(
			`INSERT INTO users (username, password_hash, email, zero_trust_enabled) VALUES (?, ?, ?, ?)
				 ON CONFLICT(username) DO UPDATE SET password_hash=excluded.password_hash, email=excluded.email, zero_trust_enabled=excluded.zero_trust_enabled`,
			user.Username, user.PasswordHash, user.Email, user.ZeroTrustEnabled)
		if err != nil {
			log.Error().Err(err).Str("username", user.Username).Msg("Failed to update user")
			usersFailed++
		} else {
			usersApplied++
		}
	}

	// Apply User Domains
	userDomainsApplied := 0
	userDomainsFailed := 0
	for _, ud := range snap.UserDomains {
		// Get user ID
		var userID int
		err := db.QueryRow("SELECT id FROM users WHERE username = ?", ud.Username).Scan(&userID)
		if err != nil {
			log.Error().Err(err).Str("username", ud.Username).Str("domain", ud.Domain).Msg("Failed to find user for domain")
			userDomainsFailed++
			continue
		}

		_, err = db.Exec(
			`INSERT INTO user_proxy_records (user_id, domain, target_url, active) VALUES (?, ?, ?, ?)
				 ON CONFLICT(user_id, domain) DO UPDATE SET target_url=excluded.target_url, active=excluded.active, updated_at=CURRENT_TIMESTAMP`,
			userID, ud.Domain, ud.TargetURL, ud.Active)
		if err != nil {
			log.Error().Err(err).Str("domain", ud.Domain).Msg("Failed to update user domain")
			userDomainsFailed++
		} else {
			userDomainsApplied++
		}
	}

	// Apply Zero Trust Global Setting
	_, err := db.Exec(`INSERT INTO zero_trust_settings (key, value) VALUES ('enabled', ?) 
		ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=CURRENT_TIMESTAMP`,
		fmt.Sprintf("%v", snap.ZeroTrustEnabled))
	if err != nil {
		log.Error().Err(err).Msg("Failed to update zero trust settings")
	}

	log.Info().Int("routes_applied", routesApplied).Int("routes_failed", routesFailed).
		Int("rules_applied", rulesApplied).Int("rules_failed", rulesFailed).
		Int("users_applied", usersApplied).Int("user_domains_applied", userDomainsApplied).
		Msg("Snapshot applied")
}
