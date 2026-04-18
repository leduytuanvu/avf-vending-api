package auth

import "errors"

var (
	// ErrUnauthenticated indicates missing or invalid credentials.
	ErrUnauthenticated = errors.New("auth: unauthenticated")
	// ErrForbidden indicates authenticated principal cannot perform the action.
	ErrForbidden = errors.New("auth: forbidden")
	// ErrMisconfigured indicates the server cannot validate tokens (e.g. empty signing secret).
	ErrMisconfigured = errors.New("auth: misconfigured")
)
