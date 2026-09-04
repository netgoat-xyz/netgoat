package fingerprint

import (
	"encoding/binary"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/net/http2/hpack"
)

const (
	h2Preface = "PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n"

	frameHeaders      = 0x1
	framePriority     = 0x2
	frameSettings     = 0x4
	frameWindowUpdate = 0x8
	frameContinuation = 0x9

	flagAck         = 0x1
	flagEndHeaders  = 0x4
	flagPadded      = 0x8
	flagPriorityHdr = 0x20
)

// h2Fingerprint records the Akamai-style H2 tuple in transmission order.
type h2Fingerprint struct {
	mu sync.Mutex

	settings    []string
	window      string
	priorities  []string
	pseudo      string
	prefaceSeen bool
	done        bool

	headerBuf []byte
	cont      bool
}

func (h *h2Fingerprint) finished() bool {
	if h == nil {
		return true
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.done
}

func (h *h2Fingerprint) field() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.prefaceSeen && len(h.settings) == 0 && h.pseudo == "" {
		return "-"
	}
	settings := strings.Join(h.settings, ";")
	window := h.window
	if window == "" {
		window = "0"
	}
	priority := "0"
	if len(h.priorities) > 0 {
		priority = strings.Join(h.priorities, ",")
	}
	pseudo := h.pseudo
	if pseudo == "" {
		pseudo = "0"
	}
	return settings + "|" + window + "|" + priority + "|" + pseudo
}

func (h *h2Fingerprint) recordSettings(payload []byte, flags byte) {
	if flags&flagAck != 0 {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := 0; i+6 <= len(payload); i += 6 {
		id := binary.BigEndian.Uint16(payload[i:])
		value := binary.BigEndian.Uint32(payload[i+2:])
		h.settings = append(h.settings, strconv.FormatUint(uint64(id), 10)+":"+strconv.FormatUint(uint64(value), 10))
	}
}

func (h *h2Fingerprint) recordWindowUpdate(streamID uint32, payload []byte) {
	if streamID != 0 || len(payload) < 4 || h.window != "" {
		return
	}
	increment := binary.BigEndian.Uint32(payload) & 0x7fffffff
	h.mu.Lock()
	h.window = strconv.FormatUint(uint64(increment), 10)
	h.mu.Unlock()
}

func (h *h2Fingerprint) recordPriority(streamID uint32, payload []byte) {
	if len(payload) < 5 {
		return
	}
	dep := binary.BigEndian.Uint32(payload)
	exclusive := byte(0)
	if dep&0x80000000 != 0 {
		exclusive = 1
	}
	dependsOn := dep & 0x7fffffff
	weight := uint16(payload[4]) + 1
	h.mu.Lock()
	h.priorities = append(h.priorities, strconv.FormatUint(uint64(streamID), 10)+":"+
		strconv.FormatUint(uint64(exclusive), 10)+":"+
		strconv.FormatUint(uint64(dependsOn), 10)+":"+
		strconv.FormatUint(uint64(weight), 10))
	h.mu.Unlock()
}

func (h *h2Fingerprint) recordHeaders(payload []byte, flags byte) {
	block, ok := headerBlock(payload, flags)
	if !ok {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.headerBuf = append(h.headerBuf, block...)
	h.cont = flags&flagEndHeaders == 0
	if flags&flagEndHeaders == 0 {
		return
	}
	h.pseudo = decodePseudoOrder(h.headerBuf)
	h.done = true
	h.headerBuf = nil
}

func (h *h2Fingerprint) recordContinuation(payload []byte, flags byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.cont {
		return
	}
	h.headerBuf = append(h.headerBuf, payload...)
	if flags&flagEndHeaders == 0 {
		return
	}
	h.pseudo = decodePseudoOrder(h.headerBuf)
	h.done = true
	h.cont = false
	h.headerBuf = nil
}

func headerBlock(payload []byte, flags byte) ([]byte, bool) {
	if flags&flagPadded != 0 {
		if len(payload) < 1 {
			return nil, false
		}
		pad := int(payload[0])
		payload = payload[1:]
		if pad > len(payload) {
			return nil, false
		}
		payload = payload[:len(payload)-pad]
	}
	if flags&flagPriorityHdr != 0 {
		if len(payload) < 5 {
			return nil, false
		}
		payload = payload[5:]
	}
	return payload, true
}

func decodePseudoOrder(block []byte) string {
	decoder := hpack.NewDecoder(4096, nil)
	fields, err := decoder.DecodeFull(block)
	if err != nil {
		return "0"
	}
	var codes []string
	for _, field := range fields {
		switch field.Name {
		case ":method":
			codes = append(codes, "m")
		case ":authority":
			codes = append(codes, "a")
		case ":scheme":
			codes = append(codes, "s")
		case ":path":
			codes = append(codes, "p")
		}
	}
	if len(codes) == 0 {
		return "0"
	}
	return strings.Join(codes, ",")
}
