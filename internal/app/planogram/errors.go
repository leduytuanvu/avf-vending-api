package planogram

import "errors"

var (
	ErrNotFound        = errors.New("planogram: not found")
	ErrInvalidSnapshot = errors.New("planogram: invalid snapshot")
	ErrValidation      = errors.New("planogram: validation failed")
)
