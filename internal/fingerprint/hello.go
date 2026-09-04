package fingerprint

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
)

const (
	maxClientHelloBytes = 32 << 10
	tlsRecordHeaderLen  = 5
	tlsHandshake        = 0x16
	tlsClientHello      = 0x01

	extServerName          = 0x0000
	extALPN                = 0x0010
	extSignatureAlgorithms = 0x000d
	extSupportedVersions   = 0x002b
)

var errNotClientHello = errors.New("fingerprint: not a TLS ClientHello")

// clientHello is the raw ClientHello fields JA4 and ALPN order need.
type clientHello struct {
	legacyVersion uint16
	ciphers       []uint16
	extensions    []uint16
	alpn          []string
	sigAlgs       []uint16
	versions      []uint16
	hasSNI        bool
	hasGREASE     bool
}

func parseClientHello(raw []byte) (clientHello, error) {
	var hello clientHello
	body, err := handshakeBody(raw)
	if err != nil {
		return hello, err
	}
	r := byteReader{b: body}
	var ok bool
	if hello.legacyVersion, ok = r.u16(); !ok {
		return hello, errNotClientHello
	}
	if !r.skip(32) { // random
		return hello, errNotClientHello
	}
	sessionLen, ok := r.u8()
	if !ok || !r.skip(int(sessionLen)) {
		return hello, errNotClientHello
	}
	cipherBytes, ok := r.u16()
	if !ok || cipherBytes%2 != 0 {
		return hello, errNotClientHello
	}
	cipherData, ok := r.bytes(int(cipherBytes))
	if !ok {
		return hello, errNotClientHello
	}
	for i := 0; i+1 < len(cipherData); i += 2 {
		suite := binary.BigEndian.Uint16(cipherData[i:])
		if isGREASE(suite) {
			hello.hasGREASE = true
			continue
		}
		hello.ciphers = append(hello.ciphers, suite)
	}
	compLen, ok := r.u8()
	if !ok || !r.skip(int(compLen)) {
		return hello, errNotClientHello
	}
	if r.remaining() == 0 {
		return hello, nil
	}
	extBytes, ok := r.u16()
	if !ok {
		return hello, errNotClientHello
	}
	extData, ok := r.bytes(int(extBytes))
	if !ok {
		return hello, errNotClientHello
	}
	er := byteReader{b: extData}
	for er.remaining() > 0 {
		extType, ok := er.u16()
		if !ok {
			return hello, errNotClientHello
		}
		extLen, ok := er.u16()
		if !ok {
			return hello, errNotClientHello
		}
		payload, ok := er.bytes(int(extLen))
		if !ok {
			return hello, errNotClientHello
		}
		if isGREASE(extType) {
			hello.hasGREASE = true
			continue
		}
		hello.extensions = append(hello.extensions, extType)
		switch extType {
		case extServerName:
			hello.hasSNI = true
		case extALPN:
			hello.alpn = parseALPN(payload)
		case extSignatureAlgorithms:
			hello.sigAlgs = parseUint16List(payload, true)
		case extSupportedVersions:
			hello.versions = parseSupportedVersions(payload)
		}
	}
	return hello, nil
}

func handshakeBody(raw []byte) ([]byte, error) {
	var assembled []byte
	for len(raw) >= tlsRecordHeaderLen {
		if raw[0] != tlsHandshake {
			return nil, errNotClientHello
		}
		n := int(binary.BigEndian.Uint16(raw[3:5]))
		if n < 0 || tlsRecordHeaderLen+n > len(raw) {
			return nil, errNotClientHello
		}
		assembled = append(assembled, raw[tlsRecordHeaderLen:tlsRecordHeaderLen+n]...)
		raw = raw[tlsRecordHeaderLen+n:]
		if len(assembled) >= 4 {
			if assembled[0] != tlsClientHello {
				return nil, errNotClientHello
			}
			need := 4 + int(assembled[1])<<16 + int(assembled[2])<<8 + int(assembled[3])
			if len(assembled) >= need {
				return assembled[4:need], nil
			}
		}
	}
	return nil, errNotClientHello
}

func parseALPN(payload []byte) []string {
	r := byteReader{b: payload}
	listLen, ok := r.u16()
	if !ok {
		return nil
	}
	list, ok := r.bytes(int(listLen))
	if !ok {
		return nil
	}
	lr := byteReader{b: list}
	var protos []string
	for lr.remaining() > 0 {
		n, ok := lr.u8()
		if !ok {
			return protos
		}
		name, ok := lr.bytes(int(n))
		if !ok {
			return protos
		}
		protos = append(protos, string(name))
	}
	return protos
}

func parseUint16List(payload []byte, skipGREASE bool) []uint16 {
	r := byteReader{b: payload}
	listLen, ok := r.u16()
	if !ok || listLen%2 != 0 {
		return nil
	}
	data, ok := r.bytes(int(listLen))
	if !ok {
		return nil
	}
	var values []uint16
	for i := 0; i+1 < len(data); i += 2 {
		value := binary.BigEndian.Uint16(data[i:])
		if skipGREASE && isGREASE(value) {
			continue
		}
		values = append(values, value)
	}
	return values
}

func parseSupportedVersions(payload []byte) []uint16 {
	r := byteReader{b: payload}
	listLen, ok := r.u8()
	if !ok || listLen%2 != 0 {
		return nil
	}
	data, ok := r.bytes(int(listLen))
	if !ok {
		return nil
	}
	var values []uint16
	for i := 0; i+1 < len(data); i += 2 {
		value := binary.BigEndian.Uint16(data[i:])
		if isGREASE(value) {
			continue
		}
		values = append(values, value)
	}
	return values
}

func peekClientHello(conn net.Conn) (raw []byte, err error) {
	var records []byte
	for len(records) < maxClientHelloBytes {
		header := make([]byte, tlsRecordHeaderLen)
		if _, err := io.ReadFull(conn, header); err != nil {
			return records, err
		}
		if header[0] != tlsHandshake {
			return append(records, header...), errNotClientHello
		}
		n := int(binary.BigEndian.Uint16(header[3:5]))
		if n <= 0 || n > maxClientHelloBytes {
			return append(records, header...), errNotClientHello
		}
		payload := make([]byte, n)
		if _, err := io.ReadFull(conn, payload); err != nil {
			return append(append(records, header...), payload...), err
		}
		records = append(records, header...)
		records = append(records, payload...)
		if _, err := handshakeBody(records); err == nil {
			return records, nil
		}
	}
	return records, errNotClientHello
}

type byteReader struct {
	b []byte
}

func (r *byteReader) remaining() int { return len(r.b) }

func (r *byteReader) u8() (byte, bool) {
	if len(r.b) < 1 {
		return 0, false
	}
	v := r.b[0]
	r.b = r.b[1:]
	return v, true
}

func (r *byteReader) u16() (uint16, bool) {
	if len(r.b) < 2 {
		return 0, false
	}
	v := binary.BigEndian.Uint16(r.b)
	r.b = r.b[2:]
	return v, true
}

func (r *byteReader) bytes(n int) ([]byte, bool) {
	if n < 0 || len(r.b) < n {
		return nil, false
	}
	v := r.b[:n]
	r.b = r.b[n:]
	return v, true
}

func (r *byteReader) skip(n int) bool {
	_, ok := r.bytes(n)
	return ok
}
