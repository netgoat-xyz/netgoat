package metrics

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRecorderSnapshot(t *testing.T) {
	rec := NewRecorder()
	rec.RecordRequest()
	rec.RecordCacheHit()
	rec.RecordBlocked("rate limit")
	rec.RecordProxyError(errors.New("dial tcp: connection refused"))
	rec.RecordResponse(http.StatusTooManyRequests, 12, 20*time.Millisecond)

	snap := rec.Snapshot()
	if snap.Requests != 1 {
		t.Fatalf("Requests = %d, want 1", snap.Requests)
	}
	if snap.CacheHits != 1 || snap.Blocked != 1 || snap.ProxyErrors != 1 {
		t.Fatalf("unexpected counters: %+v", snap)
	}
	if snap.StatusCodes["429"] != 1 {
		t.Fatalf("StatusCodes[429] = %d, want 1", snap.StatusCodes["429"])
	}
	if snap.BlockReasons["rate limit"] != 1 {
		t.Fatalf("BlockReasons[rate limit] = %d, want 1", snap.BlockReasons["rate limit"])
	}
	if snap.ErrorStatusCodes["429"] != 1 {
		t.Fatalf("ErrorStatusCodes[429] = %d, want 1", snap.ErrorStatusCodes["429"])
	}
	if len(snap.RecentErrors) == 0 {
		t.Fatalf("RecentErrors is empty")
	}
	if snap.BytesWritten != 12 {
		t.Fatalf("BytesWritten = %d, want 12", snap.BytesWritten)
	}
}

func TestRecorderBoundsDistinctProxyErrors(t *testing.T) {
	rec := NewRecorder()
	for i := 0; i < maxTrackedErrors*2; i++ {
		rec.RecordProxyError(errors.New("upstream failure " + strconv.Itoa(i)))
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if got := len(rec.errors); got > maxTrackedErrors+1 {
		t.Fatalf("tracked error cardinality = %d", got)
	}
}

func TestServeJSON(t *testing.T) {
	rec := NewRecorder()
	rec.RecordRequest()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	res := httptest.NewRecorder()
	rec.ServeJSON(res, req)

	var snap Snapshot
	if err := json.Unmarshal(res.Body.Bytes(), &snap); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if snap.Requests != 1 {
		t.Fatalf("Requests = %d, want 1", snap.Requests)
	}
}

func TestServePrometheus(t *testing.T) {
	rec := NewRecorder()
	rec.RecordResponse(http.StatusOK, 10, time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/metrics.prom", nil)
	res := httptest.NewRecorder()
	rec.ServePrometheus(res, req)

	body := res.Body.String()
	if !strings.Contains(body, "netgoat_responses_total 1") {
		t.Fatalf("missing response counter in %q", body)
	}
	if !strings.Contains(body, "netgoat_responses_by_status_total{code=\"200\"} 1") {
		t.Fatalf("missing status counter in %q", body)
	}
}

func TestResponseWriterRecordsStatusAndBytes(t *testing.T) {
	recorder := httptest.NewRecorder()
	wrapped := WrapResponseWriter(recorder)

	wrapped.WriteHeader(http.StatusCreated)
	n, err := wrapped.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != 5 || wrapped.BytesWritten() != 5 {
		t.Fatalf("bytes = %d/%d, want 5", n, wrapped.BytesWritten())
	}
	if wrapped.Status() != http.StatusCreated {
		t.Fatalf("Status = %d, want 201", wrapped.Status())
	}
}
