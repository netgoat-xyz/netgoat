package health

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Target describes an upstream to probe.
type Target struct {
	URL         string
	HealthCheck string // "http" or "tcp"; defaults to http
}

// Worker periodically probes upstreams and tracks healthy vs. unhealthy state.
type Worker struct {
	mu       sync.RWMutex
	healthy  map[string]bool
	checked  map[string]bool
	targets  map[string]Target
	interval time.Duration
	timeout  time.Duration
	path     string
	client   *http.Client
}

// NewWorker creates a health checker with the given probe interval, timeout, and HTTP path.
func NewWorker(interval, timeout time.Duration, path string) *Worker {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	if path == "" {
		path = "/"
	}
	return &Worker{
		healthy:  make(map[string]bool),
		checked:  make(map[string]bool),
		targets:  make(map[string]Target),
		interval: interval,
		timeout:  timeout,
		path:     path,
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// Sync replaces the set of upstreams to monitor.
func (w *Worker) Sync(targets []Target) {
	w.mu.Lock()
	defer w.mu.Unlock()

	next := make(map[string]Target, len(targets))
	for _, t := range targets {
		if t.URL == "" {
			continue
		}
		check := strings.ToLower(strings.TrimSpace(t.HealthCheck))
		if check == "" {
			check = "http"
		}
		next[t.URL] = Target{URL: t.URL, HealthCheck: check}
	}

	for url := range w.targets {
		if _, ok := next[url]; !ok {
			delete(w.healthy, url)
			delete(w.checked, url)
		}
	}

	for url := range next {
		if _, existed := w.targets[url]; !existed {
			w.healthy[url] = true
			w.checked[url] = false
		}
	}

	w.targets = next
}

// IsHealthy reports whether an upstream is considered reachable.
// Targets not yet probed are treated as healthy so traffic can flow immediately.
func (w *Worker) IsHealthy(targetURL string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if _, known := w.targets[targetURL]; !known {
		return true
	}
	if !w.checked[targetURL] {
		return true
	}
	return w.healthy[targetURL]
}

// HealthyTargets returns the subset of the given URLs that are currently healthy.
func (w *Worker) HealthyTargets(urls []string) []string {
	out := make([]string, 0, len(urls))
	for _, u := range urls {
		if w.IsHealthy(u) {
			out = append(out, u)
		}
	}
	return out
}

// ProbeAllOnce runs a single probe cycle synchronously.
func (w *Worker) ProbeAllOnce() {
	w.probeAll()
}

// Start runs the background probe loop until ctx is cancelled.
func (w *Worker) Start(ctx context.Context) {
	go func() {
		w.probeAll()
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.probeAll()
			}
		}
	}()
}

func (w *Worker) probeAll() {
	w.mu.RLock()
	targets := make([]Target, 0, len(w.targets))
	for _, t := range w.targets {
		targets = append(targets, t)
	}
	w.mu.RUnlock()

	for _, t := range targets {
		ok := w.probe(t)
		w.mu.Lock()
		prev := w.healthy[t.URL]
		w.healthy[t.URL] = ok
		w.checked[t.URL] = true
		w.mu.Unlock()

		if prev != ok {
			if ok {
				log.Info().Str("target", t.URL).Msg("Upstream became healthy")
			} else {
				log.Warn().Str("target", t.URL).Str("check", t.HealthCheck).Msg("Upstream became unhealthy")
			}
		}
	}
}

func (w *Worker) probe(t Target) bool {
	switch strings.ToLower(t.HealthCheck) {
	case "tcp":
		return w.checkTCP(t.URL)
	default:
		return w.checkHTTP(t.URL)
	}
}

func (w *Worker) checkHTTP(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	checkURL := *u
	checkURL.Path = w.path
	checkURL.RawQuery = ""
	checkURL.Fragment = ""

	req, err := http.NewRequest(http.MethodGet, checkURL.String(), nil)
	if err != nil {
		return false
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode < 500
}

func (w *Worker) checkTCP(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	host := u.Host
	if host == "" {
		host = u.Path
	}
	if !strings.Contains(host, ":") {
		switch strings.ToLower(u.Scheme) {
		case "https":
			host += ":443"
		default:
			host += ":80"
		}
	}

	conn, err := net.DialTimeout("tcp", host, w.timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
