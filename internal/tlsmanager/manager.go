// Package tlsmanager provides safe, hot-reloadable TLS certificate selection.
package tlsmanager

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

// DomainCertificate is PEM-encoded certificate material for one DNS name. A
// Domain may be an exact name (api.example.com) or a single-label wildcard
// (*.example.com).
type DomainCertificate struct {
	Domain         string
	CertificatePEM string
	PrivateKeyPEM  string
}

type certificateSet struct {
	exact     map[string]*tls.Certificate
	wildcards map[string]*tls.Certificate
}

type acmeState struct {
	manager *autocert.Manager
	hosts   map[string]struct{}
}

// Manager selects immutable certificate snapshots through tls.Config's
// GetCertificate callback. It always prefers a matching per-domain
// certificate over its static fallback.
type Manager struct {
	reloadMu     sync.Mutex
	certificates atomic.Pointer[certificateSet]
	fallback     atomic.Pointer[tls.Certificate]
	acme         atomic.Pointer[acmeState]
}

// New creates a certificate manager. fallback may be nil when every accepted
// server name has a per-domain certificate or ACME configuration. Callers
// should not mutate fallback after passing it to New.
func New(fallback *tls.Certificate) *Manager {
	manager := &Manager{}
	manager.certificates.Store(&certificateSet{
		exact:     make(map[string]*tls.Certificate),
		wildcards: make(map[string]*tls.Certificate),
	})
	manager.SetFallback(fallback)
	return manager
}

// NewFromPEM creates a manager with a static fallback loaded from PEM data.
func NewFromPEM(certificatePEM, privateKeyPEM string) (*Manager, error) {
	certificate, err := tls.X509KeyPair([]byte(certificatePEM), []byte(privateKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("load fallback certificate: %w", err)
	}
	return New(&certificate), nil
}

// SetFallback replaces the static fallback certificate. Passing nil removes
// the fallback. The replacement is published atomically.
func (m *Manager) SetFallback(certificate *tls.Certificate) {
	if m == nil {
		return
	}
	if certificate == nil {
		m.fallback.Store(nil)
		return
	}
	m.fallback.Store(cloneCertificate(certificate))
}

// Reload parses records into a new immutable certificate snapshot and then
// publishes it atomically. Records are authoritative: a previously configured
// domain omitted from records is removed. If a record has an invalid domain or
// invalid PEM, that record does not replace its previous certificate; the
// method still publishes all valid records and returns an error describing the
// rejected entries. This preserves last-known-good credentials without exposing
// callers to a partially-built snapshot.
func (m *Manager) Reload(records []DomainCertificate) error {
	if m == nil {
		return errors.New("tls manager is nil")
	}

	m.reloadMu.Lock()
	defer m.reloadMu.Unlock()

	current := m.certificates.Load()
	next := &certificateSet{
		exact:     make(map[string]*tls.Certificate, len(records)),
		wildcards: make(map[string]*tls.Certificate, len(records)),
	}
	var reloadErrors []error
	for _, record := range records {
		domain, wildcard, err := normalizeCertificateDomain(record.Domain)
		if err != nil {
			reloadErrors = append(reloadErrors, fmt.Errorf("certificate domain %q: %w", record.Domain, err))
			continue
		}

		certificate, err := tls.X509KeyPair([]byte(record.CertificatePEM), []byte(record.PrivateKeyPEM))
		if err != nil {
			reloadErrors = append(reloadErrors, fmt.Errorf("load certificate for %q: %w", domain, err))
			// If an earlier record in this reload was valid, it is the newest
			// known-good value. Otherwise retain the value from the previous
			// snapshot, when there is one.
			if wildcard {
				if _, exists := next.wildcards[domain]; !exists && current != nil {
					if previous := current.wildcards[domain]; previous != nil {
						next.wildcards[domain] = previous
					}
				}
			} else if _, exists := next.exact[domain]; !exists && current != nil {
				if previous := current.exact[domain]; previous != nil {
					next.exact[domain] = previous
				}
			}
			continue
		}

		if wildcard {
			next.wildcards[domain] = &certificate
		} else {
			next.exact[domain] = &certificate
		}
	}

	m.certificates.Store(next)
	if len(reloadErrors) > 0 {
		return fmt.Errorf("reload TLS certificates: %w", errors.Join(reloadErrors...))
	}
	return nil
}

// GetCertificate implements tls.Config.GetCertificate. Exact names take
// precedence over wildcard names, which match exactly one DNS label. ACME is
// consulted only for explicitly configured hosts, before the static fallback.
func (m *Manager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	if m == nil {
		return nil, errors.New("tls manager is nil")
	}

	serverName := ""
	if hello != nil {
		serverName = normalizeServerName(hello.ServerName)
	}

	// tls-alpn-01 must receive autocert's temporary challenge certificate even
	// when a static certificate exists for the same hostname.
	if state := m.acme.Load(); state != nil && hello != nil && isACMEChallenge(hello) {
		if _, configured := state.hosts[serverName]; configured {
			certificate, err := state.manager.GetCertificate(normalizedHello(hello, serverName))
			if err != nil {
				return nil, fmt.Errorf("get ACME challenge certificate for %q: %w", serverName, err)
			}
			if certificate != nil {
				return certificate, nil
			}
		}
	}

	if snapshot := m.certificates.Load(); snapshot != nil {
		if certificate := snapshot.certificateFor(serverName); certificate != nil {
			return certificate, nil
		}
	}

	if state := m.acme.Load(); state != nil && hello != nil {
		if _, configured := state.hosts[serverName]; configured {
			certificate, err := state.manager.GetCertificate(normalizedHello(hello, serverName))
			if err != nil {
				return nil, fmt.Errorf("get ACME certificate for %q: %w", serverName, err)
			}
			if certificate != nil {
				return certificate, nil
			}
		}
	}

	if fallback := m.fallback.Load(); fallback != nil {
		return fallback, nil
	}
	return nil, fmt.Errorf("no TLS certificate configured for %q", serverName)
}

func (s *certificateSet) certificateFor(serverName string) *tls.Certificate {
	if s == nil || serverName == "" {
		return nil
	}
	if certificate := s.exact[serverName]; certificate != nil {
		return certificate
	}
	labels := strings.Split(serverName, ".")
	if len(labels) < 2 {
		return nil
	}
	return s.wildcards["*."+strings.Join(labels[1:], ".")]
}

// ACMEConfig configures optional automatic certificate management. Hosts must
// be an explicit list of exact DNS names; requiring the list prevents a client
// supplied SNI name from triggering arbitrary certificate issuance.
type ACMEConfig struct {
	CacheDir string
	Email    string
	Hosts    []string
	Prompt   func(tosURL string) bool
	// Client optionally provides an ACME client (for example, a staging CA).
	// It must not be mutated after EnableACME returns.
	Client *acme.Client
	// DirectoryURL configures a new default ACME client. It cannot be used
	// together with Client; set Client.DirectoryURL directly in that case.
	DirectoryURL string
	RenewBefore  time.Duration
}

// EnableACME enables autocert with an encrypted, persistent cache. The
// NETGOAT_ACME_CACHE_KEY environment variable must be a base64-encoded
// 32-byte key. Prompt is required so callers explicitly control acceptance of
// the certificate authority's terms of service.
func (m *Manager) EnableACME(config ACMEConfig) error {
	if m == nil {
		return errors.New("tls manager is nil")
	}
	if len(config.Hosts) == 0 {
		return errors.New("ACME requires at least one allowed host")
	}
	if config.Prompt == nil {
		return errors.New("ACME requires a terms-of-service prompt")
	}

	hosts := make([]string, 0, len(config.Hosts))
	hostSet := make(map[string]struct{}, len(config.Hosts))
	for _, rawHost := range config.Hosts {
		host, wildcard, err := normalizeCertificateDomain(rawHost)
		if err != nil || wildcard {
			if wildcard && err == nil {
				err = errors.New("wildcard hosts are not supported")
			}
			return fmt.Errorf("invalid ACME host %q: %w", rawHost, err)
		}
		if net.ParseIP(host) != nil {
			return fmt.Errorf("invalid ACME host %q: IP addresses are not supported", rawHost)
		}
		if _, duplicate := hostSet[host]; duplicate {
			continue
		}
		hostSet[host] = struct{}{}
		hosts = append(hosts, host)
	}

	cache, err := NewEncryptedCacheFromEnv(config.CacheDir)
	if err != nil {
		return fmt.Errorf("configure encrypted ACME cache: %w", err)
	}

	client := config.Client
	if config.DirectoryURL != "" {
		if client != nil {
			return errors.New("ACME Client and DirectoryURL cannot both be set")
		}
		client = &acme.Client{DirectoryURL: config.DirectoryURL}
	}

	m.acme.Store(&acmeState{
		manager: &autocert.Manager{
			Prompt:      config.Prompt,
			Cache:       cache,
			HostPolicy:  autocert.HostWhitelist(hosts...),
			Email:       config.Email,
			Client:      client,
			RenewBefore: config.RenewBefore,
		},
		hosts: hostSet,
	})
	return nil
}

// DisableACME stops using ACME for new TLS handshakes. It does not remove the
// encrypted cache, so a later EnableACME call can resume renewals.
func (m *Manager) DisableACME() {
	if m != nil {
		m.acme.Store(nil)
	}
}

// HTTPHandler returns a handler that serves ACME HTTP-01 challenges when ACME
// is enabled, and delegates all other requests to fallback. When ACME is not
// enabled, fallback is returned unchanged.
func (m *Manager) HTTPHandler(fallback http.Handler) http.Handler {
	if m == nil {
		return fallback
	}
	if state := m.acme.Load(); state != nil {
		return state.manager.HTTPHandler(fallback)
	}
	return fallback
}

// TLSConfig returns a TLS configuration using this manager. Call it after
// EnableACME so tls-alpn-01 is advertised when automatic issuance is active.
func (m *Manager) TLSConfig() *tls.Config {
	config := &tls.Config{
		MinVersion:     tls.VersionTLS12,
		GetCertificate: m.GetCertificate,
	}
	if m != nil && m.acme.Load() != nil {
		config.NextProtos = []string{"h2", "http/1.1", acme.ALPNProto}
	}
	return config
}

func normalizedHello(hello *tls.ClientHelloInfo, serverName string) *tls.ClientHelloInfo {
	copy := *hello
	copy.ServerName = serverName
	return &copy
}

func isACMEChallenge(hello *tls.ClientHelloInfo) bool {
	return len(hello.SupportedProtos) == 1 && hello.SupportedProtos[0] == acme.ALPNProto
}

func normalizeServerName(serverName string) string {
	domain, wildcard, err := normalizeCertificateDomain(serverName)
	if err != nil || wildcard {
		return ""
	}
	return domain
}

func normalizeCertificateDomain(domain string) (string, bool, error) {
	domain = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), "."))
	if domain == "" {
		return "", false, errors.New("domain is empty")
	}
	if strings.HasPrefix(domain, "*.") {
		suffix := strings.TrimPrefix(domain, "*.")
		if err := validateDNSName(suffix); err != nil {
			return "", true, err
		}
		return "*." + suffix, true, nil
	}
	if strings.Contains(domain, "*") {
		return "", false, errors.New("wildcard must be the complete leftmost label")
	}
	if err := validateDNSName(domain); err != nil {
		return "", false, err
	}
	return domain, false, nil
}

func validateDNSName(domain string) error {
	if len(domain) > 253 {
		return errors.New("domain exceeds 253 characters")
	}
	for _, label := range strings.Split(domain, ".") {
		if label == "" {
			return errors.New("domain contains an empty label")
		}
		if len(label) > 63 {
			return errors.New("domain contains a label longer than 63 characters")
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return errors.New("domain label begins or ends with a hyphen")
		}
		for _, character := range label {
			if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '-' {
				continue
			}
			return fmt.Errorf("domain contains invalid character %q", character)
		}
	}
	return nil
}

func cloneCertificate(certificate *tls.Certificate) *tls.Certificate {
	clone := *certificate
	clone.Certificate = make([][]byte, len(certificate.Certificate))
	for index, chain := range certificate.Certificate {
		clone.Certificate[index] = append([]byte(nil), chain...)
	}
	clone.OCSPStaple = append([]byte(nil), certificate.OCSPStaple...)
	clone.SignedCertificateTimestamps = make([][]byte, len(certificate.SignedCertificateTimestamps))
	for index, timestamp := range certificate.SignedCertificateTimestamps {
		clone.SignedCertificateTimestamps[index] = append([]byte(nil), timestamp...)
	}
	clone.SupportedSignatureAlgorithms = append([]tls.SignatureScheme(nil), certificate.SupportedSignatureAlgorithms...)
	return &clone
}
