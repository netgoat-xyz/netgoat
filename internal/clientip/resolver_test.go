package clientip

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewValidatesTrustedProxies(t *testing.T) {
	resolver, err := New([]string{"10.0.0.5", "192.0.2.42/24", "2001:db8::/32", "::ffff:198.51.100.0/120"})
	if err != nil {
		t.Fatalf("New returned an error for valid entries: %v", err)
	}
	if len(resolver.trusted) != 4 {
		t.Fatalf("trusted prefix count = %d, want 4", len(resolver.trusted))
	}

	for _, entry := range []string{"", "proxy.example.com", "10.0.0.1/99", "fe80::1%eth0"} {
		t.Run(entry, func(t *testing.T) {
			if _, err := New([]string{entry}); err == nil {
				t.Fatalf("New(%q) succeeded, want an error", entry)
			}
		})
	}
}

func TestClientIPIgnoresHeadersByDefault(t *testing.T) {
	resolver, err := New(nil)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	req := requestFrom("203.0.113.10:4321")
	req.Header.Set("Forwarded", "for=198.51.100.1")
	req.Header.Set("X-Forwarded-For", "198.51.100.1")

	if got := resolver.ClientIP(req); got != "203.0.113.10" {
		t.Fatalf("ClientIP = %q, want direct peer", got)
	}
}

func TestClientIPIgnoresHeadersFromUntrustedPeer(t *testing.T) {
	resolver := mustResolver(t, "10.0.0.0/8")
	req := requestFrom("203.0.113.10:4321")
	req.Header.Set("X-Forwarded-For", "198.51.100.1")

	if got := resolver.ClientIP(req); got != "203.0.113.10" {
		t.Fatalf("ClientIP = %q, want direct peer", got)
	}
}

func TestClientIPWalksTrustedXForwardedForRightToLeft(t *testing.T) {
	resolver := mustResolver(t, "10.0.0.0/8", "192.0.2.0/24")
	req := requestFrom("10.0.0.2:443")
	req.Header.Set("X-Forwarded-For", "198.51.100.66, 203.0.113.9, 192.0.2.44")

	if got := resolver.ClientIP(req); got != "203.0.113.9" {
		t.Fatalf("ClientIP = %q, want first untrusted address from the right", got)
	}
}

func TestClientIPAcceptsMultipleXForwardedForFieldsAndPorts(t *testing.T) {
	resolver := mustResolver(t, "10.0.0.0/8", "192.0.2.0/24")
	req := requestFrom("10.0.0.2:443")
	req.Header.Add("X-Forwarded-For", "198.51.100.8:1234")
	req.Header.Add("X-Forwarded-For", "192.0.2.44")

	if got := resolver.ClientIP(req); got != "198.51.100.8" {
		t.Fatalf("ClientIP = %q, want 198.51.100.8", got)
	}
}

func TestClientIPParsesRFC7239Forwarded(t *testing.T) {
	resolver := mustResolver(t, "10.0.0.0/8", "2001:db8::/32")
	req := requestFrom("10.0.0.2:443")
	req.Header.Set("Forwarded", `for=198.51.100.7;proto=https, for="[2001:db8::2]:8443";by=10.0.0.2`)

	if got := resolver.ClientIP(req); got != "198.51.100.7" {
		t.Fatalf("ClientIP = %q, want 198.51.100.7", got)
	}
}

func TestClientIPAcceptsMatchingForwardingHeaders(t *testing.T) {
	resolver := mustResolver(t, "10.0.0.0/8", "192.0.2.0/24")
	req := requestFrom("10.0.0.2:443")
	req.Header.Set("Forwarded", "for=198.51.100.7, for=192.0.2.44")
	req.Header.Set("X-Forwarded-For", "198.51.100.7, 192.0.2.44")

	if got := resolver.ClientIP(req); got != "198.51.100.7" {
		t.Fatalf("ClientIP = %q, want 198.51.100.7", got)
	}
}

func TestClientIPRejectsConflictingForwardingHeaders(t *testing.T) {
	resolver := mustResolver(t, "10.0.0.0/8")
	req := requestFrom("10.0.0.2:443")
	req.Header.Set("Forwarded", "for=198.51.100.7")
	req.Header.Set("X-Forwarded-For", "203.0.113.8")

	if got := resolver.ClientIP(req); got != "10.0.0.2" {
		t.Fatalf("ClientIP = %q, want trusted peer after conflicting headers", got)
	}
}

func TestClientIPRejectsMalformedXForwardedForChain(t *testing.T) {
	resolver := mustResolver(t, "10.0.0.0/8", "192.0.2.0/24")
	for _, value := range []string{
		"198.51.100.7, garbage, 192.0.2.44",
		"198.51.100.7,,192.0.2.44",
		"unknown, 192.0.2.44",
		"_hidden, 192.0.2.44",
		"198.51.100.7, [2001:db8::1",
		"198.51.100.7\v",
	} {
		t.Run(value, func(t *testing.T) {
			req := requestFrom("10.0.0.2:443")
			req.Header.Set("X-Forwarded-For", value)
			if got := resolver.ClientIP(req); got != "10.0.0.2" {
				t.Fatalf("ClientIP = %q, want peer for malformed chain", got)
			}
		})
	}
}

func TestClientIPRejectsMalformedForwardedChain(t *testing.T) {
	resolver := mustResolver(t, "10.0.0.0/8")
	for _, value := range []string{
		"for=198.51.100.7, proto=https",
		"for=198.51.100.7;for=203.0.113.8",
		"for=unknown",
		"for=_hidden",
		"for=198.51.100.7;bad name=value",
		"for=[2001:db8::1]",
		`for="[2001:db8::1]`,
		"for=198.51.100.7,",
	} {
		t.Run(value, func(t *testing.T) {
			req := requestFrom("10.0.0.2:443")
			req.Header.Set("Forwarded", value)
			if got := resolver.ClientIP(req); got != "10.0.0.2" {
				t.Fatalf("ClientIP = %q, want peer for malformed chain", got)
			}
		})
	}
}

func TestClientIPFallsBackWhenChainContainsOnlyTrustedAddresses(t *testing.T) {
	resolver := mustResolver(t, "10.0.0.0/8")
	req := requestFrom("10.0.0.2:443")
	req.Header.Set("X-Forwarded-For", "10.0.0.10, 10.0.0.11")

	if got := resolver.ClientIP(req); got != "10.0.0.2" {
		t.Fatalf("ClientIP = %q, want peer when no untrusted client is present", got)
	}
}

func TestClientIPNormalizesIPv6AndMappedIPv4(t *testing.T) {
	resolver := mustResolver(t, "2001:db8::1", "198.51.100.0/24")

	ipv6 := requestFrom("[2001:db8::1]:443")
	ipv6.Header.Set("X-Forwarded-For", "[2001:db9::5]:1234")
	if got := resolver.ClientIP(ipv6); got != "2001:db9::5" {
		t.Fatalf("IPv6 ClientIP = %q, want 2001:db9::5", got)
	}

	mapped := requestFrom("[::ffff:198.51.100.2]:443")
	mapped.Header.Set("X-Forwarded-For", "::ffff:203.0.113.9")
	if got := resolver.ClientIP(mapped); got != "203.0.113.9" {
		t.Fatalf("mapped ClientIP = %q, want 203.0.113.9", got)
	}
}

func TestClientIPHandlesNilAndMalformedRequests(t *testing.T) {
	resolver := mustResolver(t, "10.0.0.0/8")
	if got := resolver.ClientIP(nil); got != "" {
		t.Fatalf("ClientIP(nil) = %q, want empty", got)
	}
	req := requestFrom("not-an-ip:1234")
	req.Header.Set("X-Forwarded-For", "198.51.100.7")
	if got := resolver.ClientIP(req); got != "not-an-ip:1234" {
		t.Fatalf("ClientIP with malformed peer = %q, want original RemoteAddr", got)
	}
}

func TestClientIPBoundsForwardingChains(t *testing.T) {
	resolver := mustResolver(t, "10.0.0.0/8")
	addresses := make([]string, maxForwardedAddresses+1)
	for i := range addresses {
		addresses[i] = fmt.Sprintf("192.0.2.%d", i%250+1)
	}
	req := requestFrom("10.0.0.2:443")
	req.Header.Set("X-Forwarded-For", strings.Join(addresses, ","))
	if got := resolver.ClientIP(req); got != "10.0.0.2" {
		t.Fatalf("ClientIP = %q, want peer for oversized chain", got)
	}
}

func requestFrom(remoteAddr string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
	req.RemoteAddr = remoteAddr
	return req
}

func mustResolver(t *testing.T, trusted ...string) *Resolver {
	t.Helper()
	resolver, err := New(trusted)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	return resolver
}
