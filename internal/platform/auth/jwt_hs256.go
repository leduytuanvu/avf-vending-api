package auth

import (
	"time"
)

// ValidateHS256AccessToken validates an HS256 JWT and maps claims into Principal.
// For production RS256/JWKS, use config-driven NewAccessTokenValidator instead.
func ValidateHS256AccessToken(secret []byte, token string, leeway time.Duration) (Principal, error) {
	payload, err := verifyHS256JWT(secret, token)
	if err != nil {
		if err == ErrMisconfigured {
			return Principal{}, err
		}
		return Principal{}, ErrUnauthenticated
	}
	return PrincipalFromJWTPayloadJSON(payload, leeway)
}
