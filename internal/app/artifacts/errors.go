package artifacts

import "errors"

var (
	// ErrInvalidArgument indicates a bad client input (size, checksum, content type, etc.).
	ErrInvalidArgument = errors.New("artifacts: invalid argument")
	// ErrNotFound indicates the artifact object does not exist in object storage.
	ErrNotFound = errors.New("artifacts: not found")
	// ErrChecksumMismatch indicates streamed bytes did not match the declared SHA-256.
	ErrChecksumMismatch = errors.New("artifacts: sha256 mismatch")
	// ErrTrailingBytes indicates extra bytes after a declared Content-Length payload.
	ErrTrailingBytes = errors.New("artifacts: trailing bytes after payload")
)
