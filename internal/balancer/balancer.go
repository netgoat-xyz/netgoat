package balancer

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
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
	Transport    http.RoundTripper
}

// NewProxyHandler creates a proxy handler with a per-target reverse proxy cache.
func NewProxyHandler(b *Balancer, transport http.RoundTripper) *ProxyHandler {
	return &ProxyHandler{
		Balancer:   b,
		ProxyCache: make(map[string]*httputil.ReverseProxy),
		Transport:  transport,
	}
}

// Serve routes the request to a healthy upstream, failing over on transport or 5xx errors
// for idempotent methods only.
func (p *ProxyHandler) Serve(w http.ResponseWriter, r *http.Request, routeKey string, targets []string, modify func(*http.Response) error) error {
	if len(targets) == 0 {
		return ErrNoHealthyTargets
	}

	retryOnFailure := isFailoverSafeMethod(r.Method)
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
			if !retryOnFailure {
				return lastErr
			}
			continue
		}

		clone := *proxy
		proxy = &clone

		out := &streamWriter{w: w, header: make(http.Header)}
		var attemptErr error
		proxy.Director = func(req *http.Request) {
			req.URL.Scheme = parsed.Scheme
			req.URL.Host = parsed.Host
			req.Host = parsed.Host

			// Preserve original host/proto for upstream services.
			if req.Header.Get("X-Forwarded-Host") == "" && r.Host != "" {
				req.Header.Set("X-Forwarded-Host", r.Host)
			}
			if req.Header.Get("X-Forwarded-Proto") == "" {
				if r.TLS != nil {
					req.Header.Set("X-Forwarded-Proto", "https")
				} else {
					req.Header.Set("X-Forwarded-Proto", "http")
				}
			}
			appendXForwardedFor(req.Header, clientIPFromRequest(r))
		}
		proxy.ModifyResponse = modify
		proxy.ErrorHandler = func(_ http.ResponseWriter, _ *http.Request, proxyErr error) {
			attemptErr = proxyErr
		}

		proxy.ServeHTTP(out, r)

		if attemptErr != nil {
			lastErr = attemptErr
			if !retryOnFailure {
				return lastErr
			}
			continue
		}
		if out.retry {
			lastErr = fmt.Errorf("upstream returned %d", out.status)
			if !retryOnFailure {
				out.flushRetryTo(w)
				return nil
			}
			continue
		}

		return nil
	}

	if lastErr != nil {
		return lastErr
	}
	return ErrNoHealthyTargets
}

func isFailoverSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
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
	if p.Transport != nil {
		proxy.Transport = p.Transport
	}
	p.ProxyCacheMu.Lock()
	p.ProxyCache[targetURL] = proxy
	p.ProxyCacheMu.Unlock()
	return proxy, parsed, nil
}

type streamWriter struct {
	w        http.ResponseWriter
	header   http.Header
	status   int
	wroteHdr bool
	retry    bool
	buf      bytes.Buffer
}

func (s *streamWriter) Header() http.Header {
	return s.header
}

func (s *streamWriter) WriteHeader(statusCode int) {
	if s.wroteHdr {
		return
	}
	s.wroteHdr = true
	s.status = statusCode
	if statusCode >= http.StatusInternalServerError {
		s.retry = true
		return
	}
	for k, vals := range s.header {
		for _, v := range vals {
			s.w.Header().Add(k, v)
		}
	}
	s.w.WriteHeader(statusCode)
}

func (s *streamWriter) Write(p []byte) (int, error) {
	if !s.wroteHdr {
		s.WriteHeader(http.StatusOK)
	}
	if s.retry {
		return s.buf.Write(p)
	}
	return s.w.Write(p)
}

// Flush implements http.Flusher so ReverseProxy can stream SSE/chunked responses.
func (s *streamWriter) Flush() {
	if s.retry {
		return
	}
	if flusher, ok := s.w.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Unwrap exposes the underlying ResponseWriter for http.ResponseController
// (Hijacker, additional flush paths, etc.).
func (s *streamWriter) Unwrap() http.ResponseWriter {
	return s.w
}

func (s *streamWriter) flushRetryTo(w http.ResponseWriter) {
	for k, vals := range s.header {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(s.status)
	_, _ = w.Write(s.buf.Bytes())
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

func clientIPFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	// Prefer existing X-Forwarded-For chain if present.
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// first IP is the client
		if idx := strings.Index(xff, ","); idx > 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

func appendXForwardedFor(h http.Header, clientIP string) {
	if clientIP == "" {
		return
	}
	if prior := h.Get("X-Forwarded-For"); prior != "" {
		h.Set("X-Forwarded-For", prior+", "+clientIP)
		return
	}
	h.Set("X-Forwarded-For", clientIP)
}
