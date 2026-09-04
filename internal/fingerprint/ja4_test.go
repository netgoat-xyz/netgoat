package fingerprint

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestJA4CipherHashMatchesFoxIOVector(t *testing.T) {
	ciphers := []uint16{
		0x002f, 0x0035, 0x009c, 0x009d, 0x1301, 0x1302, 0x1303,
		0xc013, 0xc014, 0xc02b, 0xc02c, 0xc02f, 0xc030, 0xcca8, 0xcca9,
	}
	got := truncatedSHA256(joinHex(ciphers))
	if got != "8daaf6152771" {
		t.Fatalf("cipher hash = %q, want 8daaf6152771", got)
	}
}

func TestJA4StripsGREASEAndIgnoresExtensionOrder(t *testing.T) {
	ciphers := []uint16{0x1301, 0x1302, 0x1303, 0xc02b, 0xc02f, 0x0a0a}
	extensions := []uint16{0x0000, 0x0010, 0x000d, 0x002b, 0x000a, 0x1a1a}
	alpn := []string{"h2", "http/1.1"}
	sig := []uint16{0x0403, 0x0804}
	versions := []uint16{0x0304, 0x0303}

	first := ja4TLS(clientHello{
		legacyVersion: 0x0303,
		ciphers:       filterGREASE(ciphers),
		extensions:    filterGREASE(extensions),
		alpn:          alpn,
		sigAlgs:       sig,
		versions:      versions,
		hasSNI:        true,
		hasGREASE:     true,
	})
	shuffled := ja4TLS(clientHello{
		legacyVersion: 0x0303,
		ciphers:       filterGREASE([]uint16{0x0a0a, 0xc02f, 0x1303, 0x1301, 0xc02b, 0x1302}),
		extensions:    filterGREASE([]uint16{0x000a, 0x1a1a, 0x002b, 0x0000, 0x000d, 0x0010}),
		alpn:          alpn,
		sigAlgs:       sig,
		versions:      versions,
		hasSNI:        true,
		hasGREASE:     true,
	})
	if first != shuffled {
		t.Fatalf("GREASE/order changed JA4: %q vs %q", first, shuffled)
	}
	if first[:1] != "t" {
		t.Fatalf("protocol prefix = %q, want t", first[:1])
	}
	if want := "t13d0504h2_"; len(first) < len(want) || first[:len(want)] != want {
		t.Fatalf("JA4 a-section = %q, want prefix %q (5 ciphers, 4 extensions after GREASE)", first, want)
	}
}

func TestJA4FromRawHelloStripsGREASE(t *testing.T) {
	raw := buildClientHello(helloSpec{
		version:    0x0303,
		ciphers:    []uint16{0x0a0a, 0x1301, 0x1302, 0x1303, 0xc02b, 0xc02f, 0x2a2a},
		serverName: "app.example.test",
		alpn:       []string{"h2", "http/1.1"},
		versions:   []uint16{0x0a0a, 0x0304, 0x0303},
		sigAlgs:    []uint16{0x0403, 0x0804},
		extensions: []uint16{0x000a, 0x1a1a, 0x000d, 0x002b},
	})
	hello, err := parseClientHello(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !hello.hasGREASE {
		t.Fatal("expected GREASE to be observed")
	}
	for _, suite := range hello.ciphers {
		if isGREASE(suite) {
			t.Fatalf("GREASE cipher %04x survived parse", suite)
		}
	}
	for _, ext := range hello.extensions {
		if isGREASE(ext) {
			t.Fatalf("GREASE extension %04x survived parse", ext)
		}
	}

	shuffled := buildClientHello(helloSpec{
		version:    0x0303,
		ciphers:    []uint16{0x1303, 0x2a2a, 0xc02f, 0x1301, 0x0a0a, 0xc02b, 0x1302},
		serverName: "app.example.test",
		alpn:       []string{"h2", "http/1.1"},
		versions:   []uint16{0x0304, 0x0a0a, 0x0303},
		sigAlgs:    []uint16{0x0403, 0x0804},
		extensions: []uint16{0x002b, 0x000d, 0x1a1a, 0x000a},
	})
	other, err := parseClientHello(shuffled)
	if err != nil {
		t.Fatal(err)
	}
	if ja4TLS(hello) != ja4TLS(other) {
		t.Fatalf("shuffled GREASE hello changed JA4: %q vs %q", ja4TLS(hello), ja4TLS(other))
	}
}

func TestSameChromeClassHelloSameKey(t *testing.T) {
	spec := helloSpec{
		version:    0x0303,
		ciphers:    []uint16{0x0a0a, 0x1301, 0x1302, 0x1303, 0xc02b, 0xc02f, 0xc02c, 0xc030, 0xcca9, 0xcca8, 0xc013, 0xc014, 0x009c, 0x009d, 0x002f, 0x0035, 0x1a1a},
		serverName: "app.example.test",
		alpn:       []string{"h2", "http/1.1"},
		versions:   []uint16{0x0a0a, 0x0304, 0x0303},
		sigAlgs:    []uint16{0x0403, 0x0804, 0x0401, 0x0503},
		extensions: []uint16{0x000a, 0x000d, 0x002b, 0x002d, 0x0033, 0x0017},
	}
	first, err := parseClientHello(buildClientHello(spec))
	if err != nil {
		t.Fatal(err)
	}
	second, err := parseClientHello(buildClientHello(spec))
	if err != nil {
		t.Fatal(err)
	}
	a := newRecorder(first).Class()
	b := newRecorder(second).Class()
	if a == "" || a != b {
		t.Fatalf("same chrome-class hello produced %q and %q", a, b)
	}
	if wantPrefix := "t13d"; a[:4] != wantPrefix {
		t.Fatalf("class %q, want TLS 1.3 domain prefix", a)
	}
}

func TestH1OnlyALPNUsesDashH2Field(t *testing.T) {
	hello, err := parseClientHello(buildClientHello(helloSpec{
		version:    0x0303,
		ciphers:    []uint16{0x1301, 0x1302},
		serverName: "app.example.test",
		alpn:       []string{"http/1.1"},
		versions:   []uint16{0x0304},
		sigAlgs:    []uint16{0x0403},
		extensions: []uint16{0x000a, 0x000d, 0x002b},
	}))
	if err != nil {
		t.Fatal(err)
	}
	class := newRecorder(hello).Class()
	if got := class; !hasH2Field(got, "-") {
		t.Fatalf("H1-only class = %q, want H2 field -", class)
	}
	if want := "http/1.1"; !containsALPN(class, want) {
		t.Fatalf("class = %q, want ALPN %s", class, want)
	}
}

func TestJA4ALPNNonAlphanumericUsesHex(t *testing.T) {
	if got := ja4ALPN([]string{string([]byte{0xab, 0xcd})}); got != "ad" {
		t.Fatalf("ja4ALPN(0xABCD) = %q, want ad", got)
	}
	if got := ja4ALPN([]string{"http/1.1"}); got != "h1" {
		t.Fatalf("ja4ALPN(http/1.1) = %q, want h1", got)
	}
	if got := ja4ALPN(nil); got != "00" {
		t.Fatalf("ja4ALPN(nil) = %q, want 00", got)
	}
}

func TestTruncatedEmptyHashIsZeros(t *testing.T) {
	if got := truncatedSHA256(""); got != "000000000000" {
		t.Fatalf("empty hash = %q", got)
	}
	sum := sha256.Sum256([]byte("002f"))
	if truncatedSHA256("002f") != hex.EncodeToString(sum[:])[:12] {
		t.Fatal("truncated hash mismatch")
	}
}

func filterGREASE(values []uint16) []uint16 {
	out := make([]uint16, 0, len(values))
	for _, value := range values {
		if !isGREASE(value) {
			out = append(out, value)
		}
	}
	return out
}

func hasH2Field(class, want string) bool {
	parts := splitClass(class)
	return len(parts) >= 2 && parts[1] == want
}

func containsALPN(class, alpn string) bool {
	parts := splitClass(class)
	return len(parts) >= 3 && parts[len(parts)-1] == alpn
}

func splitClass(class string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(class); i++ {
		if class[i] == '|' {
			parts = append(parts, class[start:i])
			start = i + 1
		}
	}
	return append(parts, class[start:])
}

type helloSpec struct {
	version    uint16
	ciphers    []uint16
	serverName string
	alpn       []string
	versions   []uint16
	sigAlgs    []uint16
	extensions []uint16
}

func buildClientHello(spec helloSpec) []byte {
	var body []byte
	body = appendU16(body, spec.version)
	body = append(body, make([]byte, 32)...)
	body = append(body, 0) // session id

	var ciphers []byte
	for _, suite := range spec.ciphers {
		ciphers = appendU16(ciphers, suite)
	}
	body = appendU16(body, uint16(len(ciphers)))
	body = append(body, ciphers...)
	body = append(body, 1, 0) // null compression

	var exts []byte
	if spec.serverName != "" {
		name := []byte(spec.serverName)
		inner := append([]byte{0}, appendU16(nil, uint16(len(name)))...)
		inner = append(inner, name...)
		payload := appendU16(nil, uint16(len(inner)))
		payload = append(payload, inner...)
		exts = appendExtension(exts, extServerName, payload)
	}
	if len(spec.alpn) > 0 {
		var list []byte
		for _, proto := range spec.alpn {
			list = append(list, byte(len(proto)))
			list = append(list, proto...)
		}
		payload := appendU16(nil, uint16(len(list)))
		payload = append(payload, list...)
		exts = appendExtension(exts, extALPN, payload)
	}
	if len(spec.sigAlgs) > 0 {
		var list []byte
		for _, alg := range spec.sigAlgs {
			list = appendU16(list, alg)
		}
		payload := appendU16(nil, uint16(len(list)))
		payload = append(payload, list...)
		exts = appendExtension(exts, extSignatureAlgorithms, payload)
	}
	if len(spec.versions) > 0 {
		var list []byte
		for _, ver := range spec.versions {
			list = appendU16(list, ver)
		}
		payload := append([]byte{byte(len(list))}, list...)
		exts = appendExtension(exts, extSupportedVersions, payload)
	}
	for _, ext := range spec.extensions {
		switch ext {
		case extServerName, extALPN, extSignatureAlgorithms, extSupportedVersions:
			continue
		}
		exts = appendExtension(exts, ext, nil)
	}
	body = appendU16(body, uint16(len(exts)))
	body = append(body, exts...)

	handshake := []byte{tlsClientHello, byte(len(body) >> 16), byte(len(body) >> 8), byte(len(body))}
	handshake = append(handshake, body...)
	record := []byte{tlsHandshake, 0x03, 0x03, byte(len(handshake) >> 8), byte(len(handshake))}
	return append(record, handshake...)
}

func appendExtension(dst []byte, typ uint16, payload []byte) []byte {
	dst = appendU16(dst, typ)
	dst = appendU16(dst, uint16(len(payload)))
	return append(dst, payload...)
}

func appendU16(dst []byte, v uint16) []byte {
	return binary.BigEndian.AppendUint16(dst, v)
}
