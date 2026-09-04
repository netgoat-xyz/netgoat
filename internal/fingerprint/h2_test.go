package fingerprint

import (
	"encoding/binary"
	"testing"

	"golang.org/x/net/http2/hpack"
)

func TestH2SettingsOrderPreserved(t *testing.T) {
	var rec h2Fingerprint
	payload := settingsPayload([][2]uint32{{3, 100}, {1, 65536}, {2, 0}})
	rec.recordSettings(payload, 0)
	rec.prefaceSeen = true
	got := rec.field()
	settings, _, _ := split3(got)
	if settings != "3:100;1:65536;2:0" {
		t.Fatalf("SETTINGS = %q, want transmission order 3:100;1:65536;2:0", settings)
	}
	if settings == "1:65536;2:0;3:100" {
		t.Fatal("sorting SETTINGS is a bug")
	}
}

func TestH2WindowAndPriorityAndPseudo(t *testing.T) {
	var rec h2Fingerprint
	rec.prefaceSeen = true
	rec.recordSettings(settingsPayload([][2]uint32{{1, 65536}, {2, 0}}), 0)
	rec.recordWindowUpdate(0, u32(15663105))
	rec.recordWindowUpdate(0, u32(1)) // first stream-0 increment wins
	rec.recordPriority(3, priorityPayload(true, 0, 200))
	block := encodeHeaders([][2]string{
		{":method", "GET"},
		{":authority", "app.example.test"},
		{":scheme", "https"},
		{":path", "/"},
	})
	rec.recordHeaders(block, flagEndHeaders)
	got := rec.field()
	if got != "1:65536;2:0|15663105|3:1:0:201|m,a,s,p" {
		t.Fatalf("h2 field = %q", got)
	}
}

func TestH2AckSettingsIgnored(t *testing.T) {
	var rec h2Fingerprint
	rec.prefaceSeen = true
	rec.recordSettings(settingsPayload([][2]uint32{{1, 1}}), flagAck)
	if rec.field() != "|0|0|0" && rec.field() != "0|0|0|0" {
		// empty settings, zeros for the rest
		if rec.field() != "|0|0|0" {
			t.Fatalf("ACK SETTINGS should not record pairs: %q", rec.field())
		}
	}
}

func settingsPayload(pairs [][2]uint32) []byte {
	out := make([]byte, 0, 6*len(pairs))
	for _, pair := range pairs {
		out = appendU16(out, uint16(pair[0]))
		out = binary.BigEndian.AppendUint32(out, pair[1])
	}
	return out
}

func u32(v uint32) []byte {
	return binary.BigEndian.AppendUint32(nil, v)
}

func priorityPayload(exclusive bool, dependsOn uint32, weightMinusOne byte) []byte {
	dep := dependsOn
	if exclusive {
		dep |= 0x80000000
	}
	out := u32(dep)
	return append(out, weightMinusOne)
}

func encodeHeaders(fields [][2]string) []byte {
	var buf []byte
	enc := hpack.NewEncoder((*sliceBuffer)(&buf))
	for _, field := range fields {
		if err := enc.WriteField(hpack.HeaderField{Name: field[0], Value: field[1]}); err != nil {
			panic(err)
		}
	}
	return buf
}

type sliceBuffer []byte

func (s *sliceBuffer) Write(p []byte) (int, error) {
	*s = append(*s, p...)
	return len(p), nil
}

func split3(field string) (string, string, string) {
	parts := splitClass(field)
	for len(parts) < 3 {
		parts = append(parts, "")
	}
	return parts[0], parts[1], parts[2]
}
