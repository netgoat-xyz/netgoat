package fingerprint

// isGREASE reports RFC 8701 grease values (0x?A?A). They are stripped
// before JA4 counts and hashes so extension randomization does not mint
// a new stack class.
func isGREASE(value uint16) bool {
	return value&0x0f0f == 0x0a0a
}
