package auth

import (
	"context"
	"strings"

	"github.com/avf/avf-vending-api/internal/platform/auth/revocation"
)

// RevocationValidator wraps AccessTokenValidator with Redis-backed JTI/subject revocation checks.
// When failOpen is false (production default), Redis errors fail closed (ErrUnauthenticated).
type RevocationValidator struct {
	inner    AccessTokenValidator
	store    revocation.Store
	failOpen bool
}

// WrapWithRevocation returns inner when store is nil or a *revocation.NoopStore.
func WrapWithRevocation(inner AccessTokenValidator, store revocation.Store, failOpen bool) AccessTokenValidator {
	if inner == nil {
		return nil
	}
	if store == nil {
		return inner
	}
	if _, ok := store.(*revocation.NoopStore); ok {
		return inner
	}
	return &RevocationValidator{inner: inner, store: store, failOpen: failOpen}
}

// ValidateAccessToken implements AccessTokenValidator.
// Inner returns the wrapped validator (for JWKS warm-up through optional wrappers).
func (v *RevocationValidator) Inner() AccessTokenValidator {
	if v == nil {
		return nil
	}
	return v.inner
}

func (v *RevocationValidator) ValidateAccessToken(ctx context.Context, rawJWT string) (Principal, error) {
	p, err := v.inner.ValidateAccessToken(ctx, rawJWT)
	if err != nil {
		return Principal{}, err
	}
	if v.store == nil {
		return p, nil
	}
	sub := strings.TrimSpace(p.Subject)
	if sub != "" {
		revoked, err := v.store.IsSubjectRevoked(ctx, sub)
		if err != nil {
			if v.failOpen {
				return p, nil
			}
			return Principal{}, ErrUnauthenticated
		}
		if revoked {
			return Principal{}, ErrUnauthenticated
		}
	}
	if jti := strings.TrimSpace(p.JTI); jti != "" {
		revoked, err := v.store.IsJTIRevoked(ctx, jti)
		if err != nil {
			if v.failOpen {
				return p, nil
			}
			return Principal{}, ErrUnauthenticated
		}
		if revoked {
			return Principal{}, ErrUnauthenticated
		}
	}
	return p, nil
}
