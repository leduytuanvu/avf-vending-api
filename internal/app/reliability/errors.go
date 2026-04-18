package reliability

import "errors"

var (
	ErrInvalidArgument = errors.New("reliability: invalid argument")
	ErrNotConfigured   = errors.New("reliability: dependency not configured")
)
