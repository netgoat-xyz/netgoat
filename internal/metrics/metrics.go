package metrics

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type Recorder struct {
	started int64

	requests     atomic.Uint64
	responses    atomic.Uint64
	blocked      atomic.Uint64
	cacheHits    atomic.Uint64
	proxyErrors  atomic.Uint64
	bytesWritten atomic.Uint64
	latencyNanos atomic.Uint64

	mu       sync.Mutex
	statuses map[int]uint64
	blocks   map[string]uint64
}

type Snapshot struct {
	StartedAt        time.Time         `json:"started_at"`
	UptimeSeconds    int64             `json:"uptime_seconds"`
	Requests         uint64            `json:"requests"`
	Responses        uint64            `json:"responses"`
	Blocked          uint64            `json:"blocked"`
	CacheHits        uint64            `json:"cache_hits"`
	ProxyErrors      uint64            `json:"proxy_errors"`
	BytesWritten     uint64            `json:"bytes_written"`
	AverageLatencyMs float64           `json:"average_latency_ms"`
	StatusCodes      map[string]uint64 `json:"status_codes"`
	BlockReasons     map[string]uint64 `json:"block_reasons"`
}

func NewRecorder() *Recorder {
	return &Recorder{
		started:  time.Now().Unix(),
		statuses: make(map[int]uint64),
		blocks:   make(map[string]uint64),
	}
}

func (r *Recorder) RecordRequest() {
	r.requests.Add(1)
}

func (r *Recorder) RecordResponse(status int, bytes int64, duration time.Duration) {
	if status == 0 {
		status = http.StatusOK
	}
	r.responses.Add(1)
	if bytes > 0 {
		r.bytesWritten.Add(uint64(bytes))
	}
	if duration > 0 {
		r.latencyNanos.Add(uint64(duration))
	}

	r.mu.Lock()
	r.statuses[status]++
	r.mu.Unlock()
}

func (r *Recorder) RecordBlocked(reason string) {
	if reason == "" {
		reason = "blocked"
	}
	r.blocked.Add(1)

	r.mu.Lock()
	r.blocks[reason]++
	r.mu.Unlock()
}

func (r *Recorder) RecordCacheHit() {
	r.cacheHits.Add(1)
}

func (r *Recorder) RecordProxyError() {
	r.proxyErrors.Add(1)
}

func (r *Recorder) Snapshot() Snapshot {
	started := time.Unix(r.started, 0)
	responses := r.responses.Load()
	avgLatency := 0.0
	if responses > 0 {
		avgLatency = float64(r.latencyNanos.Load()) / float64(responses) / float64(time.Millisecond)
	}

	r.mu.Lock()
	statuses := make(map[string]uint64, len(r.statuses))
	for code, count := range r.statuses {
		statuses[strconv.Itoa(code)] = count
	}
	blocks := make(map[string]uint64, len(r.blocks))
	for reason, count := range r.blocks {
		blocks[reason] = count
	}
	r.mu.Unlock()

	return Snapshot{
		StartedAt:        started,
		UptimeSeconds:    int64(time.Since(started).Seconds()),
		Requests:         r.requests.Load(),
		Responses:        responses,
		Blocked:          r.blocked.Load(),
		CacheHits:        r.cacheHits.Load(),
		ProxyErrors:      r.proxyErrors.Load(),
		BytesWritten:     r.bytesWritten.Load(),
		AverageLatencyMs: avgLatency,
		StatusCodes:      statuses,
		BlockReasons:     blocks,
	}
}

func (r *Recorder) ServeJSON(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(r.Snapshot())
}

func (r *Recorder) ServePrometheus(w http.ResponseWriter, _ *http.Request) {
	snap := r.Snapshot()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	fmt.Fprintf(w, "netgoat_requests_total %d\n", snap.Requests)
	fmt.Fprintf(w, "netgoat_responses_total %d\n", snap.Responses)
	fmt.Fprintf(w, "netgoat_blocked_total %d\n", snap.Blocked)
	fmt.Fprintf(w, "netgoat_cache_hits_total %d\n", snap.CacheHits)
	fmt.Fprintf(w, "netgoat_proxy_errors_total %d\n", snap.ProxyErrors)
	fmt.Fprintf(w, "netgoat_bytes_written_total %d\n", snap.BytesWritten)
	fmt.Fprintf(w, "netgoat_average_latency_ms %.3f\n", snap.AverageLatencyMs)

	codes := sortedKeys(snap.StatusCodes)
	for _, code := range codes {
		fmt.Fprintf(w, "netgoat_responses_by_status_total{code=%q} %d\n", code, snap.StatusCodes[code])
	}
	reasons := sortedKeys(snap.BlockReasons)
	for _, reason := range reasons {
		fmt.Fprintf(w, "netgoat_blocks_by_reason_total{reason=%q} %d\n", reason, snap.BlockReasons[reason])
	}
}

func sortedKeys(m map[string]uint64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

type ResponseWriter struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func WrapResponseWriter(w http.ResponseWriter) *ResponseWriter {
	return &ResponseWriter{ResponseWriter: w}
}

func (w *ResponseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *ResponseWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += int64(n)
	return n, err
}

func (w *ResponseWriter) Status() int {
	return w.status
}

func (w *ResponseWriter) BytesWritten() int64 {
	return w.bytes
}

func (w *ResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *ResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hijacker.Hijack()
}

func (w *ResponseWriter) Push(target string, opts *http.PushOptions) error {
	pusher, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, opts)
}

func (w *ResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
