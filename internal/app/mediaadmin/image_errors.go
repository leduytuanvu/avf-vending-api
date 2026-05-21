package mediaadmin

import (
	"errors"
	"fmt"
)

// InvalidImageFileError is returned when multipart upload validation fails.
type InvalidImageFileError struct {
	Message      string
	AllowedTypes []string
	MaxSizeMB    int
	TooLarge     bool
}

func (e *InvalidImageFileError) Error() string {
	if e == nil {
		return "invalid image file"
	}
	if e.Message != "" {
		return e.Message
	}
	return "invalid image file"
}

func AsInvalidImageFile(err error) (*InvalidImageFileError, bool) {
	var target *InvalidImageFileError
	if errors.As(err, &target) {
		return target, true
	}
	return nil, false
}

func asInvalidImageFile(err error) (*InvalidImageFileError, bool) {
	return AsInvalidImageFile(err)
}

func invalidImageFile(msg string, allowed []string, maxMB int) error {
	return &InvalidImageFileError{
		Message:      msg,
		AllowedTypes: allowed,
		MaxSizeMB:    maxMB,
	}
}

func fileTooLarge(maxMB int, allowed []string) error {
	return &InvalidImageFileError{
		Message:      fmt.Sprintf("file exceeds maximum size of %d MB", maxMB),
		AllowedTypes: allowed,
		MaxSizeMB:    maxMB,
		TooLarge:     true,
	}
}
