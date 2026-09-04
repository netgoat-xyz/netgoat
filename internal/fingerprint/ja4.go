package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// ja4TLS returns the FoxIO JA4 TLS fingerprint (BSD-3-Clause algorithm)
// for a TCP/TLS ClientHello. QUIC (`q`) is out of v1.
func ja4TLS(hello clientHello) string {
	a := ja4A(hello)
	b := truncatedSHA256(joinHex(sortedCopy(hello.ciphers)))
	c := ja4C(hello)
	return a + "_" + b + "_" + c
}

func ja4A(hello clientHello) string {
	sni := byte('i')
	if hello.hasSNI {
		sni = 'd'
	}
	cipherCount := min99(len(hello.ciphers))
	extCount := min99(len(hello.extensions))
	return fmt.Sprintf("t%s%c%02d%02d%s", tlsVersionToken(hello), sni, cipherCount, extCount, ja4ALPN(hello.alpn))
}

func ja4C(hello clientHello) string {
	hashed := make([]uint16, 0, len(hello.extensions))
	for _, ext := range hello.extensions {
		if ext == extServerName || ext == extALPN {
			continue
		}
		hashed = append(hashed, ext)
	}
	if len(hashed) == 0 {
		return "000000000000"
	}
	exts := joinHex(sortedCopy(hashed))
	if len(hello.sigAlgs) == 0 {
		return truncatedSHA256(exts)
	}
	return truncatedSHA256(exts + "_" + joinHex(hello.sigAlgs))
}

func ja4ALPN(protos []string) string {
	if len(protos) == 0 || protos[0] == "" {
		return "00"
	}
	first := []byte(protos[0])
	head, tail := first[0], first[len(first)-1]
	if isASCIIAlphanumeric(head) && isASCIIAlphanumeric(tail) {
		return string([]byte{head, tail})
	}
	encoded := hex.EncodeToString(first)
	return string([]byte{encoded[0], encoded[len(encoded)-1]})
}

func tlsVersionToken(hello clientHello) string {
	version := hello.legacyVersion
	for _, candidate := range hello.versions {
		if candidate > version {
			version = candidate
		}
	}
	switch version {
	case 0x0304:
		return "13"
	case 0x0303:
		return "12"
	case 0x0302:
		return "11"
	case 0x0301:
		return "10"
	case 0x0300:
		return "s3"
	case 0x0002:
		return "s2"
	case 0xfeff:
		return "d1"
	case 0xfefd:
		return "d2"
	case 0xfefc:
		return "d3"
	default:
		return "00"
	}
}

func joinHex(values []uint16) string {
	if len(values) == 0 {
		return ""
	}
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = fmt.Sprintf("%04x", value)
	}
	return strings.Join(parts, ",")
}

func sortedCopy(values []uint16) []uint16 {
	out := append([]uint16(nil), values...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func truncatedSHA256(value string) string {
	if value == "" {
		return "000000000000"
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:12]
}

func min99(n int) int {
	if n > 99 {
		return 99
	}
	if n < 0 {
		return 0
	}
	return n
}

func isASCIIAlphanumeric(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z'
}
