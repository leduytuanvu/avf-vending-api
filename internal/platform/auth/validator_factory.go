package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/avf/avf-vending-api/internal/config"
)

// Well-known HTTP_AUTH_MODE values (case-insensitive in config).
const (
	HTTPAuthModeHS256     = "hs256"
	HTTPAuthModeRS256PEM  = "rs256_pem"
	HTTPAuthModeRS256JWKS = "rs256_jwks"
)

// NewAccessTokenValidator builds the validator described by cfg (mode, keys, JWKS URL, iss/aud).
func NewAccessTokenValidator(cfg config.HTTPAuthConfig) (AccessTokenValidator, error) {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	switch mode {
	case "", HTTPAuthModeHS256:
		return newHS256Validator(cfg.JWTSecret, cfg.JWTSecretPrevious, cfg.JWTLeeway), nil
	case HTTPAuthModeRS256PEM:
		if len(cfg.RSAPublicKeyPEM) == 0 {
			return nil, fmt.Errorf("auth: HTTP_AUTH_MODE=rs256_pem requires HTTP_AUTH_JWT_RSA_PUBLIC_KEY or HTTP_AUTH_JWT_RSA_PUBLIC_KEY_FILE")
		}
		return newRS256PEMValidator(cfg.RSAPublicKeyPEM, cfg.ExpectedIssuer, cfg.ExpectedAudience, cfg.JWTLeeway)
	case HTTPAuthModeRS256JWKS:
		if strings.TrimSpace(cfg.JWKSURL) == "" {
			return nil, fmt.Errorf("auth: HTTP_AUTH_MODE=rs256_jwks requires HTTP_AUTH_JWT_JWKS_URL")
		}
		return newRS256JWKSCachedValidator(cfg.JWKSURL, cfg.JWKSCacheTTL, cfg.ExpectedIssuer, cfg.ExpectedAudience, cfg.JWTLeeway), nil
	default:
		return nil, fmt.Errorf("auth: unknown HTTP_AUTH_MODE %q", cfg.Mode)
	}
}

// MaybeWarmJWKS calls WarmJWKS when the validator is JWKS-backed (startup connectivity check).
func MaybeWarmJWKS(ctx context.Context, v AccessTokenValidator) error {
	jwks, ok := v.(*rs256JWKSCachedValidator)
	if !ok {
		return nil
	}
	return jwks.WarmJWKS(ctx)
}
