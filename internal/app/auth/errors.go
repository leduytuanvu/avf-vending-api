package auth

import "errors"

var (
	// ErrInvalidCredentials is returned for bad email/password combinations (never leak which failed).
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrInvalidRequest indicates missing or malformed fields.
	ErrInvalidRequest = errors.New("invalid request")
	// ErrInvalidRefreshToken is returned when refresh token is unknown or revoked.
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
)
