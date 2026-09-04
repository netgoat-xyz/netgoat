package challenge

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"hash"
	"math/bits"
	"strconv"
	"strings"
)

func computeCommitment(secret []byte, sessionID, nonce string, difficulty int, expiresUnix int64) []byte {
	mac := hmac.New(sha256.New, secret)
	writeLenPrefixed(mac, sessionID)
	writeLenPrefixed(mac, nonce)
	var meta [12]byte
	binary.BigEndian.PutUint32(meta[0:4], uint32(difficulty))
	binary.BigEndian.PutUint64(meta[4:12], uint64(expiresUnix))
	_, _ = mac.Write(meta[:])
	return mac.Sum(nil)
}

func writeLenPrefixed(mac hash.Hash, value string) {
	var n [4]byte
	binary.BigEndian.PutUint32(n[:], uint32(len(value)))
	_, _ = mac.Write(n[:])
	_, _ = mac.Write([]byte(value))
}

func powDigest(commitment []byte, counter uint64) []byte {
	var tail [8]byte
	binary.BigEndian.PutUint64(tail[:], counter)
	sum := sha256.Sum256(append(append([]byte{}, commitment...), tail[:]...))
	return sum[:]
}

func leadingZeroBits(sum []byte) int {
	bitsCount := 0
	for _, b := range sum {
		if b == 0 {
			bitsCount += 8
			continue
		}
		bitsCount += bits.LeadingZeros8(b)
		return bitsCount
	}
	return bitsCount
}

func parseCounter(answer string) (uint64, bool) {
	answer = strings.TrimSpace(answer)
	if answer == "" || len(answer) > 20 {
		return 0, false
	}
	for _, c := range answer {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	value, err := strconv.ParseUint(answer, 10, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

func encodeMAC(commitment []byte) string {
	return hex.EncodeToString(commitment)
}

func decodeMAC(value string) ([]byte, bool) {
	value = strings.TrimSpace(value)
	if len(value) != sha256.Size*2 {
		return nil, false
	}
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != sha256.Size {
		return nil, false
	}
	return decoded, true
}
