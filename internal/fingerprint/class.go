package fingerprint

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

const cloudflareConnectingIP = "CF-Connecting-IP"

type recorderKey struct{}
type classKey struct{}

// Recorder is the per-connection stack observation. JA4 and ALPN are
// filled from the raw ClientHello. The H2 field starts as "-" and is
// replaced if this connection negotiates HTTP/2 and initial frames arrive.
type Recorder struct {
	mu     sync.Mutex
	ja4    string
	alpn   string
	h2     string
	grease bool
}

func newRecorder(hello clientHello) *Recorder {
	return &Recorder{
		ja4:    ja4TLS(hello),
		alpn:   strings.Join(hello.alpn, ","),
		h2:     "-",
		grease: hello.hasGREASE,
	}
}

// Class is the opaque v1 tuple. Empty when JA4 was never computed.
func (r *Recorder) Class() string {
	if r == nil {
		return ""
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ja4 == "" {
		return ""
	}
	h2 := r.h2
	if h2 == "" {
		h2 = "-"
	}
	return r.ja4 + "|" + h2 + "|" + r.alpn
}

func (r *Recorder) setH2(field string) {
	if r == nil || field == "" || field == "-" {
		return
	}
	r.mu.Lock()
	r.h2 = field
	r.mu.Unlock()
}

func (r *Recorder) libraryStack() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.grease {
		return true
	}
	return ja4CipherCount(r.ja4) < 13
}

// WithRecorder stores the connection recorder on ctx.
func WithRecorder(ctx context.Context, rec *Recorder) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if rec == nil {
		return ctx
	}
	return context.WithValue(ctx, recorderKey{}, rec)
}

// WithStackClass stores a completed opaque class. Tests and WAF fixtures
// use this when no live TLS connection exists.
func WithStackClass(ctx context.Context, class string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	class = strings.TrimSpace(class)
	if class == "" {
		return ctx
	}
	return context.WithValue(ctx, classKey{}, class)
}

// WithConn copies a peeked ClientHello recorder from conn onto ctx.
func WithConn(ctx context.Context, conn net.Conn) context.Context {
	return WithRecorder(ctx, recorderFromConn(conn))
}

// FromRequest returns the opaque stack_class for this request, or empty
// when the agent must emit nothing (HTTP, pass-through, Cloudflare in
// front, or no terminate-time observation).
func FromRequest(r *http.Request) string {
	if r == nil || BehindCloudflare(r) {
		return ""
	}
	return ClassFromContext(r.Context())
}

// ClassFromContext returns a stored class or recorder tuple.
func ClassFromContext(ctx context.Context) string {
	if rec := RecorderFromContext(ctx); rec != nil {
		return rec.Class()
	}
	if ctx == nil {
		return ""
	}
	class, _ := ctx.Value(classKey{}).(string)
	return class
}

// RecorderFromContext returns the live connection recorder, if any.
func RecorderFromContext(ctx context.Context) *Recorder {
	if ctx == nil {
		return nil
	}
	rec, _ := ctx.Value(recorderKey{}).(*Recorder)
	return rec
}

// BehindCloudflare reports a Cloudflare origin hop. The ClientHello this
// process saw is Cloudflare's, not the browser's.
func BehindCloudflare(r *http.Request) bool {
	return r != nil && strings.TrimSpace(r.Header.Get(cloudflareConnectingIP)) != ""
}

// CanEmit is the terminate-only gate used by tests and callers that know
// the listen mode. Pass-through / HTTP / Cloudflare origin must be false.
func CanEmit(terminated, behindCloudflare bool) bool {
	return terminated && !behindCloudflare
}

// BrowserLikeUA reports a User-Agent that claims to be a browser.
// Honest library tokens (go-http-client, python-requests, curl) are not
// browser-like even if they mention Mozilla in passing.
func BrowserLikeUA(userAgent string) bool {
	lower := strings.ToLower(userAgent)
	for _, token := range []string{
		"go-http-client",
		"python-requests",
		"python-urllib",
		"curl/",
		"wget/",
		"libwww-perl",
	} {
		if strings.Contains(lower, token) {
			return false
		}
	}
	if !strings.Contains(lower, "mozilla/") {
		return false
	}
	return strings.Contains(lower, "chrome/") ||
		strings.Contains(lower, "firefox/") ||
		strings.Contains(lower, "safari/") ||
		strings.Contains(lower, "edg/")
}

// Mismatch reports a stack_class that disagrees with a browser-like
// User-Agent. Difficulty must not rise from UA alone: honest library UAs
// never mismatch, and a missing class only mismatches when we terminated
// the client (not when Cloudflare sits in front).
func Mismatch(stackClass, userAgent string, terminated bool) bool {
	if !terminated || !BrowserLikeUA(userAgent) {
		return false
	}
	if strings.TrimSpace(stackClass) == "" {
		return true
	}
	return looksLibraryJA4(stackClass)
}

// MismatchFromRequest is the #114 hook input: terminated client TLS plus
// observed class (or its absence) versus User-Agent.
func MismatchFromRequest(r *http.Request) bool {
	if r == nil || r.TLS == nil || BehindCloudflare(r) {
		return false
	}
	if rec := RecorderFromContext(r.Context()); rec != nil {
		if !BrowserLikeUA(r.UserAgent()) {
			return false
		}
		if rec.Class() == "" {
			return true
		}
		return rec.libraryStack()
	}
	return Mismatch(FromRequest(r), r.UserAgent(), true)
}

func looksLibraryJA4(stackClass string) bool {
	ja4, _, _ := strings.Cut(stackClass, "|")
	return ja4CipherCount(ja4) < 13
}

func ja4CipherCount(ja4 string) int {
	a, _, _ := strings.Cut(ja4, "_")
	if len(a) < 10 {
		return 0
	}
	n, err := strconv.Atoi(a[4:6])
	if err != nil {
		return 0
	}
	return n
}

func recorderFromConn(conn net.Conn) *Recorder {
	for i := 0; i < 8 && conn != nil; i++ {
		switch c := conn.(type) {
		case *helloConn:
			return c.rec
		case *h2Conn:
			return c.rec
		case interface{ NetConn() net.Conn }:
			next := c.NetConn()
			if next == nil || next == conn {
				return nil
			}
			conn = next
		default:
			return nil
		}
	}
	return nil
}
