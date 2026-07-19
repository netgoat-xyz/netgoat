// Package clientip resolves client addresses across explicitly trusted proxies.
package clientip

import (
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"strings"
)

const (
	maxForwardedHeaderBytes = 8 << 10
	maxForwardedAddresses   = 64
)

// Resolver resolves client IP addresses using an immutable trusted-proxy set.
// An empty trusted-proxy set always ignores forwarding headers.
type Resolver struct {
	trusted []netip.Prefix
}

// New constructs a Resolver from individual IP addresses or CIDR prefixes.
// Invalid and empty entries are rejected instead of being silently ignored.
func New(trustedProxies []string) (*Resolver, error) {
	resolver := &Resolver{trusted: make([]netip.Prefix, 0, len(trustedProxies))}
	seen := make(map[netip.Prefix]struct{}, len(trustedProxies))
	for _, raw := range trustedProxies {
		prefix, err := parseTrustedPrefix(raw)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[prefix]; exists {
			continue
		}
		seen[prefix] = struct{}{}
		resolver.trusted = append(resolver.trusted, prefix)
	}
	return resolver, nil
}

// ClientIP returns the request's client IP. Forwarding headers are considered
// only when the direct socket peer is trusted. A malformed or contradictory
// chain fails closed to the direct peer address.
func (r *Resolver) ClientIP(req *http.Request) string {
	if req == nil {
		return ""
	}

	peer, ok := parseAddress(req.RemoteAddr)
	if !ok {
		return strings.TrimSpace(req.RemoteAddr)
	}
	peerText := peer.String()
	if r == nil || !r.isTrusted(peer) {
		return peerText
	}

	forwardedValues := req.Header.Values("Forwarded")
	xffValues := req.Header.Values("X-Forwarded-For")
	if len(forwardedValues) == 0 && len(xffValues) == 0 {
		return peerText
	}

	var chain []netip.Addr
	if len(forwardedValues) > 0 {
		var err error
		chain, err = parseForwarded(forwardedValues)
		if err != nil {
			return peerText
		}
	}
	if len(xffValues) > 0 {
		xffChain, err := parseXForwardedFor(xffValues)
		if err != nil {
			return peerText
		}
		if chain != nil && !sameChain(chain, xffChain) {
			return peerText
		}
		chain = xffChain
	}

	for i := len(chain) - 1; i >= 0; i-- {
		if !r.isTrusted(chain[i]) {
			return chain[i].String()
		}
	}

	// A valid proxy chain must eventually identify an address outside the
	// trusted proxy set. If it does not, no client address was established.
	return peerText
}

func (r *Resolver) isTrusted(addr netip.Addr) bool {
	addr = addr.Unmap()
	for _, prefix := range r.trusted {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func parseTrustedPrefix(raw string) (netip.Prefix, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return netip.Prefix{}, errors.New("trusted proxy entry cannot be empty")
	}

	if strings.Contains(value, "/") {
		prefix, err := netip.ParsePrefix(value)
		if err != nil || prefix.Addr().Zone() != "" {
			return netip.Prefix{}, fmt.Errorf("invalid trusted proxy prefix %q", raw)
		}
		if prefix.Addr().Is4In6() {
			if prefix.Bits() < 96 {
				return netip.Prefix{}, fmt.Errorf("invalid IPv4-mapped trusted proxy prefix %q", raw)
			}
			prefix = netip.PrefixFrom(prefix.Addr().Unmap(), prefix.Bits()-96)
		}
		return prefix.Masked(), nil
	}

	addr, err := netip.ParseAddr(value)
	if err != nil || addr.Zone() != "" {
		return netip.Prefix{}, fmt.Errorf("invalid trusted proxy address %q", raw)
	}
	addr = addr.Unmap()
	return netip.PrefixFrom(addr, addr.BitLen()), nil
}

func parseAddress(raw string) (netip.Addr, bool) {
	value := trimOWS(raw)
	if value == "" {
		return netip.Addr{}, false
	}
	if addr, err := netip.ParseAddr(value); err == nil && addr.Zone() == "" {
		return addr.Unmap(), true
	}
	if addrPort, err := netip.ParseAddrPort(value); err == nil && addrPort.Addr().Zone() == "" {
		return addrPort.Addr().Unmap(), true
	}
	if len(value) > 2 && value[0] == '[' && value[len(value)-1] == ']' {
		if addr, err := netip.ParseAddr(value[1 : len(value)-1]); err == nil && addr.Zone() == "" {
			return addr.Unmap(), true
		}
	}
	return netip.Addr{}, false
}

func parseXForwardedFor(values []string) ([]netip.Addr, error) {
	if headerSize(values) > maxForwardedHeaderBytes {
		return nil, errors.New("X-Forwarded-For header is too large")
	}

	chain := make([]netip.Addr, 0, len(values))
	for _, value := range values {
		if containsInvalidHeaderControl(value) {
			return nil, errors.New("X-Forwarded-For contains a control character")
		}
		for _, raw := range strings.Split(value, ",") {
			if len(chain) >= maxForwardedAddresses {
				return nil, errors.New("X-Forwarded-For contains too many addresses")
			}
			addr, ok := parseAddress(raw)
			if !ok {
				return nil, fmt.Errorf("invalid X-Forwarded-For element %q", strings.TrimSpace(raw))
			}
			chain = append(chain, addr)
		}
	}
	if len(chain) == 0 {
		return nil, errors.New("X-Forwarded-For contains no addresses")
	}
	return chain, nil
}

func parseForwarded(values []string) ([]netip.Addr, error) {
	if headerSize(values) > maxForwardedHeaderBytes {
		return nil, errors.New("Forwarded header is too large")
	}

	chain := make([]netip.Addr, 0, len(values))
	for _, value := range values {
		elements, err := splitOutsideQuotes(value, ',')
		if err != nil {
			return nil, err
		}
		for _, element := range elements {
			if len(chain) >= maxForwardedAddresses {
				return nil, errors.New("Forwarded contains too many addresses")
			}
			addr, err := forwardedForAddress(element)
			if err != nil {
				return nil, err
			}
			chain = append(chain, addr)
		}
	}
	if len(chain) == 0 {
		return nil, errors.New("Forwarded contains no addresses")
	}
	return chain, nil
}

func forwardedForAddress(element string) (netip.Addr, error) {
	parameters, err := splitOutsideQuotes(element, ';')
	if err != nil {
		return netip.Addr{}, err
	}
	seen := make(map[string]struct{}, len(parameters))
	var forValue string
	for _, parameter := range parameters {
		parameter = trimOWS(parameter)
		name, value, found := strings.Cut(parameter, "=")
		name = strings.ToLower(trimOWS(name))
		value = trimOWS(value)
		if !found || !isToken(name) || value == "" {
			return netip.Addr{}, fmt.Errorf("invalid Forwarded parameter %q", parameter)
		}
		if _, duplicate := seen[name]; duplicate {
			return netip.Addr{}, fmt.Errorf("duplicate Forwarded parameter %q", name)
		}
		seen[name] = struct{}{}
		decoded, err := decodeForwardedValue(value)
		if err != nil {
			return netip.Addr{}, err
		}
		if name == "for" {
			forValue = decoded
		}
	}
	if forValue == "" {
		return netip.Addr{}, errors.New("Forwarded element is missing for parameter")
	}
	addr, ok := parseAddress(forValue)
	if !ok {
		return netip.Addr{}, fmt.Errorf("invalid Forwarded for value %q", forValue)
	}
	return addr, nil
}

func splitOutsideQuotes(value string, delimiter byte) ([]string, error) {
	if trimOWS(value) == "" {
		return nil, errors.New("forwarding header contains an empty element")
	}

	parts := make([]string, 0, 2)
	start := 0
	quoted := false
	escaped := false
	for i := 0; i < len(value); i++ {
		char := value[i]
		if char < 0x20 && char != '\t' || char == 0x7f {
			return nil, errors.New("forwarding header contains a control character")
		}
		switch char {
		case '\\':
			if quoted {
				escaped = !escaped
			}
		case '"':
			if !escaped {
				quoted = !quoted
			}
			escaped = false
		default:
			if escaped {
				escaped = false
			}
			if value[i] == delimiter && !quoted {
				part := trimOWS(value[start:i])
				if part == "" {
					return nil, errors.New("forwarding header contains an empty element")
				}
				parts = append(parts, part)
				start = i + 1
			}
		}
	}
	if quoted || escaped {
		return nil, errors.New("forwarding header contains an unterminated quoted string")
	}
	part := trimOWS(value[start:])
	if part == "" {
		return nil, errors.New("forwarding header contains an empty element")
	}
	return append(parts, part), nil
}

func decodeForwardedValue(value string) (string, error) {
	if value == "" {
		return "", errors.New("Forwarded value cannot be empty")
	}
	if value[0] != '"' {
		if !isToken(value) {
			return "", fmt.Errorf("invalid unquoted Forwarded value %q", value)
		}
		return value, nil
	}
	if len(value) < 2 || value[len(value)-1] != '"' {
		return "", errors.New("unterminated Forwarded quoted string")
	}

	var decoded strings.Builder
	decoded.Grow(len(value) - 2)
	for i := 1; i < len(value)-1; i++ {
		char := value[i]
		if char == '\\' {
			i++
			if i >= len(value)-1 {
				return "", errors.New("invalid Forwarded quoted escape")
			}
			char = value[i]
			if char < 0x20 || char == 0x7f {
				return "", errors.New("invalid character in Forwarded quoted string")
			}
			decoded.WriteByte(char)
			continue
		}
		if char < 0x20 || char == 0x7f || char == '"' {
			return "", errors.New("invalid character in Forwarded quoted string")
		}
		decoded.WriteByte(char)
	}
	return decoded.String(), nil
}

func isToken(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		char := value[i]
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' ||
			char >= '0' && char <= '9' {
			continue
		}
		switch char {
		case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
			continue
		default:
			return false
		}
	}
	return true
}

func containsInvalidHeaderControl(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] < 0x20 && value[i] != '\t' || value[i] == 0x7f {
			return true
		}
	}
	return false
}

func trimOWS(value string) string {
	return strings.Trim(value, " \t")
}

func sameChain(a, b []netip.Addr) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func headerSize(values []string) int {
	size := 0
	for _, value := range values {
		size += len(value)
	}
	return size
}
