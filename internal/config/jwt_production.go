package config

import (
	"bytes"
	"fmt"
	"strings"
)

// jwtSecretIsDocumentationPlaceholder rejects well-known example secrets from .env.example / docs
// when APP_ENV is staging or production.
func jwtSecretIsDocumentationPlaceholder(secret []byte) bool {
	s := strings.TrimSpace(strings.ToLower(string(bytes.TrimSpace(secret))))
	if len(s) < 16 {
		return false
	}
	switch s {
	case "dev-change-me-use-long-random-string":
		return true
	case "change_me_long_random_secret":
		return true
	case "your-256-bit-secret", "your-secret-key", "super_secret", "secret", "jwt_secret":
		return true
	default:
		return false
	}
}

func jwtDocumentationPlaceholderError(field string) error {
	return fmt.Errorf("config: %s must not use a documentation/local example value; set a unique secret", field)
}
