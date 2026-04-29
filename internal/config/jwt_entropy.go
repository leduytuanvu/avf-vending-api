package config

import "bytes"

// jwtHMACSecretBytesAreTriviallyWeak reports whether a secret long enough for HS256 is a single
// repeated byte (e.g. tests-only patterns), which must not pass production gates.
func jwtHMACSecretBytesAreTriviallyWeak(secret []byte) bool {
	s := bytes.TrimSpace(secret)
	if len(s) < 32 {
		return false
	}
	first := s[0]
	for _, b := range s[1:] {
		if b != first {
			return false
		}
	}
	return true
}
