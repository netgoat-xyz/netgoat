package balancer

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"

	"netgoat.xyz/agent/internal/health"
)

var ErrNoHealthyTargets = errors.New("no healthy upstream targets available")

// Balancer selects healthy upstreams using round-robin.
type Balancer struct {
	health *health.Worker
	mu     sync.Mutex
	rr     map[string]uint64
}

// New creates a load balancer backed by the given health worker.
func New(h *health.Worker) *Balancer {
	return &Balancer{
		health: h,
		rr:     make(map[string]uint64),
	}
}

// Pick returns the next healthy target URL for routeKey using round-robin.
func (b *Balancer) Pick(routeKey string, targets []string) (string, error) {
	healthy := b.health.HealthyTargets(targets)
	if len(healthy) == 0 {
		return "", ErrNoHealthyTargets
	}

	b.mu.Lock()
	idx := b.rr[routeKey]
	b.rr[routeKey] = idx + 1
	b.mu.Unlock()

	return healthy[idx%uint64(len(healthy))], nil
}

// HealthyAlternatives returns other healthy targets excluding the given URL.
func (b *Balancer) HealthyAlternatives(targets []string, exclude string) []string {
	healthy := b.health.HealthyTargets(targets)
	out := make([]string, 0, len(healthy))
	for _, t := range healthy {
		if t != exclude {
			out = append(out, t)
		}
	}
	return out
}

// ProxyHandler proxies a request to an upstream with optional failover.
type ProxyHandler struct {
	Balancer     *Balancer
	ProxyCache   map[string]*httputil.ReverseProxy
	ProxyCacheMu sync.RWMutex
}

// NewProxyHandler creates a proxy handler with a per-target reverse proxy cache.
func NewProxyHandler(b *Balancer) *ProxyHandler {
	return &ProxyHandler{
		Balancer:   b,
		ProxyCache: make(map[string]*httputil.ReverseProxy),
	}
}

// Serve routes the request to a healthy upstream, failing over on transport or 5xx errors.
func (p *ProxyHandler) Serve(w http.ResponseWriter, r *http.Request, routeKey string, targets []string, modify func(*http.Response) error) error {
	if len(targets) == 0 {
		return ErrNoHealthyTargets
	}

	tried := make(map[string]struct{}, len(targets))
	var lastErr error

	for len(tried) < len(targets) {
		candidates := make([]string, 0, len(targets))
		for _, t := range targets {
			if _, ok := tried[t]; !ok {
				candidates = append(candidates, t)
			}
		}
		if len(candidates) == 0 {
			break
		}

		targetURL, err := p.Balancer.Pick(routeKey, candidates)
		if err != nil {
			return err
		}
		tried[targetURL] = struct{}{}

		proxy, parsed, err := p.proxyFor(targetURL)
		if err != nil {
			lastErr = err
			continue
		}

		buf := &bufferedResponse{header: make(http.Header), status: http.StatusOK}
		var attemptErr error
		proxy.Director = func(req *http.Request) {
			req.URL.Scheme = parsed.Scheme
			req.URL.Host = parsed.Host
			req.Host = parsed.Host
		}
		proxy.ModifyResponse = modify
		proxy.ErrorHandler = func(_ http.ResponseWriter, _ *http.Request, proxyErr error) {
			attemptErr = proxyErr
		}

		proxy.ServeHTTP(buf, r)

		if attemptErr != nil {
			lastErr = attemptErr
			continue
		}
		if buf.status >= http.StatusInternalServerError {
			lastErr = fmt.Errorf("upstream returned %d", buf.status)
			continue
		}

		buf.flushTo(w)
		return nil
	}

	if lastErr != nil {
		return lastErr
	}
	return ErrNoHealthyTargets
}

func (p *ProxyHandler) proxyFor(targetURL string) (*httputil.ReverseProxy, *url.URL, error) {
	parsed, err := url.Parse(targetURL)
	if err != nil {
		return nil, nil, err
	}

	p.ProxyCacheMu.RLock()
	proxy, ok := p.ProxyCache[targetURL]
	p.ProxyCacheMu.RUnlock()
	if ok {
		return proxy, parsed, nil
	}

	proxy = httputil.NewSingleHostReverseProxy(parsed)
	p.ProxyCacheMu.Lock()
	p.ProxyCache[targetURL] = proxy
	p.ProxyCacheMu.Unlock()
	return proxy, parsed, nil
}

type bufferedResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
	wrote  bool
}

func (b *bufferedResponse) Header() http.Header {
	return b.header
}

func (b *bufferedResponse) WriteHeader(statusCode int) {
	if b.wrote {
		return
	}
	b.wrote = true
	b.status = statusCode
}

func (b *bufferedResponse) Write(p []byte) (int, error) {
	if !b.wrote {
		b.WriteHeader(http.StatusOK)
	}
	return b.body.Write(p)
}

func (b *bufferedResponse) flushTo(w http.ResponseWriter) {
	for k, vals := range b.header {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(b.status)
	_, _ = w.Write(b.body.Bytes())
}

// ReadResponseBody reads and replaces a response body, returning the bytes read.
func ReadResponseBody(res *http.Response) ([]byte, error) {
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	res.Body.Close()
	res.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}
