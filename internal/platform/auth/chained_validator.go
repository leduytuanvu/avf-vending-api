package auth

import (
	"context"
	"time"
)

// ChainAccessTokenValidators tries primary first; on ErrUnauthenticated it tries secondary (e.g. HS256
// login-issued tokens when the primary validator is RS256/JWKS).
func ChainAccessTokenValidators(primary, secondary AccessTokenValidator) AccessTokenValidator {
	if secondary == nil {
		return primary
	}
	return chainedValidator{primary: primary, secondary: secondary}
}

type chainedValidator struct {
	primary   AccessTokenValidator
	secondary AccessTokenValidator
}

func (c chainedValidator) ValidateAccessToken(ctx context.Context, raw string) (Principal, error) {
	p, err := c.primary.ValidateAccessToken(ctx, raw)
	if err == nil {
		return p, nil
	}
	if err != ErrUnauthenticated {
		return Principal{}, err
	}
	return c.secondary.ValidateAccessToken(ctx, raw)
}

// NewHS256AccessTokenValidator returns a validator that accepts HS256 JWTs signed with secret (login tokens).
func NewHS256AccessTokenValidator(secret []byte, leeway time.Duration) AccessTokenValidator {
	return newHS256Validator(secret, nil, leeway)
}
