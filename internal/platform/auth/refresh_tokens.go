package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
)

const refreshTokenEntropyBytes = 32

// NewRefreshToken returns a high-entropy opaque refresh token suitable for clients.
// The returned slice is raw random bytes before base64url encoding for wire format.
func NewRefreshToken() (rawForClient string, hashSHA256 []byte, err error) {
	var b [refreshTokenEntropyBytes]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", nil, fmt.Errorf("auth: refresh entropy: %w", err)
	}
	raw := base64.RawURLEncoding.EncodeToString(b[:])
	sum := sha256.Sum256([]byte(raw))
	return raw, sum[:], nil
}

// HashRefreshToken returns SHA-256 of the client refresh token string (for persistence / lookup).
func HashRefreshToken(clientToken string) []byte {
	sum := sha256.Sum256([]byte(strings.TrimSpace(clientToken)))
	return sum[:]
}
