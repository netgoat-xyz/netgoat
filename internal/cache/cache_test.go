package cache

import (
	"net/http"
	"testing"
	"time"
)

func TestNewStore(t *testing.T) {
	tests := []struct {
		name         string
		ttl          time.Duration
		maxEntries   int
		maxBodyBytes int
		wantTTL      time.Duration
		wantMax      int
		wantMaxBytes int
	}{
		{
			name:         "valid values",
			ttl:          30 * time.Second,
			maxEntries:   100,
			maxBodyBytes: 512,
			wantTTL:      30 * time.Second,
			wantMax:      100,
			wantMaxBytes: 512,
		},
		{
			name:         "zero ttl defaults to 60s",
			ttl:          0,
			maxEntries:   100,
			maxBodyBytes: 512,
			wantTTL:      60 * time.Second,
			wantMax:      100,
			wantMaxBytes: 512,
		},
		{
			name:         "negative ttl defaults to 60s",
			ttl:          -5 * time.Second,
			maxEntries:   100,
			maxBodyBytes: 512,
			wantTTL:      60 * time.Second,
			wantMax:      100,
			wantMaxBytes: 512,
		},
		{
			name:         "zero maxEntries defaults to 1024",
			ttl:          30 * time.Second,
			maxEntries:   0,
			maxBodyBytes: 512,
			wantTTL:      30 * time.Second,
			wantMax:      1024,
			wantMaxBytes: 512,
		},
		{
			name:         "negative maxEntries defaults to 1024",
			ttl:          30 * time.Second,
			maxEntries:   -10,
			maxBodyBytes: 512,
			wantTTL:      30 * time.Second,
			wantMax:      1024,
			wantMaxBytes: 512,
		},
		{
			name:         "zero maxBodyBytes defaults to 1MB",
			ttl:          30 * time.Second,
			maxEntries:   100,
			maxBodyBytes: 0,
			wantTTL:      30 * time.Second,
			wantMax:      100,
			wantMaxBytes: 1 << 20,
		},
		{
			name:         "negative maxBodyBytes defaults to 1MB",
			ttl:          30 * time.Second,
			maxEntries:   100,
			maxBodyBytes: -100,
			wantTTL:      30 * time.Second,
			wantMax:      100,
			wantMaxBytes: 1 << 20,
		},
		{
			name:         "all defaults",
			ttl:          0,
			maxEntries:   0,
			maxBodyBytes: 0,
			wantTTL:      60 * time.Second,
			wantMax:      1024,
			wantMaxBytes: 1 << 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewStore(tt.ttl, tt.maxEntries, tt.maxBodyBytes)
			if store == nil {
				t.Fatal("NewStore returned nil")
			}
			if store.ttl != tt.wantTTL {
				t.Errorf("ttl = %v, want %v", store.ttl, tt.wantTTL)
			}
			if store.maxEntries != tt.wantMax {
				t.Errorf("maxEntries = %d, want %d", store.maxEntries, tt.wantMax)
			}
			if store.maxBodyBytes != tt.wantMaxBytes {
				t.Errorf("maxBodyBytes = %d, want %d", store.maxBodyBytes, tt.wantMaxBytes)
			}
			if store.ll == nil {
				t.Error("linked list is nil")
			}
			if store.items == nil {
				t.Error("items map is nil")
			}
		})
	}
}

func TestStoreSetAndGet(t *testing.T) {
	store := NewStore(10*time.Second, 10, 1024)

	header := make(http.Header)
	header.Set("Content-Type", "text/html")
	header.Set("X-Custom", "value")
	body := []byte("<html>test</html>")

	// Test Set and Get
	store.Set("key1", 200, header, body)

	entry := store.Get("key1")
	if entry == nil {
		t.Fatal("Get returned nil for existing key")
	}

	if entry.Status() != 200 {
		t.Errorf("Status = %d, want 200", entry.Status())
	}

	if string(entry.Body()) != string(body) {
		t.Errorf("Body = %s, want %s", string(entry.Body()), string(body))
	}

	if entry.Header().Get("Content-Type") != "text/html" {
		t.Errorf("Content-Type = %s, want text/html", entry.Header().Get("Content-Type"))
	}

	if entry.Header().Get("X-Custom") != "value" {
		t.Errorf("X-Custom = %s, want value", entry.Header().Get("X-Custom"))
	}
}

func TestStoreGetNonExistent(t *testing.T) {
	store := NewStore(10*time.Second, 10, 1024)

	entry := store.Get("nonexistent")
	if entry != nil {
		t.Errorf("Get returned non-nil for nonexistent key: %v", entry)
	}
}

func TestStoreExpiration(t *testing.T) {
	store := NewStore(100*time.Millisecond, 10, 1024)

	header := make(http.Header)
	body := []byte("test")

	store.Set("key1", 200, header, body)

	// Should exist immediately
	entry := store.Get("key1")
	if entry == nil {
		t.Fatal("Entry should exist immediately after Set")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired now
	entry = store.Get("key1")
	if entry != nil {
		t.Error("Entry should be expired and return nil")
	}
}

func TestStoreUpdate(t *testing.T) {
	store := NewStore(10*time.Second, 10, 1024)

	header1 := make(http.Header)
	header1.Set("Content-Type", "text/html")
	body1 := []byte("body1")

	store.Set("key1", 200, header1, body1)

	// Update the same key
	header2 := make(http.Header)
	header2.Set("Content-Type", "application/json")
	body2 := []byte("body2")

	store.Set("key1", 201, header2, body2)

	entry := store.Get("key1")
	if entry == nil {
		t.Fatal("Get returned nil after update")
	}

	if entry.Status() != 201 {
		t.Errorf("Status = %d, want 201", entry.Status())
	}

	if string(entry.Body()) != "body2" {
		t.Errorf("Body = %s, want body2", string(entry.Body()))
	}

	if entry.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %s, want application/json", entry.Header().Get("Content-Type"))
	}

	// Should still only have 1 entry
	if store.ll.Len() != 1 {
		t.Errorf("Store length = %d, want 1", store.ll.Len())
	}
}

func TestStoreLRUEviction(t *testing.T) {
	store := NewStore(10*time.Second, 3, 1024)

	header := make(http.Header)
	body := []byte("test")

	// Add 3 entries
	store.Set("key1", 200, header, body)
	store.Set("key2", 200, header, body)
	store.Set("key3", 200, header, body)

	// All should exist
	if store.Get("key1") == nil {
		t.Error("key1 should exist")
	}
	if store.Get("key2") == nil {
		t.Error("key2 should exist")
	}
	if store.Get("key3") == nil {
		t.Error("key3 should exist")
	}

	// Add 4th entry - should evict oldest (key1)
	store.Set("key4", 200, header, body)

	if store.Get("key1") != nil {
		t.Error("key1 should have been evicted")
	}
	if store.Get("key2") == nil {
		t.Error("key2 should still exist")
	}
	if store.Get("key3") == nil {
		t.Error("key3 should still exist")
	}
	if store.Get("key4") == nil {
		t.Error("key4 should exist")
	}

	if store.ll.Len() != 3 {
		t.Errorf("Store length = %d, want 3", store.ll.Len())
	}
}

func TestStoreLRUOrder(t *testing.T) {
	store := NewStore(10*time.Second, 3, 1024)

	header := make(http.Header)
	body := []byte("test")

	// Add 3 entries
	store.Set("key1", 200, header, body)
	store.Set("key2", 200, header, body)
	store.Set("key3", 200, header, body)

	// Access key1 to move it to front
	store.Get("key1")

	// Add 4th entry - should evict key2 (oldest unused)
	store.Set("key4", 200, header, body)

	if store.Get("key1") == nil {
		t.Error("key1 should still exist (was accessed)")
	}
	if store.Get("key2") != nil {
		t.Error("key2 should have been evicted")
	}
	if store.Get("key3") == nil {
		t.Error("key3 should still exist")
	}
	if store.Get("key4") == nil {
		t.Error("key4 should exist")
	}
}

func TestStoreMaxBodyBytes(t *testing.T) {
	store := NewStore(10*time.Second, 10, 10) // Only 10 bytes max

	header := make(http.Header)
	smallBody := []byte("small")
	largeBody := []byte("this is a large body that exceeds the limit")

	// Small body should be cached
	store.Set("key1", 200, header, smallBody)
	if store.Get("key1") == nil {
		t.Error("Small body should be cached")
	}

	// Large body should not be cached
	store.Set("key2", 200, header, largeBody)
	if store.Get("key2") != nil {
		t.Error("Large body should not be cached")
	}

	// Small body should still exist
	if store.Get("key1") == nil {
		t.Error("key1 should still exist")
	}
}

func TestCloneHeader(t *testing.T) {
	original := make(http.Header)
	original.Set("Content-Type", "text/html")
	original.Set("X-Custom", "value")
	original.Add("X-Multi", "value1")
	original.Add("X-Multi", "value2")
	original.Set("Connection", "keep-alive") // hop-by-hop header

	cloned := cloneHeader(original)

	// Regular headers should be cloned
	if cloned.Get("Content-Type") != "text/html" {
		t.Error("Content-Type not cloned")
	}
	if cloned.Get("X-Custom") != "value" {
		t.Error("X-Custom not cloned")
	}

	// Multi-value headers should be cloned
	if len(cloned["X-Multi"]) != 2 {
		t.Errorf("X-Multi values = %d, want 2", len(cloned["X-Multi"]))
	}

	// Hop-by-hop headers should be filtered
	if cloned.Get("Connection") != "" {
		t.Error("Connection header should be filtered out")
	}

	// Modifying clone should not affect original
	cloned.Set("Content-Type", "application/json")
	if original.Get("Content-Type") != "text/html" {
		t.Error("Modifying clone affected original")
	}
}

func TestIsHopByHop(t *testing.T) {
	hopByHopHeaders := []string{
		"connection", "Connection", "CONNECTION",
		"keep-alive", "Keep-Alive",
		"proxy-authenticate", "Proxy-Authenticate",
		"proxy-authorization", "Proxy-Authorization",
		"te", "TE",
		"trailers", "Trailers",
		"transfer-encoding", "Transfer-Encoding",
		"upgrade", "Upgrade",
	}

	for _, header := range hopByHopHeaders {
		if !isHopByHop(header) {
			t.Errorf("isHopByHop(%s) = false, want true", header)
		}
	}

	regularHeaders := []string{
		"Content-Type", "X-Custom", "Authorization", "Content-Length",
	}

	for _, header := range regularHeaders {
		if isHopByHop(header) {
			t.Errorf("isHopByHop(%s) = true, want false", header)
		}
	}
}

func TestCacheKey(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		host     string
		path     string
		query    string
		wantKey  string
	}{
		{
			name:    "simple GET",
			method:  "GET",
			host:    "example.com",
			path:    "/path",
			query:   "",
			wantKey: "GET|example.com|/path?",
		},
		{
			name:    "with query",
			method:  "GET",
			host:    "example.com",
			path:    "/path",
			query:   "foo=bar&baz=qux",
			wantKey: "GET|example.com|/path?foo=bar&baz=qux",
		},
		{
			name:    "POST request",
			method:  "POST",
			host:    "api.example.com",
			path:    "/api/users",
			query:   "",
			wantKey: "POST|api.example.com|/api/users?",
		},
		{
			name:    "root path",
			method:  "GET",
			host:    "example.com",
			path:    "/",
			query:   "",
			wantKey: "GET|example.com|/?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, "http://"+tt.host+tt.path+"?"+tt.query, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			key := CacheKey(req)
			if key != tt.wantKey {
				t.Errorf("CacheKey() = %s, want %s", key, tt.wantKey)
			}
		})
	}
}

func TestStoreConcurrency(t *testing.T) {
	store := NewStore(10*time.Second, 100, 1024)
	header := make(http.Header)
	body := []byte("test")

	// Test concurrent writes
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				key := string(rune('a' + id))
				store.Set(key, 200, header, body)
				store.Get(key)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not panic and should have entries
	if store.ll.Len() == 0 {
		t.Error("Store should have entries after concurrent access")
	}
}

func TestEntryMethods(t *testing.T) {
	header := make(http.Header)
	header.Set("Content-Type", "text/html")
	body := []byte("test body")

	entry := &Entry{
		key:     "test",
		status:  404,
		header:  header,
		body:    body,
		expires: time.Now().Add(10 * time.Second),
	}

	if entry.Status() != 404 {
		t.Errorf("Status() = %d, want 404", entry.Status())
	}

	if string(entry.Body()) != "test body" {
		t.Errorf("Body() = %s, want 'test body'", string(entry.Body()))
	}

	if entry.Header().Get("Content-Type") != "text/html" {
		t.Errorf("Header().Get('Content-Type') = %s, want 'text/html'", entry.Header().Get("Content-Type"))
	}
}

func TestStoreBodyIsolation(t *testing.T) {
	store := NewStore(10*time.Second, 10, 1024)
	header := make(http.Header)
	originalBody := []byte("original")

	store.Set("key1", 200, header, originalBody)

	// Modify original body
	originalBody[0] = 'X'

	// Retrieved body should not be affected
	entry := store.Get("key1")
	if entry == nil {
		t.Fatal("Entry should exist")
	}

	if string(entry.Body()) != "original" {
		t.Errorf("Body was affected by modification of original: %s", string(entry.Body()))
	}
}