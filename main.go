package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"netgoat.xyz/agent/internal/anomaly"
	"netgoat.xyz/agent/internal/auth"
	"netgoat.xyz/agent/internal/balancer"
	"netgoat.xyz/agent/internal/cache"
	"netgoat.xyz/agent/internal/challenge"
	"netgoat.xyz/agent/internal/config"
	"netgoat.xyz/agent/internal/database"
	"netgoat.xyz/agent/internal/debugoverlay"
	"netgoat.xyz/agent/internal/health"
	"netgoat.xyz/agent/internal/honeypot"
	"netgoat.xyz/agent/internal/koda2"
	"netgoat.xyz/agent/internal/koda_waf"
	"netgoat.xyz/agent/internal/metrics"
	"netgoat.xyz/agent/internal/modeldl"
	"netgoat.xyz/agent/internal/streaming"
	"netgoat.xyz/agent/internal/telemetry"
	"netgoat.xyz/agent/internal/traffic"
	"netgoat.xyz/agent/internal/waf"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	loadEnvFromFile(".env")
	if k := os.Getenv("DiamondKey"); k != "" {
		log.Info().Int("diamond_key_len", len(k)).Msg("DiamondKey loaded from environment")
	} else {
		log.Warn().Msg("DiamondKey not set in environment")
	}

	cfg, err := config.Load("config.yml")
	if err != nil {
		log.Warn().Err(err).Msg("Could not read config.yml, using defaults")
		cfg = &config.Config{}
	} else {
		log.Info().Bool("debug_logs", cfg.DebugLogs).Bool("honeypot", cfg.Honeypot).Bool("auth_enabled", cfg.Auth.Enabled).Msg("Loaded configuration")
	}

	dbPath := cfg.DatabasePath()
	standbyPath := cfg.DatabaseStandbyPath()
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		log.Fatal().Err(err).Msg("Failed to create database directory")
	}
	// Keep the historical snapshot location stable so custom database.path values
	// do not orphan an existing ./database/config-snapshot.json.
	const snapshotPath = "./database/config-snapshot.json"
	if err := os.MkdirAll(filepath.Dir(snapshotPath), 0755); err != nil {
		log.Fatal().Err(err).Msg("Failed to create config snapshot directory")
	}

	db, recovered, err := database.OpenWithFailover(dbPath, standbyPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize database")
	}
	defer db.Close()
	if recovered {
		log.Info().Str("primary", dbPath).Str("standby", standbyPath).Msg("Database recovered from standby")
	}

	streamMgr := streaming.NewManager(snapshotPath)
	defer streamMgr.Close()

	log.Info().Msg("Applying initial configuration from snapshot")
	localSnap := localConfigSnapshot(cfg)
	if localSnap.RoutesConfigured {
		if err := applySnapshotToDB(db, localSnap); err != nil {
			log.Error().Err(err).Msg("Failed to apply local routes")
		}
	}
	initialSnap := streamMgr.GetSnapshot()
	if snapshotHasContent(initialSnap) {
		if err := applySnapshotToDB(db, mergeConfigSnapshots(localSnap, initialSnap)); err != nil {
			log.Error().Err(err).Msg("Failed to apply recovered configuration")
		}
		applyAgentConfigToConfig(cfg, initialSnap.AgentConfig)
	}
	wafEngine := waf.NewEngine()
	if err := wafEngine.Reload(db); err != nil {
		log.Error().Err(err).Msg("Failed to compile initial WAF rules")
	}

	if backupEvery := cfg.DatabaseBackupIntervalSeconds(); backupEvery > 0 {
		startDatabaseBackupLoop(db, standbyPath, time.Duration(backupEvery)*time.Second)
		log.Info().Int("interval_seconds", backupEvery).Str("standby", standbyPath).Msg("Periodic database standby backups enabled")
	}

	healthInterval := time.Duration(ifZeroInt(cfg.Health.IntervalSeconds, 10)) * time.Second
	healthTimeout := time.Duration(ifZeroInt(cfg.Health.TimeoutSeconds, 3)) * time.Second
	healthPath := ifEmpty(cfg.Health.Path, "/")
	healthWorker := health.NewWorker(healthInterval, healthTimeout, healthPath)
	healthChecksEnabled := cfg.HealthChecksEnabled()
	if healthChecksEnabled {
		syncHealthTargets(db, healthWorker)
		healthWorker.Start(context.Background())
		log.Info().Dur("interval", healthInterval).Dur("timeout", healthTimeout).Str("path", healthPath).Msg("Upstream health checks enabled")
	} else {
		log.Info().Msg("Upstream health checks disabled")
	}

	proxyTransport := newStableProxyTransport()

	lb := balancer.New(healthWorker)
	proxyHandler := balancer.NewProxyHandler(lb, proxyTransport)

	apiURL := os.Getenv("API_STREAM_URL")
	if apiURL == "" && cfg.API.URL != "" {
		apiURL = cfg.API.URL
	}

	if apiURL != "" {
		apiKey := resolveAPIKey(cfg)
		if apiKey == "" {
			log.Warn().Msg("API_STREAM_URL set but no API_STREAM_KEY/API_KEY provided; external updates will likely be unauthorized")
		}
		if agentConfig, err := fetchAgentConfig(apiURL, apiKey); err != nil {
			log.Warn().Err(err).Msg("Could not fetch startup agent config from stream-server")
		} else {
			applyAgentConfigToConfig(cfg, agentConfig)
			log.Info().Msg("Applied startup agent config from stream-server")
		}
		go connectToAPIStream(streamMgr, apiURL, apiKey, streamSettingsFromConfig(cfg))
	} else {
		log.Info().Msg("No API_STREAM_URL configured, running in offline mode with local configuration")
	}

	go applyConfigUpdates(db, streamMgr, healthWorker, healthChecksEnabled, localSnap, wafEngine)

	pages := buildErrorPageStore(cfg)

	var cacheStore *cache.Store
	if cfg.Cache.Enabled {
		ttl := time.Duration(ifZeroInt(cfg.Cache.TTLSeconds, 60)) * time.Second
		cacheStore = cache.NewStore(ttl, ifZeroInt(cfg.Cache.MaxEntries, 1024), ifZeroInt(cfg.Cache.MaxBodyBytes, 1<<20))
		log.Info().Dur("ttl", ttl).Int("max_entries", ifZeroInt(cfg.Cache.MaxEntries, 1024)).Int("max_body_bytes", ifZeroInt(cfg.Cache.MaxBodyBytes, 1<<20)).Msg("Response cache enabled")
	}

	var rateLimiter *traffic.RateLimiter
	if cfg.RateLimit.Enabled {
		rateLimiter = traffic.NewRateLimiter(cfg.RateLimit.RequestsPerMinute, cfg.RateLimit.Burst)
		log.Info().Int("requests_per_minute", ifZeroInt(cfg.RateLimit.RequestsPerMinute, 60)).Int("burst", ifZeroInt(cfg.RateLimit.Burst, cfg.RateLimit.RequestsPerMinute)).Str("key", ifEmpty(cfg.RateLimit.Key, "ip")).Msg("Rate limiting enabled")
	}

	var requestQueue *traffic.Queue
	if cfg.RequestQueue.Enabled {
		timeout := time.Duration(ifZeroInt(cfg.RequestQueue.TimeoutSeconds, 5)) * time.Second
		requestQueue = traffic.NewQueue(cfg.RequestQueue.MaxConcurrent, cfg.RequestQueue.MaxQueued, timeout)
		log.Info().Int("max_concurrent", ifZeroInt(cfg.RequestQueue.MaxConcurrent, 1)).Int("max_queued", cfg.RequestQueue.MaxQueued).Dur("timeout", timeout).Msg("Request queue enabled")
	}

	var bandwidthLimiter *traffic.BandwidthLimiter
	if cfg.Bandwidth.Enabled {
		bandwidthLimiter = traffic.NewBandwidthLimiter(cfg.Bandwidth.BytesPerSecond, cfg.Bandwidth.BurstBytes)
		log.Info().Int("bytes_per_second", ifZeroInt(cfg.Bandwidth.BytesPerSecond, 1<<20)).Int("burst_bytes", ifZeroInt(cfg.Bandwidth.BurstBytes, cfg.Bandwidth.BytesPerSecond)).Str("key", ifEmpty(cfg.Bandwidth.Key, "ip")).Msg("Bandwidth limiting enabled")
	}

	var metricsRecorder *metrics.Recorder
	if cfg.Metrics.Enabled {
		metricsRecorder = metrics.NewRecorder()
		metricsPath := ifEmpty(cfg.Metrics.Path, "/__netgoat/metrics")
		http.HandleFunc(metricsPath, metricsRecorder.ServeJSON)
		http.HandleFunc(metricsPath+".prom", metricsRecorder.ServePrometheus)
		log.Info().Str("path", metricsPath).Str("prometheus_path", metricsPath+".prom").Msg("Metrics endpoint enabled")
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

	var kodaWafDetector *koda_waf.Detector
	if cfg.KodaWaf.Enabled {
		modelPath := ifEmpty(cfg.KodaWaf.ModelPath, "ai/smart_waf_model.pkl")
		scalerPath := ifEmpty(cfg.KodaWaf.ScalerPath, "ai/model_features.pkl")
		modeldl.EnsureDownloaded([]modeldl.ModelFile{
			{
				URL:      "https://huggingface.co/netgoat-ai/koda-waf/resolve/main/smart_waf_model.pkl",
				DestPath: modelPath,
				Label:    "koda-waf model",
			},
			{
				URL:      "https://huggingface.co/netgoat-ai/koda-waf/resolve/main/model_features.pkl",
				DestPath: scalerPath,
				Label:    "koda-waf features",
			},
		})

		var err error
		kodaWafDetector, err = koda_waf.NewDetector(koda_waf.Settings{
			Enabled:      cfg.KodaWaf.Enabled,
			Threshold:    ifZero(cfg.KodaWaf.Threshold, 0.7),
			ModelPath:    modelPath,
			ScalerPath:   scalerPath,
			PythonScript: ifEmpty(cfg.KodaWaf.PythonScript, "ai/koda_waf_server.py"),
		})
		if err != nil {
			log.Warn().Err(err).Msg("Failed to initialize Koda-Waf detector")
			kodaWafDetector = nil
		} else {
			defer kodaWafDetector.Close()
			log.Info().Bool("enabled", true).Str("model", modelPath).Float64("threshold", ifZero(cfg.KodaWaf.Threshold, 0.7)).Msg("Koda-Waf detection configured")
		}
	}

	var koda2Detector *koda2.Detector
	if cfg.Koda2.Enabled {
		modelPath := ifEmpty(cfg.Koda2.ModelPath, "ai/koda2.keras")
		scalerPath := ifEmpty(cfg.Koda2.ScalerPath, "ai/koda2_scaler.pkl")
		modeldl.EnsureDownloaded([]modeldl.ModelFile{
			{
				URL:      "https://huggingface.co/netgoat-ai/koda-2/resolve/main/model.keras",
				DestPath: modelPath,
				Label:    "koda-2 model",
			},
			{
				URL:      "https://huggingface.co/netgoat-ai/koda-2/resolve/main/scaler.pkl",
				DestPath: scalerPath,
				Label:    "koda-2 scaler",
			},
		})

		var err error
		koda2Detector, err = koda2.NewDetector(koda2.Settings{
			Enabled:      cfg.Koda2.Enabled,
			Threshold:    ifZero(cfg.Koda2.Threshold, 0.7),
			ModelPath:    modelPath,
			ScalerPath:   scalerPath,
			PythonScript: ifEmpty(cfg.Koda2.PythonScript, "ai/koda2_server.py"),
		})
		if err != nil {
			log.Warn().Err(err).Msg("Failed to initialize Koda-2 detector")
			koda2Detector = nil
		} else {
			defer koda2Detector.Close()
			log.Info().Bool("enabled", true).Str("model", modelPath).Float64("threshold", ifZero(cfg.Koda2.Threshold, 0.7)).Msg("Koda-2 detection configured")
		}
	}

	challengeStore := challenge.NewStore()
	log.Info().Msg("Challenge system initialized")

	telemetryClient := telemetry.NewClient(telemetry.Config{
		Enabled:   cfg.Telemetry.Enabled,
		Endpoint:  cfg.Telemetry.Endpoint,
		IngestKey: cfg.Telemetry.IngestKey,
		DataDir:   "./database",
		Interval:  time.Duration(ifZeroInt(cfg.Telemetry.IntervalSeconds, 300)) * time.Second,
		StatsFunc: func() telemetry.AppStats {
			stats := telemetry.AppStats{}
			db.QueryRow("SELECT COUNT(*) FROM routes WHERE active = 1").Scan(&stats.Routes)
			db.QueryRow("SELECT COUNT(*) FROM users").Scan(&stats.Users)
			if metricsRecorder != nil {
				snap := metricsRecorder.Snapshot()
				stats.Requests = snap.Requests
				stats.Blocked = snap.Blocked
				stats.ProxyErrors = snap.ProxyErrors
				stats.TotalErrors = snap.Blocked + snap.ProxyErrors
				stats.AvgLatency = snap.AverageLatencyMs
				stats.StatusCodes = snap.StatusCodes
				stats.BlockReasons = snap.BlockReasons
				stats.ErrorStatusCodes = snap.ErrorStatusCodes
				stats.RecentErrors = make([]telemetry.ErrorInfo, 0, len(snap.RecentErrors))
				for _, info := range snap.RecentErrors {
					stats.RecentErrors = append(stats.RecentErrors, telemetry.ErrorInfo{
						Kind:     info.Kind,
						Message:  info.Message,
						Count:    info.Count,
						LastSeen: info.LastSeen,
					})
				}
			}
			return stats
		},
	})
	telemetryClient.Start()
	defer telemetryClient.Stop()

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
			redirectTo := r.Header.Get("Referer")
			if redirectTo == "" {
				redirectTo = "/"
			}
			http.Redirect(w, r, redirectTo, http.StatusFound)
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
		if metricsRecorder != nil {
			metricsRecorder.RecordRequest()
			metricWriter := metrics.WrapResponseWriter(w)
			w = metricWriter
			defer func() {
				metricsRecorder.RecordResponse(metricWriter.Status(), metricWriter.BytesWritten(), time.Since(startTime))
			}()
		}
		if bandwidthLimiter != nil {
			key := rateLimitKey(r, cfg.Bandwidth.Key)
			r.Body = traffic.WrapReadCloser(r.Body, bandwidthLimiter, key+":in", r.Context())
			w = traffic.WrapResponseWriter(w, bandwidthLimiter, key+":out", r.Context())
		}

		analysisInfo := &debugoverlay.AnalysisInfo{
			RequestID:        fmt.Sprintf("%d", time.Now().UnixNano()),
			Timestamp:        startTime,
			ClientIP:         getClientIP(r),
			Host:             r.Host,
			Path:             r.URL.Path,
			Method:           r.Method,
			RequestAllowed:   true,
			AIEnabled:        detector != nil,
			AIThreshold:      ifZero(cfg.Anomaly.Threshold, 0.7),
			KodaWafEnabled:   kodaWafDetector != nil,
			KodaWafThreshold: ifZero(cfg.KodaWaf.Threshold, 0.7),
			Koda2Enabled:     koda2Detector != nil,
			Koda2Threshold:   ifZero(cfg.Koda2.Threshold, 0.7),
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
			if auth.RequireZeroTrustChallenge(authResult, database.IsZeroTrustEnabled(db), challengeStore.IsVerified(getClientIP(r))) {
				analysisInfo.RequestAllowed = false
				analysisInfo.BlockReason = "zero-trust verification required"
				recordBlocked(metricsRecorder, "zero-trust")
				log.Info().Str("user", authResult.Username).Str("ip", getClientIP(r)).Msg("Zero-trust challenge required")
				writeZeroTrustChallenge(w, challengeStore, r)
				return
			}
		}

		if cfg.Honeypot {
			if honeypot.Check(w, r) {
				log.Warn().Str("ip", r.RemoteAddr).Str("path", r.URL.Path).Msg("Honeypot triggered")
				return
			}
		}

		if rateLimiter != nil && !rateLimiter.Allow(rateLimitKey(r, cfg.RateLimit.Key)) {
			analysisInfo.RequestAllowed = false
			analysisInfo.BlockReason = "rate limit exceeded"
			recordBlocked(metricsRecorder, "rate-limit")
			log.Warn().Str("ip", getClientIP(r)).Str("host", r.Host).Str("path", r.URL.Path).Msg("Request rate limited")
			writeError(w, pages, challengeStore, r, http.StatusTooManyRequests, "Too Many Requests")
			return
		}

		if requestQueue != nil {
			release, err := requestQueue.Acquire(r.Context())
			if err != nil {
				analysisInfo.RequestAllowed = false
				analysisInfo.BlockReason = "request queue full"
				status := http.StatusServiceUnavailable
				if errors.Is(err, traffic.ErrQueueFull) {
					status = http.StatusTooManyRequests
				}
				recordBlocked(metricsRecorder, "request-queue")
				log.Warn().Err(err).Str("ip", getClientIP(r)).Str("host", r.Host).Str("path", r.URL.Path).Msg("Request rejected by queue")
				writeError(w, pages, challengeStore, r, status, http.StatusText(status))
				return
			}
			defer release()
		}

		if kodaWafDetector != nil {
			kodaWafHeader := ifEmpty(cfg.KodaWaf.FeatureHeader, "X-KodaWaf-Features")
			csv := r.Header.Get(kodaWafHeader)
			if csv == "" {
				csv = r.URL.Query().Get("kodawaf")
			}
			if csv != "" {
				analysisInfo.KodaWafChecked = true
				kwStart := time.Now()
				ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
				pred, kerr := kodaWafDetector.Predict(ctx, csv)
				cancel()
				analysisInfo.KodaWafProcessingMs = time.Since(kwStart).Milliseconds()

				if kerr != nil {
					log.Warn().Err(kerr).Msg("Koda-Waf detection error")
					analysisInfo.KodaWafError = kerr.Error()
				} else {
					analysisInfo.KodaWafLabel = pred.Label
					analysisInfo.KodaWafScore = pred.Score
					analysisInfo.KodaWafAttackType = pred.AttackType
					log.Info().Str("label", pred.Label).Float64("score", pred.Score).Str("attack_type", pred.AttackType).Msg("Koda-Waf prediction")
					if kodaWafDetector.IsBlocked(pred) {
						analysisInfo.KodaWafBlocked = true
						analysisInfo.RequestAllowed = false
						analysisInfo.BlockReason = fmt.Sprintf("Koda-Waf blocked: %s (%.1f%%)", pred.Label, pred.Score*100)
						recordBlocked(metricsRecorder, "koda-waf")
						log.Warn().Str("label", pred.Label).Float64("score", pred.Score).Str("ip", r.RemoteAddr).Str("path", r.URL.Path).Msg("Blocked by Koda-Waf")
						writeError(w, pages, challengeStore, r, http.StatusForbidden, "Forbidden")
						return
					}
				}
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
						recordBlocked(metricsRecorder, "goatai")
						log.Warn().Str("label", label).Float64("score", score).Str("ip", r.RemoteAddr).Str("path", r.URL.Path).Msg("Blocked by local anomaly detector")
						writeError(w, pages, challengeStore, r, http.StatusForbidden, "Forbidden")
						return
					}
				}
			}
		}

		if koda2Detector != nil {
			koda2Header := ifEmpty(cfg.Koda2.FeatureHeader, "X-Koda2-Features")
			csv := r.Header.Get(koda2Header)
			if csv == "" {
				csv = r.URL.Query().Get("koda2")
			}
			if csv != "" {
				analysisInfo.Koda2Checked = true
				k2Start := time.Now()
				ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
				pred, kerr := koda2Detector.Predict(ctx, csv)
				cancel()
				analysisInfo.Koda2ProcessingMs = time.Since(k2Start).Milliseconds()

				if kerr != nil {
					log.Warn().Err(kerr).Msg("Koda-2 detection error")
					analysisInfo.Koda2Error = kerr.Error()
				} else {
					analysisInfo.Koda2Label = pred.Label
					analysisInfo.Koda2Score = pred.Score
					log.Info().Str("label", pred.Label).Float64("score", pred.Score).Msg("Koda-2 prediction")
					if koda2Detector.IsAnomalous(pred) {
						analysisInfo.Koda2Blocked = true
						analysisInfo.RequestAllowed = false
						analysisInfo.BlockReason = fmt.Sprintf("Koda-2 detected anomaly: %s (%.1f%%)", pred.Label, pred.Score*100)
						recordBlocked(metricsRecorder, "koda-2")
						log.Warn().Str("label", pred.Label).Float64("score", pred.Score).Str("ip", r.RemoteAddr).Str("path", r.URL.Path).Msg("Blocked by Koda-2")
						writeError(w, pages, challengeStore, r, http.StatusForbidden, "Forbidden")
						return
					}
				}
			}
		}

		analysisInfo.WAFChecked = true
		block, ruleName := wafEngine.Check(r, cfg.DebugLogs)
		if block {
			analysisInfo.WAFBlocked = true
			analysisInfo.WAFRuleName = ruleName
			analysisInfo.RequestAllowed = false
			analysisInfo.BlockReason = fmt.Sprintf("WAF rule triggered: %s", ruleName)
			recordBlocked(metricsRecorder, "waf:"+ruleName)
			log.Warn().Str("rule", ruleName).Str("ip", r.RemoteAddr).Str("host", r.Host).Msg("Request blocked by WAF")
			writeError(w, pages, challengeStore, r, http.StatusForbidden, "Forbidden")
			return
		}

		host := r.Host
		if idx := strings.LastIndex(host, ":"); idx > 0 {
			host = host[:idx]
		}
		log.Debug().Str("host", host).Str("method", r.Method).Str("path", r.URL.Path).Msg("Processing request")

		routeMatch, err := database.GetRouteTargets(db, host, r.URL.Path)
		if err != nil {
			log.Warn().Err(err).Str("host", host).Str("path", r.URL.Path).Msg("No route found for domain or path")
			writeError(w, pages, challengeStore, r, http.StatusNotFound, "No route found")
			return
		}
		if len(routeMatch.Targets) == 0 {
			log.Warn().Str("host", host).Str("path", r.URL.Path).Msg("Route lookup returned no targets")
			writeError(w, pages, challengeStore, r, http.StatusNotFound, "No route found")
			return
		}

		targetURLs := make([]string, 0, len(routeMatch.Targets))
		for _, t := range routeMatch.Targets {
			targetURLs = append(targetURLs, t.URL)
		}
		primaryTarget := targetURLs[0]

		log.Info().Str("host", host).Str("path", r.URL.Path).Str("target", primaryTarget).Int("targets", len(targetURLs)).Str("method", r.Method).Msg("Route resolved")

		analysisInfo.TargetURL = primaryTarget

		if r.Header.Get("Upgrade") == "websocket" {
			log.Info().Str("client", r.RemoteAddr).Str("host", host).Msg("WebSocket upgrade detected")
		}

		isCacheable := isRequestCacheableForSharedStore(cacheStore, r)
		cacheKey := ""
		if isCacheable {
			cacheKey = cache.CacheKey(r)
			if ent := cacheStore.Get(cacheKey); ent != nil {
				analysisInfo.CacheHit = true
				if metricsRecorder != nil {
					metricsRecorder.RecordCacheHit()
				}
				for k, vals := range ent.Header() {
					for _, v := range vals {
						w.Header().Add(k, v)
					}
				}
				w.Header().Set("X-Cache", "HIT")

				body := ent.Body()
				if cfg.DebugOverlay && strings.Contains(ent.Header().Get("Content-Type"), "text/html") {
					body = debugoverlay.InjectOverlay(body, analysisInfo)
				}

				w.WriteHeader(ent.Status())
				_, _ = w.Write(body)
				return
			}
		}

		if err := proxyHandler.Serve(w, r, routeMatch.RouteKey, targetURLs, func(res *http.Response) error {
			if cfg.DebugOverlay && shouldInjectOverlay(res) {
				body, err := io.ReadAll(res.Body)
				if err != nil {
					return err
				}
				_ = res.Body.Close()

				modifiedBody := debugoverlay.InjectOverlay(body, analysisInfo)

				res.ContentLength = int64(len(modifiedBody))
				res.Header.Set("Content-Length", fmt.Sprintf("%d", len(modifiedBody)))
				res.Header.Del("Transfer-Encoding")
				res.Body = io.NopCloser(bytes.NewReader(modifiedBody))
			}

			if !isCacheable || cacheStore == nil {
				return nil
			}
			if res.StatusCode != http.StatusOK {
				return nil
			}
			if !isSharedCacheableResponse(res) {
				return nil
			}
			responseTTL, ok := sharedCacheTTL(res.Header.Get("Cache-Control"), cacheStore.TTL())
			if !ok {
				return nil
			}

			if !cfg.DebugOverlay || !strings.Contains(res.Header.Get("Content-Type"), "text/html") {
				status := res.StatusCode
				header := res.Header.Clone()
				res.Body = cache.CaptureOnEOF(res.Body, cacheStore.MaxBodyBytes(), func(body []byte) {
					cacheStore.SetWithTTL(cacheKey, status, header, body, responseTTL)
				})
				res.Header.Set("X-Cache", "MISS")
			}

			return nil
		}); err != nil {
			status := http.StatusBadGateway
			if isTimeoutErr(err) {
				status = http.StatusGatewayTimeout
			}
			if metricsRecorder != nil {
				metricsRecorder.RecordProxyError(err)
			}
			log.Error().Err(err).Int("status", status).Str("host", host).Str("path", r.URL.Path).Msg("Failed to proxy request to upstream")
			writeError(w, pages, challengeStore, r, status, http.StatusText(status))
		}
	})

	server := &http.Server{
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       90 * time.Second,
		Handler:           nil,
	}

	if cfg.SSL.Enabled {
		port := cfg.SSL.Port
		if port == "" {
			port = ":8443"
		}
		server.Addr = port
		log.Info().Str("port", port).Msg("Reverse proxy listening (HTTPS)")
		if err := server.ListenAndServeTLS(cfg.SSL.CertFile, cfg.SSL.KeyFile); err != nil {
			log.Fatal().Err(err).Msg("Server failed")
		}
	} else {
		port := ":8080"
		server.Addr = port
		log.Info().Str("port", port).Msg("Reverse proxy listening (HTTP)")
		if err := server.ListenAndServe(); err != nil {
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

	if store.IsVerified(ip) {
		if p := pages.pick(r); len(p) > 0 && isHTML(p) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(status)
			_, _ = w.Write(p)
			return
		}
		http.Error(w, fallback, status)
		return
	}

	suspicion := challenge.CalculateSuspicion(userAgent, ip)
	challengeType := challenge.DetermineChallengeType(suspicion)

	log.Info().Str("ip", ip).Str("user_agent", userAgent).Int("suspicion", suspicion).Str("challenge_type", string(challengeType)).Msg("Generating dynamic error page")

	var ch *challenge.Challenge
	if challengeType != challenge.ChallengeNone {
		ch = store.Create(ip, userAgent, suspicion, challengeType)
	}

	dynamicHTML := challenge.RenderDynamicErrorPage(ch, status, fallback)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(dynamicHTML))
}

func writeZeroTrustChallenge(w http.ResponseWriter, store *challenge.Store, r *http.Request) {
	ip := getClientIP(r)
	ch := store.Create(ip, r.UserAgent(), 50, challenge.ChallengeText)
	html := challenge.RenderDynamicErrorPage(ch, http.StatusForbidden, "Zero-trust verification required")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(html))
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
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.Index(xff, ","); idx > 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	if idx := strings.LastIndex(r.RemoteAddr, ":"); idx > 0 {
		return r.RemoteAddr[:idx]
	}
	return r.RemoteAddr
}

func rateLimitKey(r *http.Request, keyMode string) string {
	switch strings.ToLower(strings.TrimSpace(keyMode)) {
	case "host":
		return r.Host
	case "route":
		return r.Host + "|" + r.URL.Path
	case "global":
		return "global"
	default:
		return getClientIP(r)
	}
}

func recordBlocked(rec *metrics.Recorder, reason string) {
	if rec != nil {
		rec.RecordBlocked(reason)
	}
}

func newStableProxyTransport() *http.Transport {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   50,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
	}
}

func isTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var nerr net.Error
	return errors.As(err, &nerr) && nerr.Timeout()
}

func shouldInjectOverlay(res *http.Response) bool {
	if res == nil {
		return false
	}
	ct := res.Header.Get("Content-Type")
	if ct == "" || !strings.Contains(strings.ToLower(ct), "text/html") {
		return false
	}
	if enc := strings.ToLower(strings.TrimSpace(res.Header.Get("Content-Encoding"))); enc != "" && enc != "identity" {
		return false
	}
	const maxInjectBytes = 256 * 1024
	if res.ContentLength < 0 || res.ContentLength > maxInjectBytes {
		return false
	}
	return true
}

func loadEnvFromFile(path string) {
	candidates := []string{path}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), ".env"))
	}
	candidates = append(candidates, filepath.Join("PinkDiamond", ".env"))

	var data []byte
	var err error
	for _, p := range candidates {
		log.Debug().Str("env_path", p).Msg("Trying .env candidate")
		data, err = os.ReadFile(p)
		if err == nil {
			log.Info().Str("env_path", p).Msg("Loaded .env file")
			break
		}
	}
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}

const maxDomainsResponseBytes = 8 << 20

type streamPollSettings struct {
	pollInterval     time.Duration
	requestTimeout   time.Duration
	maxRetryInterval time.Duration
}

type domainPollState struct {
	digest      [sha256.Size]byte
	hasDigest   bool
	lastVersion int64
}

type domainsResponse struct {
	Domains          []domainRecord            `json:"domains"`
	WAFRules         []wafRuleRecord           `json:"waf_rules"`
	ZeroTrustEnabled *bool                     `json:"zero_trust_enabled"`
	AgentConfig      streaming.AgentConfigData `json:"agent_config"`
}

type domainRecord struct {
	Domain         string            `json:"domain"`
	TargetURL      string            `json:"target_url"`
	TargetURLs     []string          `json:"target_urls"`
	CertificatePEM string            `json:"certificate_pem"`
	PrivateKeyPEM  string            `json:"private_key_pem"`
	Active         any               `json:"active"`
	Subdomains     []subdomainRecord `json:"subdomains"`
}

type subdomainRecord struct {
	FullDomain string   `json:"full_domain"`
	TargetURL  string   `json:"target_url"`
	TargetURLs []string `json:"target_urls"`
	Active     any      `json:"active"`
}

type wafRuleRecord struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Expression string `json:"expression"`
	Action     string `json:"action"`
	Priority   int    `json:"priority"`
}

func streamSettingsFromConfig(cfg *config.Config) streamPollSettings {
	settings := streamPollSettings{
		pollInterval:     5 * time.Second,
		requestTimeout:   10 * time.Second,
		maxRetryInterval: 2 * time.Minute,
	}
	if cfg == nil {
		return settings
	}
	if cfg.API.PollIntervalSeconds > 0 {
		settings.pollInterval = time.Duration(cfg.API.PollIntervalSeconds) * time.Second
	}
	if cfg.API.ConnectionTimeoutSeconds > 0 {
		settings.requestTimeout = time.Duration(cfg.API.ConnectionTimeoutSeconds) * time.Second
	}
	if cfg.API.MaxRetryIntervalSeconds > 0 {
		settings.maxRetryInterval = time.Duration(cfg.API.MaxRetryIntervalSeconds) * time.Second
	}
	if settings.maxRetryInterval < settings.pollInterval {
		settings.maxRetryInterval = settings.pollInterval
	}
	return settings
}

func connectToAPIStream(mgr *streaming.Manager, apiURL, apiKey string, settings streamPollSettings) {
	domainsURL := strings.TrimSuffix(apiURL, "/") + "/domains"
	state := &domainPollState{lastVersion: mgr.GetSnapshot().Version}
	retryInterval := settings.pollInterval
	consecutiveFailures := 0
	log.Info().Str("url", domainsURL).Dur("poll_interval", settings.pollInterval).Msg("Starting domains polling connection")

	for {
		ctx, cancel := context.WithTimeout(context.Background(), settings.requestTimeout)
		changed, err := pollDomains(ctx, mgr, domainsURL, apiKey, state)
		cancel()

		delay := settings.pollInterval
		if err != nil {
			consecutiveFailures++
			mgr.SetConnectionStatus(false, err)
			delay = retryInterval
			log.Warn().Err(err).Str("url", domainsURL).Dur("retry_in", delay).Int("failures", consecutiveFailures).Msg("Domains poll failed; serving cached configuration")
			if retryInterval < settings.maxRetryInterval {
				retryInterval *= 2
				if retryInterval > settings.maxRetryInterval {
					retryInterval = settings.maxRetryInterval
				}
			}
		} else {
			if consecutiveFailures > 0 {
				log.Info().Int("previous_failures", consecutiveFailures).Msg("Domains connection recovered")
			}
			consecutiveFailures = 0
			retryInterval = settings.pollInterval
			mgr.SetConnectionStatus(true, nil)
			if !changed {
				log.Debug().Msg("Domains configuration unchanged")
			}
		}
		time.Sleep(delay)
	}
}

// pollDomains fetches and applies one control-plane snapshot.
func pollDomains(ctx context.Context, mgr *streaming.Manager, domainsURL, apiKey string, state *domainPollState) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, domainsURL, nil)
	if err != nil {
		return false, fmt.Errorf("build domains request: %w", err)
	}
	addStreamAuthHeaders(req, apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return false, errors.New("unauthorized: check API key / zero trust key")
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected status from domains: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxDomainsResponseBytes+1))
	if err != nil {
		return false, fmt.Errorf("read domains response: %w", err)
	}
	if len(body) > maxDomainsResponseBytes {
		return false, fmt.Errorf("domains response exceeds %d bytes", maxDomainsResponseBytes)
	}

	var payload domainsResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return false, fmt.Errorf("decode domains response: %w", err)
	}
	snapshot := snapshotFromDomainsResponse(payload)

	digestPayload, err := json.Marshal(snapshot)
	if err != nil {
		return false, fmt.Errorf("encode domains snapshot: %w", err)
	}
	digest := sha256.Sum256(digestPayload)
	if state.hasDigest && digest == state.digest {
		return false, nil
	}

	version := time.Now().UnixNano()
	if version <= state.lastVersion {
		version = state.lastVersion + 1
	}
	snapshot.Version = version
	snapshot.Timestamp = time.Now()
	data, err := json.Marshal(snapshot)
	if err != nil {
		return false, fmt.Errorf("encode versioned snapshot: %w", err)
	}
	if err := mgr.HandleMessage(&streaming.Message{Type: "snapshot", Version: version, Timestamp: snapshot.Timestamp, Data: data}); err != nil {
		return false, fmt.Errorf("apply domains snapshot: %w", err)
	}
	state.digest = digest
	state.hasDigest = true
	state.lastVersion = version
	log.Info().Int64("version", version).Int("routes", len(snapshot.Routes)).Msg("Applied domains configuration")
	return true, nil
}

func snapshotFromDomainsResponse(payload domainsResponse) streaming.ConfigSnapshot {
	snapshot := streaming.ConfigSnapshot{
		Routes:             make(map[string]streaming.RouteData),
		RoutesConfigured:   payload.Domains != nil,
		WAFRules:           make(map[string]streaming.WAFRuleData),
		WAFRulesConfigured: payload.WAFRules != nil,
		Users:              []streaming.UserData{},
		UserDomains:        []streaming.UserDomainData{},
		AgentConfig:        payload.AgentConfig,
	}
	if payload.ZeroTrustEnabled != nil {
		snapshot.ZeroTrustEnabled = *payload.ZeroTrustEnabled
		snapshot.ZeroTrustConfigured = true
	}

	for _, domain := range payload.Domains {
		if apiRecordActive(domain.Active) && strings.TrimSpace(domain.Domain) != "" {
			snapshot.Routes[domain.Domain] = streaming.RouteData{
				Type:           "domain",
				Target:         domain.TargetURL,
				Targets:        routeTargetsFromAPI(domain.TargetURL, domain.TargetURLs),
				CertificatePEM: domain.CertificatePEM,
				PrivateKeyPEM:  domain.PrivateKeyPEM,
			}
		}
		for _, subdomain := range domain.Subdomains {
			if !apiRecordActive(domain.Active) || !apiRecordActive(subdomain.Active) || strings.TrimSpace(subdomain.FullDomain) == "" {
				continue
			}
			snapshot.Routes[subdomain.FullDomain] = streaming.RouteData{
				Type:    "domain",
				Target:  subdomain.TargetURL,
				Targets: routeTargetsFromAPI(subdomain.TargetURL, subdomain.TargetURLs),
			}
		}
	}

	for _, rule := range payload.WAFRules {
		name := ifEmpty(strings.TrimSpace(rule.Name), strings.TrimSpace(rule.ID))
		if name == "" || strings.TrimSpace(rule.Expression) == "" {
			continue
		}
		snapshot.WAFRules[name] = streaming.WAFRuleData{
			Name:       name,
			Expression: rule.Expression,
			Action:     ifEmpty(rule.Action, "BLOCK"),
			Priority:   rule.Priority,
		}
	}
	return snapshot
}

func apiRecordActive(value any) bool {
	switch active := value.(type) {
	case nil:
		return true
	case bool:
		return active
	case float64:
		return active != 0
	case string:
		switch strings.ToLower(strings.TrimSpace(active)) {
		case "", "1", "true", "yes", "on", "active", "enabled":
			return true
		case "0", "false", "no", "off", "inactive", "disabled":
			return false
		default:
			return true
		}
	default:
		return true
	}
}

func routeTargetsFromAPI(primary string, urls []string) []streaming.RouteTarget {
	targets := make([]streaming.RouteTarget, 0, len(urls)+1)
	if primary != "" {
		targets = append(targets, streaming.RouteTarget{URL: primary, HealthCheck: "http"})
	}
	for _, u := range urls {
		if u != "" && u != primary {
			targets = append(targets, streaming.RouteTarget{URL: u, HealthCheck: "http"})
		}
	}
	if len(targets) == 0 {
		return nil
	}
	return targets
}

func localConfigSnapshot(cfg *config.Config) *streaming.ConfigSnapshot {
	snapshot := &streaming.ConfigSnapshot{
		Timestamp:        time.Now(),
		Routes:           make(map[string]streaming.RouteData),
		RoutesConfigured: cfg != nil && cfg.Routes != nil,
		WAFRules:         make(map[string]streaming.WAFRuleData),
		Users:            []streaming.UserData{},
		UserDomains:      []streaming.UserDomainData{},
	}
	if cfg == nil {
		return snapshot
	}

	for key, route := range cfg.Routes {
		key = strings.TrimSpace(key)
		if key == "" || !route.IsActive() {
			continue
		}
		targets := make([]streaming.RouteTarget, 0, len(route.Targets)+1)
		if target := strings.TrimSpace(route.Target); target != "" {
			targets = append(targets, streaming.RouteTarget{URL: target, HealthCheck: "http"})
		}
		for _, target := range route.Targets {
			targetURL := strings.TrimSpace(target.URL)
			if targetURL == "" {
				continue
			}
			check := strings.ToLower(strings.TrimSpace(target.HealthCheck))
			if check == "" {
				check = "http"
			}
			targets = append(targets, streaming.RouteTarget{URL: targetURL, HealthCheck: check})
		}
		if len(targets) == 0 {
			log.Warn().Str("route", key).Msg("Ignoring local route without an upstream target")
			continue
		}
		snapshot.Routes[key] = streaming.RouteData{
			Type:           ifEmpty(strings.ToLower(strings.TrimSpace(route.Type)), "domain"),
			Targets:        targets,
			CertificatePEM: route.CertificatePEM,
			PrivateKeyPEM:  route.PrivateKeyPEM,
		}
	}
	return snapshot
}

func snapshotHasContent(snapshot *streaming.ConfigSnapshot) bool {
	if snapshot == nil {
		return false
	}
	return snapshot.Version > 0 || snapshot.RoutesConfigured || snapshot.WAFRulesConfigured || len(snapshot.Routes) > 0 || len(snapshot.WAFRules) > 0 ||
		len(snapshot.Users) > 0 || len(snapshot.UserDomains) > 0 || !snapshot.AgentConfig.IsZero()
}

func mergeConfigSnapshots(local, remote *streaming.ConfigSnapshot) *streaming.ConfigSnapshot {
	merged := &streaming.ConfigSnapshot{
		Routes:      make(map[string]streaming.RouteData),
		WAFRules:    make(map[string]streaming.WAFRuleData),
		Users:       []streaming.UserData{},
		UserDomains: []streaming.UserDomainData{},
	}
	if local != nil {
		merged.RoutesConfigured = local.RoutesConfigured || len(local.Routes) > 0
		for key, route := range local.Routes {
			merged.Routes[key] = route
		}
	}
	if remote == nil {
		return merged
	}
	merged.Version = remote.Version
	merged.Timestamp = remote.Timestamp
	merged.RoutesConfigured = merged.RoutesConfigured || remote.RoutesConfigured || len(remote.Routes) > 0
	for key, route := range remote.Routes {
		merged.Routes[key] = route
	}
	merged.WAFRulesConfigured = remote.WAFRulesConfigured || len(remote.WAFRules) > 0
	for key, rule := range remote.WAFRules {
		merged.WAFRules[key] = rule
	}
	merged.Users = append(merged.Users, remote.Users...)
	merged.UserDomains = append(merged.UserDomains, remote.UserDomains...)
	merged.ZeroTrustEnabled = remote.ZeroTrustEnabled
	merged.ZeroTrustConfigured = remote.ZeroTrustConfigured
	merged.AgentConfig = remote.AgentConfig
	return merged
}

func isRequestCacheableForSharedStore(store *cache.Store, r *http.Request) bool {
	if store == nil || r == nil {
		return false
	}
	if r.Method != http.MethodGet || r.Header.Get("Upgrade") != "" {
		return false
	}
	if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
		return false
	}
	for _, header := range []string{"Range", "If-Match", "If-None-Match", "If-Modified-Since", "If-Unmodified-Since"} {
		if r.Header.Get(header) != "" {
			return false
		}
	}
	requestCacheControl := strings.ToLower(r.Header.Get("Cache-Control"))
	if hasCacheDirective(requestCacheControl, "no-cache") || hasCacheDirective(requestCacheControl, "no-store") ||
		hasCacheDirective(requestCacheControl, "max-age") || strings.EqualFold(strings.TrimSpace(r.Header.Get("Pragma")), "no-cache") {
		return false
	}
	return true
}

func isSharedCacheableResponse(res *http.Response) bool {
	if res == nil {
		return false
	}
	if res.Header.Get("Set-Cookie") != "" {
		return false
	}
	cacheControl := strings.ToLower(res.Header.Get("Cache-Control"))
	if cacheControl == "" || !hasCacheDirective(cacheControl, "public") {
		return false
	}
	for _, directive := range []string{"private", "no-store", "no-cache"} {
		if hasCacheDirective(cacheControl, directive) {
			return false
		}
	}
	vary := strings.ToLower(strings.TrimSpace(res.Header.Get("Vary")))
	if vary == "" {
		return true
	}
	for _, part := range strings.Split(vary, ",") {
		switch strings.TrimSpace(part) {
		case "accept-encoding":
			continue
		default:
			return false
		}
	}
	return true
}

func sharedCacheTTL(cacheControl string, maximum time.Duration) (time.Duration, bool) {
	if maximum <= 0 {
		return 0, false
	}
	for _, preferred := range []string{"s-maxage", "max-age"} {
		for _, part := range strings.Split(cacheControl, ",") {
			pieces := strings.SplitN(strings.TrimSpace(part), "=", 2)
			if len(pieces) != 2 || !strings.EqualFold(pieces[0], preferred) {
				continue
			}
			seconds, err := strconv.ParseInt(strings.Trim(strings.TrimSpace(pieces[1]), `"`), 10, 64)
			if err != nil || seconds <= 0 {
				return 0, false
			}
			ttl := time.Duration(seconds) * time.Second
			if ttl < maximum {
				return ttl, true
			}
			return maximum, true
		}
	}
	return maximum, true
}

func hasCacheDirective(header, directive string) bool {
	for _, part := range strings.Split(header, ",") {
		name := strings.TrimSpace(strings.SplitN(part, "=", 2)[0])
		if name == directive {
			return true
		}
	}
	return false
}

func fetchAgentConfig(apiURL, apiKey string) (streaming.AgentConfigData, error) {
	configURL := strings.TrimSuffix(apiURL, "/") + "/agent-config"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, configURL, nil)
	if err != nil {
		return streaming.AgentConfigData{}, err
	}
	addStreamAuthHeaders(req, apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return streaming.AgentConfigData{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return streaming.AgentConfigData{}, fmt.Errorf("unexpected agent config status: %d", resp.StatusCode)
	}

	var payload struct {
		AgentConfig streaming.AgentConfigData `json:"agent_config"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return streaming.AgentConfigData{}, err
	}
	return payload.AgentConfig, nil
}

func addStreamAuthHeaders(req *http.Request, apiKey string) {
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	if ztk := os.Getenv("DiamondKey"); ztk != "" {
		req.Header.Set("X-Diamond-Key", ztk)
		req.Header.Set("X-Zero-Trust-Key", ztk)
	}
	if legacy := os.Getenv("ZERO_TRUST_KEY"); legacy != "" {
		req.Header.Set("X-Zero-Trust-Key", legacy)
	}
}

func applyAgentConfigToConfig(cfg *config.Config, agentConfig streaming.AgentConfigData) {
	if cfg == nil || agentConfig.IsZero() {
		return
	}

	cfg.Cache.Enabled = agentConfig.Cache.Enabled
	cfg.Cache.TTLSeconds = agentConfig.Cache.TTLSeconds
	cfg.Cache.MaxEntries = agentConfig.Cache.MaxEntries
	cfg.Cache.MaxBodyBytes = agentConfig.Cache.MaxBodyBytes

	cfg.RateLimit.Enabled = agentConfig.RateLimit.Enabled
	cfg.RateLimit.RequestsPerMinute = agentConfig.RateLimit.RequestsPerMinute
	cfg.RateLimit.Burst = agentConfig.RateLimit.Burst
	cfg.RateLimit.Key = string(agentConfig.RateLimit.Key)

	cfg.RequestQueue.Enabled = agentConfig.RequestQueue.Enabled
	cfg.RequestQueue.MaxConcurrent = agentConfig.RequestQueue.MaxConcurrent
	cfg.RequestQueue.MaxQueued = agentConfig.RequestQueue.MaxQueued
	cfg.RequestQueue.TimeoutSeconds = agentConfig.RequestQueue.TimeoutSeconds

	cfg.Bandwidth.Enabled = agentConfig.Bandwidth.Enabled
	cfg.Bandwidth.BytesPerSecond = agentConfig.Bandwidth.BytesPerSecond
	cfg.Bandwidth.BurstBytes = agentConfig.Bandwidth.BurstBytes
	cfg.Bandwidth.Key = string(agentConfig.Bandwidth.Key)

	cfg.Metrics.Enabled = agentConfig.Metrics.Enabled
	cfg.Metrics.Path = agentConfig.Metrics.Path

	cfg.KodaWaf.Enabled = agentConfig.KodaWaf.Enabled
	cfg.KodaWaf.Threshold = agentConfig.KodaWaf.Threshold
	cfg.KodaWaf.ModelPath = agentConfig.KodaWaf.ModelPath
	cfg.KodaWaf.ScalerPath = agentConfig.KodaWaf.ScalerPath
	cfg.KodaWaf.PythonScript = agentConfig.KodaWaf.PythonScript
	cfg.KodaWaf.FeatureHeader = agentConfig.KodaWaf.FeatureHeader

	cfg.Koda2.Enabled = agentConfig.Koda2.Enabled
	cfg.Koda2.Threshold = agentConfig.Koda2.Threshold
	cfg.Koda2.ModelPath = agentConfig.Koda2.ModelPath
	cfg.Koda2.ScalerPath = agentConfig.Koda2.ScalerPath
	cfg.Koda2.PythonScript = agentConfig.Koda2.PythonScript
	cfg.Koda2.FeatureHeader = agentConfig.Koda2.FeatureHeader
}

// applyConfigUpdates subscribes to config changes and applies them to the database.
func applyConfigUpdates(db *sql.DB, mgr *streaming.Manager, healthWorker *health.Worker, healthChecksEnabled bool, local *streaming.ConfigSnapshot, wafEngine *waf.Engine) {
	ch := mgr.Subscribe()
	log.Info().Msg("Config update subscriber started")

	for snap := range ch {
		if snap == nil {
			log.Warn().Msg("Received nil snapshot")
			continue
		}
		if err := applySnapshotToDB(db, mergeConfigSnapshots(local, snap)); err != nil {
			log.Error().Err(err).Int64("version", snap.Version).Msg("Failed to apply config snapshot atomically")
			continue
		}
		if err := wafEngine.Reload(db); err != nil {
			log.Error().Err(err).Int64("version", snap.Version).Msg("Failed to reload WAF rules")
		}
		if healthChecksEnabled {
			syncHealthTargets(db, healthWorker)
		}
	}
}

// databaseStandbyMu serializes refreshDatabaseStandby so config-update and
// periodic backup goroutines cannot race on the same standby staging files.
var databaseStandbyMu sync.Mutex

func refreshDatabaseStandby(db *sql.DB, standbyPath string) {
	if standbyPath == "" {
		return
	}
	databaseStandbyMu.Lock()
	defer databaseStandbyMu.Unlock()

	if err := database.BackupTo(db, standbyPath); err != nil {
		log.Warn().Err(err).Str("path", standbyPath).Msg("Failed to update database standby")
		return
	}
	log.Debug().Str("path", standbyPath).Msg("Database standby updated")
}

func startDatabaseBackupLoop(db *sql.DB, standbyPath string, interval time.Duration) {
	if db == nil || standbyPath == "" || interval <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			refreshDatabaseStandby(db, standbyPath)
		}
	}()
}

func syncHealthTargets(db *sql.DB, worker *health.Worker) {
	targets, err := database.ListAllRouteTargets(db)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to list route targets for health checks")
		return
	}
	healthTargets := make([]health.Target, len(targets))
	for i, t := range targets {
		healthTargets[i] = health.Target{URL: t.URL, HealthCheck: t.HealthCheck}
	}
	worker.Sync(healthTargets)
}

func applySnapshotToDB(db *sql.DB, snap *streaming.ConfigSnapshot) error {
	if db == nil || snap == nil {
		return errors.New("database and snapshot are required")
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin snapshot transaction: %w", err)
	}
	defer tx.Rollback()

	reconcileRoutes := snap.RoutesConfigured || len(snap.Routes) > 0
	if reconcileRoutes {
		if _, err := tx.Exec(`UPDATE routes SET active = 0`); err != nil {
			return fmt.Errorf("mark existing routes stale: %w", err)
		}
	}

	routesApplied := 0
	for routeKey, route := range snap.Routes {
		routeKey = strings.TrimSpace(routeKey)
		routeType := ifEmpty(strings.ToLower(strings.TrimSpace(route.Type)), "domain")
		var domainVal, pathVal string
		switch routeType {
		case "path":
			if !strings.HasPrefix(routeKey, "/") {
				return fmt.Errorf("path route %q must start with /", routeKey)
			}
			pathVal = routeKey
		case "domain", "wildcard", "regex":
			if routeKey == "" {
				return errors.New("domain route key cannot be empty")
			}
			domainVal = routeKey
		default:
			return fmt.Errorf("route %q has unsupported type %q", routeKey, route.Type)
		}

		targets, err := normalizedRouteTargets(route.AllTargets())
		if err != nil {
			return fmt.Errorf("route %q: %w", routeKey, err)
		}
		primaryTarget := targets[0].URL
		if _, err := tx.Exec(
			`INSERT INTO routes (route_type, domain, path_prefix, target_url, certificate_pem, private_key_pem, active) VALUES (?, ?, ?, ?, ?, ?, 1)
			 ON CONFLICT(route_type, domain, path_prefix) DO UPDATE SET target_url=excluded.target_url, certificate_pem=excluded.certificate_pem, private_key_pem=excluded.private_key_pem, active=1, updated_at=CURRENT_TIMESTAMP`,
			routeType, domainVal, pathVal, primaryTarget, route.CertificatePEM, route.PrivateKeyPEM); err != nil {
			return fmt.Errorf("upsert route %q: %w", routeKey, err)
		}

		var routeID int
		if err := tx.QueryRow(`SELECT id FROM routes WHERE route_type = ? AND domain = ? AND path_prefix = ?`, routeType, domainVal, pathVal).Scan(&routeID); err != nil {
			return fmt.Errorf("resolve route %q: %w", routeKey, err)
		}
		if err := database.SetRouteTargetsTx(tx, routeID, targets); err != nil {
			return fmt.Errorf("replace targets for route %q: %w", routeKey, err)
		}
		routesApplied++
	}

	if reconcileRoutes {
		if _, err := tx.Exec(`DELETE FROM route_targets WHERE route_id IN (SELECT id FROM routes WHERE active = 0)`); err != nil {
			return fmt.Errorf("delete stale route targets: %w", err)
		}
		if _, err := tx.Exec(`DELETE FROM routes WHERE active = 0`); err != nil {
			return fmt.Errorf("delete stale routes: %w", err)
		}
	}

	reconcileRules := snap.WAFRulesConfigured || len(snap.WAFRules) > 0
	if reconcileRules {
		if _, err := tx.Exec(`DELETE FROM waf_rules`); err != nil {
			return fmt.Errorf("clear stale WAF rules: %w", err)
		}
	}
	rulesApplied := 0
	for key, rule := range snap.WAFRules {
		name := ifEmpty(strings.TrimSpace(rule.Name), strings.TrimSpace(key))
		if name == "" || strings.TrimSpace(rule.Expression) == "" {
			return errors.New("WAF rule name and expression are required")
		}
		if err := waf.ValidateExpression(rule.Expression); err != nil {
			return fmt.Errorf("validate WAF rule %q: %w", name, err)
		}
		if _, err := tx.Exec(`INSERT INTO waf_rules (name, expression, action, priority) VALUES (?, ?, ?, ?)`,
			name, rule.Expression, ifEmpty(strings.ToUpper(strings.TrimSpace(rule.Action)), "BLOCK"), rule.Priority); err != nil {
			return fmt.Errorf("insert WAF rule %q: %w", name, err)
		}
		rulesApplied++
	}

	usersApplied := 0
	for _, user := range snap.Users {
		if _, err := tx.Exec(
			`INSERT INTO users (username, password_hash, email, zero_trust_enabled) VALUES (?, ?, ?, ?)
			 ON CONFLICT(username) DO UPDATE SET password_hash=excluded.password_hash, email=excluded.email, zero_trust_enabled=excluded.zero_trust_enabled`,
			user.Username, user.PasswordHash, user.Email, user.ZeroTrustEnabled); err != nil {
			return fmt.Errorf("upsert user %q: %w", user.Username, err)
		}
		usersApplied++
	}

	userDomainsApplied := 0
	for _, userDomain := range snap.UserDomains {
		var userID int
		if err := tx.QueryRow("SELECT id FROM users WHERE username = ?", userDomain.Username).Scan(&userID); err != nil {
			return fmt.Errorf("resolve user %q for domain %q: %w", userDomain.Username, userDomain.Domain, err)
		}
		if _, err := tx.Exec(
			`INSERT INTO user_proxy_records (user_id, domain, target_url, active) VALUES (?, ?, ?, ?)
			 ON CONFLICT(user_id, domain) DO UPDATE SET target_url=excluded.target_url, active=excluded.active, updated_at=CURRENT_TIMESTAMP`,
			userID, userDomain.Domain, userDomain.TargetURL, userDomain.Active); err != nil {
			return fmt.Errorf("upsert user domain %q: %w", userDomain.Domain, err)
		}
		userDomainsApplied++
	}

	if snap.ZeroTrustConfigured || snap.ZeroTrustEnabled {
		if _, err := tx.Exec(`INSERT INTO zero_trust_settings (key, value) VALUES ('enabled', ?)
			ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=CURRENT_TIMESTAMP`,
			fmt.Sprintf("%v", snap.ZeroTrustEnabled)); err != nil {
			return fmt.Errorf("update zero-trust setting: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit snapshot: %w", err)
	}
	log.Info().Int("routes_applied", routesApplied).Int("rules_applied", rulesApplied).
		Int("users_applied", usersApplied).Int("user_domains_applied", userDomainsApplied).
		Int64("version", snap.Version).Msg("Snapshot applied atomically")
	return nil
}

func normalizedRouteTargets(targets []streaming.RouteTarget) ([]database.RouteTarget, error) {
	seen := make(map[string]struct{}, len(targets))
	normalized := make([]database.RouteTarget, 0, len(targets))
	for _, target := range targets {
		targetURL := strings.TrimSpace(target.URL)
		if targetURL == "" {
			continue
		}
		parsed, err := url.Parse(targetURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return nil, fmt.Errorf("invalid HTTP upstream URL %q", targetURL)
		}
		if _, ok := seen[targetURL]; ok {
			continue
		}
		seen[targetURL] = struct{}{}
		check := strings.ToLower(strings.TrimSpace(target.HealthCheck))
		if check == "" {
			check = "http"
		}
		if check != "http" && check != "tcp" {
			return nil, fmt.Errorf("unsupported health check %q", target.HealthCheck)
		}
		normalized = append(normalized, database.RouteTarget{URL: targetURL, HealthCheck: check})
	}
	if len(normalized) == 0 {
		return nil, errors.New("at least one valid upstream target is required")
	}
	return normalized, nil
}
