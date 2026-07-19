package cache

import (
	"net/http"
	"testing"
	"time"
)

func TestStoreSetWithTTLHonorsShorterResponseLifetime(t *testing.T) {
	store := NewStore(time.Minute, 10, 1024)
	store.SetWithTTL("short", http.StatusOK, make(http.Header), []byte("body"), 20*time.Millisecond)
	if store.Get("short") == nil {
		t.Fatal("entry was not stored")
	}
	time.Sleep(30 * time.Millisecond)
	if store.Get("short") != nil {
		t.Fatal("entry outlived response max-age")
	}
	store.SetWithTTL("zero", http.StatusOK, make(http.Header), []byte("body"), 0)
	if store.Get("zero") != nil {
		t.Fatal("zero-lifetime response was cached")
	}
}
