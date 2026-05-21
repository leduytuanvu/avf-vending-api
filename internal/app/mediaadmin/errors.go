package mediaadmin

import "errors"

var (
	// ErrNotConfigured is returned when neither object storage nor external URL registration is wired.
	ErrNotConfigured         = errors.New("mediaadmin: not configured")
	ErrUploadNotConfigured   = errors.New("mediaadmin: object storage upload not configured")
	ErrExternalNotConfigured = errors.New("mediaadmin: external product image URLs not configured")
	ErrCloudinaryNotConfigured = errors.New("mediaadmin: cloudinary product image upload not configured")
	ErrInvalidArgument       = errors.New("mediaadmin: invalid argument")
	ErrNotFound              = errors.New("mediaadmin: not found")
	ErrConflict              = errors.New("mediaadmin: conflict")
)
