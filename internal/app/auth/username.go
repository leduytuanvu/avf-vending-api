package auth

import (
	"regexp"
	"strings"
	"unicode"
)

const (
	usernameMinLen = 3
	usernameMaxLen = 32
)

var usernameFormatRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{1,30}[A-Za-z0-9]$`)

var reservedUsernames = map[string]struct{}{
	"root": {}, "system": {}, "service": {}, "machine": {}, "technician": {},
	"platform": {}, "avf": {}, "me": {}, "null": {}, "undefined": {}, "anonymous": {},
}

func normalizeUsername(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "", ErrInvalidUsername
	}
	if strings.ContainsFunc(v, unicode.IsSpace) {
		return "", ErrInvalidUsername
	}
	if len(v) < usernameMinLen || len(v) > usernameMaxLen {
		return "", ErrInvalidUsername
	}
	if !usernameFormatRE.MatchString(v) {
		return "", ErrInvalidUsername
	}
	if strings.HasPrefix(v, ".") || strings.HasPrefix(v, "_") || strings.HasPrefix(v, "-") ||
		strings.HasSuffix(v, ".") || strings.HasSuffix(v, "_") || strings.HasSuffix(v, "-") {
		return "", ErrInvalidUsername
	}
	lower := strings.ToLower(v)
	if _, reserved := reservedUsernames[lower]; reserved {
		return "", ErrInvalidUsername
	}
	return lower, nil
}

// NormalizeUsernameForBootstrap exposes username validation for cmd/bootstrap-admin.
func NormalizeUsernameForBootstrap(raw string) (string, error) {
	return normalizeUsername(raw)
}
