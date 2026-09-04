package challenge

import (
	"crypto/sha256"
	"os"
	"strings"
)

const (
	// ChallengeSecretEnv is the preferred HMAC key source. When unset,
	// DiamondKey is used. When both are unset the process generates an
	// ephemeral key and the verified set is not preserved across restarts.
	ChallengeSecretEnv = "NETGOAT_CHALLENGE_SECRET"
	diamondKeyEnv      = "DiamondKey"
	secretDomain       = "netgoat-challenge-v1"
)

// ResolveSecret returns the HMAC key used to bind PoW commitments.
// ephemeral is true when the key was generated for this process only.
func ResolveSecret() (secret []byte, ephemeral bool) {
	if value := strings.TrimSpace(os.Getenv(ChallengeSecretEnv)); value != "" {
		return deriveSecret(value), false
	}
	if value := strings.TrimSpace(os.Getenv(diamondKeyEnv)); value != "" {
		return deriveSecret(value), false
	}
	secret = make([]byte, sha256.Size)
	readRandom(secret)
	return secret, true
}

func deriveSecret(material string) []byte {
	sum := sha256.Sum256([]byte(secretDomain + ":" + material))
	return sum[:]
}
