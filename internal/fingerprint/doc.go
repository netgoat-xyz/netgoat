// Package fingerprint computes a terminate-only TLS/HTTP stack class.
//
// The v1 key is the opaque tuple {JA4}|{akamai_h2}|{alpn_order}, emitted
// only when this process is the TLS terminator. It identifies a client
// TLS/HTTP stack, not a user: Chrome on a million laptops will collide.
// Do not persist stack_class as identity or join it to accounts.
//
// HTTP/3 / QUIC clients are a documented hole: this agent does not
// terminate QUIC in v1, so those clients never appear in stack_class.
// When QUIC termination exists, add transport parameters (and JA4 q…)
// as a fourth field on the same tuple.
//
// Emit nothing for plaintext HTTP, TLS pass-through, or another
// terminator in front (Cloudflare origin / CF-Connecting-IP). Wrong
// deploy plus a fingerprint is worse than no fingerprint.
package fingerprint
