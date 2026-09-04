package fingerprint

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"sync"

	"golang.org/x/net/http2"
)

// WrapListener peeks the TLS ClientHello on each accepted connection
// before crypto/tls reads it. The same bytes are replayed into the
// handshake. Certificate selection is unchanged.
func WrapListener(ln net.Listener) net.Listener {
	if ln == nil {
		return nil
	}
	return &helloListener{Listener: ln}
}

type helloListener struct {
	net.Listener
}

func (l *helloListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	raw, peekErr := peekClientHello(conn)
	replay := &replayConn{Conn: conn, r: io.MultiReader(bytes.NewReader(raw), conn)}
	if peekErr != nil || len(raw) == 0 {
		return replay, nil
	}
	hello, err := parseClientHello(raw)
	if err != nil {
		return replay, nil
	}
	return &helloConn{Conn: replay, rec: newRecorder(hello)}, nil
}

type replayConn struct {
	net.Conn
	r io.Reader
}

func (c *replayConn) Read(p []byte) (int, error) {
	return c.r.Read(p)
}

type helloConn struct {
	net.Conn
	rec *Recorder
}

func (c *helloConn) NetConn() net.Conn { return c.Conn }

// ConfigureHTTP2 installs an x/net/http2 server that records the first
// SETTINGS / WINDOW_UPDATE / PRIORITY / HEADERS, then serves the request.
// Call it after TLSConfig is set and before ServeTLS. GetCertificate is
// not modified.
func ConfigureHTTP2(server *http.Server) error {
	if server == nil {
		return nil
	}
	h2s := &http2.Server{}
	if err := http2.ConfigureServer(server, h2s); err != nil {
		return err
	}
	if server.TLSNextProto == nil {
		server.TLSNextProto = make(map[string]func(*http.Server, *tls.Conn, http.Handler))
	}
	server.TLSNextProto["h2"] = func(s *http.Server, c *tls.Conn, h http.Handler) {
		var ctx context.Context
		if bc, ok := h.(interface{ BaseContext() context.Context }); ok {
			ctx = bc.BaseContext()
		}
		rec := recorderFromConn(c)
		if rec == nil {
			rec = RecorderFromContext(ctx)
		}
		h2s.ServeConn(newH2Conn(c, rec), &http2.ServeConnOpts{
			Context:    ctx,
			Handler:    h,
			BaseConfig: s,
		})
	}
	return nil
}

type h2Conn struct {
	*tls.Conn
	rec    *Recorder
	h2     *h2Fingerprint
	buf    []byte
	mu     sync.Mutex
	closed bool
}

func newH2Conn(c *tls.Conn, rec *Recorder) net.Conn {
	return &h2Conn{Conn: c, rec: rec, h2: &h2Fingerprint{}}
}

func (c *h2Conn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if n > 0 && c.h2 != nil && !c.h2.finished() {
		c.feed(p[:n])
	}
	return n, err
}

func (c *h2Conn) feed(chunk []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.h2.finished() {
		return
	}
	c.buf = append(c.buf, chunk...)
	if !c.h2.prefaceSeen {
		if len(c.buf) < len(h2Preface) {
			return
		}
		if string(c.buf[:len(h2Preface)]) != h2Preface {
			c.h2.mu.Lock()
			c.h2.done = true
			c.h2.mu.Unlock()
			return
		}
		c.buf = c.buf[len(h2Preface):]
		c.h2.mu.Lock()
		c.h2.prefaceSeen = true
		c.h2.mu.Unlock()
	}
	for !c.h2.finished() && len(c.buf) >= 9 {
		length := int(c.buf[0])<<16 | int(c.buf[1])<<8 | int(c.buf[2])
		need := 9 + length
		if length < 0 || need > 16<<20 {
			c.h2.mu.Lock()
			c.h2.done = true
			c.h2.mu.Unlock()
			return
		}
		if len(c.buf) < need {
			return
		}
		frameType := c.buf[3]
		flags := c.buf[4]
		streamID := uint32(c.buf[5])<<24 | uint32(c.buf[6])<<16 | uint32(c.buf[7])<<8 | uint32(c.buf[8])
		streamID &= 0x7fffffff
		payload := c.buf[9:need]
		c.buf = c.buf[need:]
		switch frameType {
		case frameSettings:
			c.h2.recordSettings(payload, flags)
		case frameWindowUpdate:
			c.h2.recordWindowUpdate(streamID, payload)
		case framePriority:
			c.h2.recordPriority(streamID, payload)
		case frameHeaders:
			c.h2.recordHeaders(payload, flags)
		case frameContinuation:
			c.h2.recordContinuation(payload, flags)
		}
	}
	if c.h2.done {
		c.rec.setH2(c.h2.field())
		c.buf = nil
	}
}

func (c *h2Conn) Close() error {
	c.mu.Lock()
	if !c.closed && c.rec != nil && c.h2 != nil && c.h2.prefaceSeen {
		c.rec.setH2(c.h2.field())
	}
	c.closed = true
	c.mu.Unlock()
	return c.Conn.Close()
}
