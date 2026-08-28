package config

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

// defaultPlaintextListenAddr preserves the historical HTTP bind when `listen`
// is omitted. That address is public (all interfaces), so ValidateListenSafety
// rejects it unless TLS is enabled or AllowInsecurePublicHTTP is set.
const defaultPlaintextListenAddr = ":8080"

// HTTPListenAddr returns the plaintext HTTP bind address used when SSL is
// disabled. An empty listen value defaults to ":8080".
func (c *Config) HTTPListenAddr() string {
	if c == nil {
		return defaultPlaintextListenAddr
	}
	if addr := strings.TrimSpace(c.Listen); addr != "" {
		return addr
	}
	return defaultPlaintextListenAddr
}

// ValidateListenSafety refuses to start a public plaintext HTTP listener
// unless the operator has set allow_insecure_public_http. Loopback binds
// (127.0.0.0/8, ::1, localhost) are allowed without TLS. TLS-enabled
// configurations skip this check because the HTTPS listener is used instead.
func (c *Config) ValidateListenSafety() error {
	if c == nil {
		return errors.New("configuration is required")
	}
	if c.SSL.Enabled {
		return nil
	}
	addr := c.HTTPListenAddr()
	loopback, err := listenAddrIsLoopback(addr)
	if err != nil {
		return fmt.Errorf("invalid listen address %q: %w", addr, err)
	}
	if loopback || c.AllowInsecurePublicHTTP {
		return nil
	}
	return fmt.Errorf("refusing to bind plaintext HTTP on public address %q; set listen to a loopback address (for example 127.0.0.1:8080), enable ssl.enabled, or set allow_insecure_public_http: true", addr)
}

func listenAddrIsLoopback(addr string) (bool, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return false, errors.New("listen address is empty")
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(port) == "" {
		return false, errors.New("listen address is missing a port")
	}
	host = strings.TrimSpace(host)
	if host == "" {
		// ":8080" binds every interface.
		return false, nil
	}
	if strings.EqualFold(host, "localhost") {
		return true, nil
	}
	ip := net.ParseIP(host)
	if ip == nil {
		// Unknown hostnames are treated as public rather than resolved, so a
		// DNS or hosts-file change cannot silently widen the bind.
		return false, nil
	}
	return ip.IsLoopback(), nil
}
