package auth

import (
	"context"
)

// AccessTokenValidator validates a raw JWT access token string after the Bearer prefix is removed.
type AccessTokenValidator interface {
	// ValidateAccessToken returns ErrUnauthenticated / ErrMisconfigured on failure.
	ValidateAccessToken(ctx context.Context, rawJWT string) (Principal, error)
}
