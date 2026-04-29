package provisioning

import "errors"

var (
	ErrInvalidArgument = errors.New("provisioning: invalid argument")
	ErrNotFound        = errors.New("provisioning: not found")
)
