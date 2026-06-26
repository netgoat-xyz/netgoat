package health

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthWorker_HTTP(t *testing.T) {
	// 1. Create a mock healthy server and a mock failing server
	healthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthyServer.Close()

	failingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError) // 500 error
	}))
	defer failingServer.Close()

	// 2. Initialize your worker with a fast interval for testing
	worker := NewWorker(100*time.Millisecond, 50*time.Millisecond, "/")
	
	// Add targets manually or simulate via worker.Sync()
	// Test the status updates
	// worker.probe(healthyServer.URL) -> assert true
	// worker.probe(failingServer.URL) -> assert false
}