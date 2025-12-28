package main

import (
	"context"
	"errors"
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
	"netgoat.xyz/agent/internal/challenge"
	"netgoat.xyz/agent/internal/config"
	"netgoat.xyz/agent/internal/database"
	"netgoat.xyz/agent/internal/honeypot"
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

	pages := buildErrorPageStore(cfg)

	var detector *anomaly.Detector
	featureHeader := "X-GoatAI-Features"
	if cfg.Anomaly.FeatureHeader != "" {
		featureHeader = cfg.Anomaly.FeatureHeader
	}
	if cfg.Anomaly.Enabled {
		detector = anomaly.NewDetector(anomaly.Settings{
			Enabled:   cfg.Anomaly.Enabled,
			Threshold: ifZero(cfg.Anomaly.Threshold, 0.7),
			Model:     ifEmpty(cfg.Anomaly.Model, "netgoat-ai/GoatAI"),
			Token:     cfg.Anomaly.HuggingFaceToken,
		})
		log.Info().Bool("enabled", true).Str("model", ifEmpty(cfg.Anomaly.Model, "netgoat-ai/GoatAI")).Float64("threshold", ifZero(cfg.Anomaly.Threshold, 0.7)).Msg("Anomaly detection configured")
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
		if cfg.Auth.Enabled {
			if !auth.Check(r) {
				if strings.Contains(r.Header.Get("Accept"), "application/json") {
					writeError(w, pages, challengeStore, r, http.StatusUnauthorized, "Unauthorized")
				} else {
					http.Redirect(w, r, "/login", http.StatusFound)
				}
				return
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
				ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
				label, score, derr := detector.PredictCSV(ctx, csv)
				cancel()
				if derr != nil {
					log.Warn().Err(derr).Msg("Anomaly detection error")
				} else {
					log.Info().Str("label", label).Float64("score", score).Msg("Anomaly prediction")
					if detector.IsAnomalous(label, score) {
						log.Warn().Str("label", label).Float64("score", score).Str("ip", r.RemoteAddr).Str("path", r.URL.Path).Msg("Blocked by AI anomaly detector")
						writeError(w, pages, challengeStore, r, http.StatusForbidden, "Forbidden")
						return
					}
				}
			}
		}

		blocked, ruleName := waf.Check(db, r, cfg.DebugLogs)
		if blocked {
			log.Warn().Str("rule", ruleName).Str("ip", r.RemoteAddr).Str("path", r.URL.Path).Msg("Request blocked by WAF")
			writeError(w, pages, challengeStore, r, http.StatusForbidden, "Forbidden")
			return
		}

		targetStr := database.GetTarget(db, r.URL.Path)
		if targetStr == "" {
			writeError(w, pages, challengeStore, r, http.StatusNotFound, "No route found")
			return
		}

		targetURL, err := url.Parse(targetStr)
		if err != nil {
			log.Error().Err(err).Str("target", targetStr).Msg("Invalid target URL in DB")
			writeError(w, pages, challengeStore, r, http.StatusInternalServerError, "Internal Server Error")
			return
		}

		if r.Header.Get("Upgrade") == "websocket" {
			log.Info().Str("client", r.RemoteAddr).Msg("WebSocket upgrade detected")
		}

		log.Info().Str("method", r.Method).Str("path", r.URL.Path).Str("target", targetStr).Msg("Proxying request")

		proxy := httputil.NewSingleHostReverseProxy(targetURL)

		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			req.Host = targetURL.Host
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

func getClientIP(r *http.Request) string {
	// Try X-Forwarded-For first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.Index(xff, ","); idx > 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	// Try X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Fall back to RemoteAddr
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip == "" {
		ip = r.RemoteAddr
	}
	return ip
}
