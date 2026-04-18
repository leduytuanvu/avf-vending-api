package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
)

// verifyHS256JWT returns the payload JSON bytes if the HMAC signature matches.
func verifyHS256JWT(secret []byte, token string) ([]byte, error) {
	if len(secret) == 0 {
		return nil, ErrMisconfigured
	}
	token = strings.TrimSpace(token)
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrUnauthenticated
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrUnauthenticated
	}
	var hdr struct {
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerJSON, &hdr); err != nil || !strings.EqualFold(hdr.Alg, "HS256") {
		return nil, ErrUnauthenticated
	}

	signing := parts[0] + "." + parts[1]
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrUnauthenticated
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(signing))
	expected := mac.Sum(nil)
	if !hmac.Equal(sig, expected) {
		return nil, ErrUnauthenticated
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrUnauthenticated
	}
	return payloadJSON, nil
}
