package auth

import "errors"

var (
	// ErrInvalidCredentials is returned for bad email/password combinations (never leak which failed).
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrInvalidRequest indicates missing or malformed fields.
	ErrInvalidRequest = errors.New("invalid request")
	// ErrInvalidRefreshToken is returned when refresh token is unknown or revoked.
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
	ErrInvalidResetToken   = errors.New("invalid reset token")

	ErrInvalidEmail           = errors.New("invalid email address")
	ErrWeakPassword           = errors.New("password must be at least 10 characters")
	ErrInvalidRole            = errors.New("invalid role")
	ErrConflictDuplicateEmail = errors.New("email already registered for this organization")
	ErrAccountNotFound        = errors.New("account not found")
	ErrForbiddenLastOrgAdmin  = errors.New("cannot remove or deactivate the last organization administrator")

	ErrMFANotConfigured = errors.New("MFA encryption is not configured")
	ErrMFAConflict      = errors.New("MFA enrollment conflict")
)
