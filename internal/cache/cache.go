package cache

import (
	"bytes"
	"container/list"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Entry stores a cached HTTP response.
type Entry struct {
	key     string
	status  int
	header  http.Header
	body    []byte
	expires time.Time
}

// Status returns the cached HTTP status code.
func (e *Entry) Status() int {
	return e.status
}

// Header returns the cached HTTP headers.
func (e *Entry) Header() http.Header {
	return e.header
}

// Body returns the cached response body.
func (e *Entry) Body() []byte {
	return e.body
}

// Store is a simple in-memory LRU cache with TTL.
type Store struct {
	mu           sync.Mutex
	ttl          time.Duration
	maxEntries   int
	maxBodyBytes int
	ll           *list.List
	items        map[string]*list.Element
}

// NewStore creates a cache store.
func NewStore(ttl time.Duration, maxEntries int, maxBodyBytes int) *Store {
	if maxEntries <= 0 {
		maxEntries = 1024
	}
	if maxBodyBytes <= 0 {
		maxBodyBytes = 1 << 20 // 1MB default
	}
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	return &Store{
		ttl:          ttl,
		maxEntries:   maxEntries,
		maxBodyBytes: maxBodyBytes,
		ll:           list.New(),
		items:        make(map[string]*list.Element, maxEntries),
	}
}

// Get returns a cached entry if present and not expired.
func (s *Store) Get(key string) *Entry {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ele, ok := s.items[key]; ok {
		ent := ele.Value.(*Entry)
		if time.Now().After(ent.expires) {
			s.removeElement(ele)
			return nil
		}
		s.ll.MoveToFront(ele)
		return ent
	}
	return nil
}

// Set inserts or updates a cache entry.
func (s *Store) Set(key string, status int, header http.Header, body []byte) {
	if len(body) > s.maxBodyBytes {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if ele, ok := s.items[key]; ok {
		s.ll.MoveToFront(ele)
		// Entries may still be referenced by concurrent cache hits. Replace the
		// value instead of mutating it so readers always observe an immutable
		// response and do not race with cache refreshes.
		ele.Value = &Entry{
			key:     key,
			status:  status,
			header:  cloneHeader(header),
			body:    append([]byte(nil), body...),
			expires: time.Now().Add(s.ttl),
		}
		return
	}

	ent := &Entry{
		key:     key,
		status:  status,
		header:  cloneHeader(header),
		body:    append([]byte(nil), body...),
		expires: time.Now().Add(s.ttl),
	}
	ele := s.ll.PushFront(ent)
	s.items[key] = ele

	if s.ll.Len() > s.maxEntries {
		s.removeOldest()
	}
}

func (s *Store) removeOldest() {
	ele := s.ll.Back()
	if ele != nil {
		s.removeElement(ele)
	}
}

func (s *Store) removeElement(e *list.Element) {
	s.ll.Remove(e)
	ent := e.Value.(*Entry)
	delete(s.items, ent.key)
}

func cloneHeader(h http.Header) http.Header {
	dst := make(http.Header, len(h))
	for k, vals := range h {
		if isHopByHop(k) {
			continue
		}
		copied := make([]string, len(vals))
		copy(copied, vals)
		dst[k] = copied
	}
	return dst
}

func isHopByHop(k string) bool {
	k = strings.ToLower(k)
	switch k {
	case "connection", "keep-alive", "proxy-authenticate", "proxy-authorization", "te", "trailers", "transfer-encoding", "upgrade":
		return true
	default:
		return false
	}
}

// CacheKey builds a cache key from method, host, path, query.
func CacheKey(r *http.Request) string {
	// Vary: Accept-Encoding is supported by the shared cache, so the request
	// encoding preference must be part of the key. Otherwise a gzip response
	// can be replayed to a client that never advertised gzip support.
	encoding := strings.ToLower(strings.Join(strings.Fields(r.Header.Get("Accept-Encoding")), ""))
	return r.Method + "|" + strings.ToLower(r.Host) + "|" + r.URL.EscapedPath() + "?" + r.URL.RawQuery + "|ae=" + encoding
}

// MaxBodyBytes returns the largest response body accepted by the store.
func (s *Store) MaxBodyBytes() int {
	if s == nil {
		return 0
	}
	return s.maxBodyBytes
}

// CaptureOnEOF wraps a response body and records at most maxBytes while the
// response streams to the client. onComplete is called only after a complete,
// successful read whose body fits within the limit. This avoids buffering an
// untrusted upstream response before proxying it.
func CaptureOnEOF(body io.ReadCloser, maxBytes int, onComplete func([]byte)) io.ReadCloser {
	if body == nil || maxBytes <= 0 || onComplete == nil {
		return body
	}
	return &captureReadCloser{
		ReadCloser: body,
		maxBytes:   maxBytes,
		onComplete: onComplete,
	}
}

type captureReadCloser struct {
	io.ReadCloser
	buf        bytes.Buffer
	maxBytes   int
	overflow   bool
	completed  bool
	onComplete func([]byte)
}

func (r *captureReadCloser) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if n > 0 && !r.overflow {
		if r.buf.Len()+n <= r.maxBytes {
			_, _ = r.buf.Write(p[:n])
		} else {
			r.overflow = true
			r.buf.Reset()
		}
	}
	if err == io.EOF && !r.completed {
		r.completed = true
		if !r.overflow {
			r.onComplete(append([]byte(nil), r.buf.Bytes()...))
		}
	}
	return n, err
}
