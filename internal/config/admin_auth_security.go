package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// AdminAuthSecurityConfig configures interactive admin MFA, login lockout, password policy,
// and reset/session TTL aliases used by internal/app/auth.
type AdminAuthSecurityConfig struct {
	MFARequiredInProduction bool

	LoginMaxFailedAttempts int32
	LoginLockoutTTL        time.Duration

	PasswordMinLength        int
	PasswordRequireDigit     bool
	PasswordRequireUpper     bool
	PasswordRequireLower     bool
	PasswordRequireSymbol    bool
	PasswordRejectCommonList bool

	PasswordResetTTL time.Duration

	MFAEncryptionKey []byte // AES-256 key material (32 bytes); never logged or returned.
}

func loadAdminAuthSecurityConfig(appEnv AppEnvironment) (AdminAuthSecurityConfig, error) {
	out := AdminAuthSecurityConfig{
		MFARequiredInProduction: getenvBool("ADMIN_MFA_REQUIRED_IN_PRODUCTION", false),

		LoginMaxFailedAttempts: int32(getenvInt("ADMIN_LOGIN_MAX_FAILED_ATTEMPTS", 5)),
		LoginLockoutTTL:        mustParseDuration("ADMIN_LOGIN_LOCKOUT_TTL", getenv("ADMIN_LOGIN_LOCKOUT_TTL", "15m")),

		PasswordMinLength:        getenvInt("PASSWORD_MIN_LENGTH", 10),
		PasswordRequireDigit:     getenvBool("PASSWORD_REQUIRE_DIGIT", false),
		PasswordRequireUpper:     getenvBool("PASSWORD_REQUIRE_UPPER", false),
		PasswordRequireLower:     getenvBool("PASSWORD_REQUIRE_LOWER", false),
		PasswordRequireSymbol:    getenvBool("PASSWORD_REQUIRE_SYMBOL", false),
		PasswordRejectCommonList: getenvBool("PASSWORD_REJECT_COMMON_DEFAULTS", true),

		PasswordResetTTL: mustParseDuration("PASSWORD_RESET_TTL", getenv("PASSWORD_RESET_TTL", "15m")),
	}

	keyStr := strings.TrimSpace(os.Getenv("ADMIN_MFA_ENCRYPTION_KEY"))
	if keyStr != "" {
		raw, err := base64.StdEncoding.DecodeString(keyStr)
		if err != nil {
			return AdminAuthSecurityConfig{}, fmt.Errorf("config: ADMIN_MFA_ENCRYPTION_KEY must be standard base64: %w", err)
		}
		out.MFAEncryptionKey = raw
	}

	if err := out.validate(appEnv); err != nil {
		return AdminAuthSecurityConfig{}, err
	}

	return out, nil
}

func (c AdminAuthSecurityConfig) validate(appEnv AppEnvironment) error {
	if c.LoginMaxFailedAttempts < 1 {
		return errors.New("config: ADMIN_LOGIN_MAX_FAILED_ATTEMPTS must be >= 1")
	}
	if c.LoginLockoutTTL <= 0 {
		return errors.New("config: ADMIN_LOGIN_LOCKOUT_TTL must be > 0")
	}
	if c.PasswordMinLength < 8 {
		return errors.New("config: PASSWORD_MIN_LENGTH must be >= 8")
	}
	if c.PasswordResetTTL <= 0 {
		return errors.New("config: PASSWORD_RESET_TTL must be > 0")
	}
	if c.MFARequiredInProduction && appEnv == AppEnvProduction {
		if len(c.MFAEncryptionKey) != 32 {
			return errors.New("config: ADMIN_MFA_REQUIRED_IN_PRODUCTION requires a 32-byte ADMIN_MFA_ENCRYPTION_KEY (base64-encoded)")
		}
	}
	return nil
}
