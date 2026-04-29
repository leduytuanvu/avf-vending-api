package auth

import (
	"strings"
	"unicode"

	"github.com/avf/avf-vending-api/internal/config"
)

var commonLocalDefaultPasswords = map[string]struct{}{
	"password": {}, "password123": {}, "password12345": {}, "admin123": {}, "qwerty123": {},
	"changeme": {}, "welcome123": {}, "12345678": {}, "87654321": {},
}

func (s *Service) validatePassword(pw string) error {
	if s == nil {
		return ErrWeakPassword
	}
	return validatePasswordAgainstPolicy(pw, s.adminSec)
}

func validatePasswordAgainstPolicy(pw string, pol config.AdminAuthSecurityConfig) error {
	pw = strings.TrimSpace(pw)
	blocked := pol.PasswordRejectCommonList && looksLikeCommonLocalDefault(pw)
	if blocked {
		return ErrWeakPassword
	}
	minLen := pol.PasswordMinLength
	if minLen < 8 {
		minLen = 10
	}
	if pw == "" || len(pw) < minLen {
		return ErrWeakPassword
	}
	var hasDigit, hasUpper, hasLower, hasSymbol bool
	for _, r := range pw {
		switch {
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSymbol = true
		}
	}
	if pol.PasswordRequireDigit && !hasDigit {
		return ErrWeakPassword
	}
	if pol.PasswordRequireUpper && !hasUpper {
		return ErrWeakPassword
	}
	if pol.PasswordRequireLower && !hasLower {
		return ErrWeakPassword
	}
	if pol.PasswordRequireSymbol && !hasSymbol {
		return ErrWeakPassword
	}
	return nil
}

func looksLikeCommonLocalDefault(pw string) bool {
	k := strings.ToLower(strings.TrimSpace(pw))
	if _, ok := commonLocalDefaultPasswords[k]; ok {
		return true
	}
	// Exact-match subset only (avoid rejecting benign phrases containing substrings).
	return false
}
