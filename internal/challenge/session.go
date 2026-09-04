package challenge

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
)

type sessionContextKey struct{}

// SessionBinding identifies the TLS session a challenge is bound to.
//
// SessionID is an opaque nonce minted when this process terminated TLS.
// Issue #113 (JA4+H2+ALPN stack_class) may later fold fingerprint material
// into SessionID; until then it is only the connection nonce.
//
// StackClassMismatch is the #113 hook. It stays false until that stack is
// wired. When true, difficulty is bumped independently of User-Agent.
//
// Subject is an optional authenticated identity (for example a user id) so
// zero-trust users behind the same TLS session stay distinct. It is not an IP.
type SessionBinding struct {
	SessionID          string
	Terminated         bool
	StackClassMismatch bool
	Subject            string
}

// Key is the verified-set lookup key. It is never an IP address.
func (b SessionBinding) Key() string {
	sessionID := boundedClone(b.SessionID, maxStoredBindingBytes)
	subject := boundedClone(b.Subject, maxStoredSubjectBytes)
	if subject == "" {
		return sessionID
	}
	return sessionID + "|sub:" + subject
}

func (b SessionBinding) usable() bool {
	return b.Terminated && strings.TrimSpace(b.SessionID) != ""
}

// WithConnSessionID stores a minted connection nonce on the connection
// context. The HTTP server should call this from ConnContext so every request
// on a terminated TLS connection shares one SessionID.
func WithConnSessionID(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Value(sessionContextKey{}).(string); ok {
		return ctx
	}
	return context.WithValue(ctx, sessionContextKey{}, GenerateID())
}

func sessionIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(sessionContextKey{}).(string)
	return id
}

// BindingFromRequest reports whether this process terminated TLS for the
// client and, if so, the session nonce for that connection.
//
// HTTP-only, TLS pass-through, and Cloudflare-in-front origins that never
// complete a handshake here yield Terminated=false. Callers must not issue
// PoW in that case: there is no session to bind.
func BindingFromRequest(r *http.Request) SessionBinding {
	if r == nil || r.TLS == nil {
		return SessionBinding{Terminated: false}
	}
	id := sessionIDFromContext(r.Context())
	if id == "" {
		id = sessionIDForTLS(r.TLS)
	}
	return SessionBinding{
		SessionID:  id,
		Terminated: true,
	}
}

func sessionIDForTLS(state *tls.ConnectionState) string {
	if state == nil {
		return ""
	}
	// Pointer identity is stable for the life of the net/http connection
	// (HTTP/1 and HTTP/2 both reuse one ConnectionState pointer). Production
	// traffic prefers the ConnContext nonce so verified keys are not reused if
	// a pointer is recycled after GC.
	return fmt.Sprintf("tls:%p", state)
}
