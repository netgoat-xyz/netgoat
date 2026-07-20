package balancer

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"netgoat.xyz/agent/internal/health"
)

func TestProxyHandlerDoesNotReplayGETBodyDuringFailover(t *testing.T) {
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer first.Close()
	var secondRequests atomic.Int32
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondRequests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer second.Close()

	handler := NewProxyHandler(New(health.NewWorker(time.Second, time.Second, "/")), nil)
	req := httptest.NewRequest(http.MethodGet, "http://example.test/", strings.NewReader("request body"))
	response := httptest.NewRecorder()
	if err := handler.Serve(response, req, "route", []string{first.URL, second.URL}, nil); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want first upstream response", response.Code)
	}
	if got := secondRequests.Load(); got != 0 {
		t.Fatalf("body-bearing request was replayed %d times", got)
	}
}
