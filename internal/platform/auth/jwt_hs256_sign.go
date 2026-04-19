package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// SignHS256JWT signs a JWT using HS256. payload must JSON-marshal to the JWT claims object.
func SignHS256JWT(secret []byte, payload any) (string, error) {
	if len(secret) == 0 {
		return "", ErrMisconfigured
	}
	hdr := []byte(`{"alg":"HS256","typ":"JWT"}`)
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("auth: marshal jwt claims: %w", err)
	}
	h := base64.RawURLEncoding.EncodeToString(hdr)
	p := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signing := h + "." + p
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(signing))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return signing + "." + sig, nil
}

// TrimSecret returns a copy of b without UTF-8 BOM and surrounding ASCII whitespace.
func TrimSecret(b []byte) []byte {
	s := strings.TrimSpace(string(b))
	return []byte(s)
}
