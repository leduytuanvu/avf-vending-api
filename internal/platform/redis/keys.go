package redis

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const defaultKeyPrefix = "avf"

func cleanPrefix(prefix string) string {
	p := strings.Trim(strings.TrimSpace(prefix), ":")
	if p == "" {
		return defaultKeyPrefix
	}
	return sanitizePart(p)
}

func key(prefix string, parts ...string) string {
	all := append([]string{cleanPrefix(prefix), "v1"}, parts...)
	return strings.Join(all, ":")
}

func digest(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:16])
}

func digestBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:16])
}

func sanitizePart(raw string) string {
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" {
		return "empty"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == ':':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-_:")
	if out == "" {
		return "empty"
	}
	return out
}
