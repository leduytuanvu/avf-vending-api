package rollout

import "errors"

var (
	ErrInvalidArgument = errors.New("rollout: invalid argument")
	ErrNotFound        = errors.New("rollout: not found")
	ErrForbiddenState  = errors.New("rollout: invalid campaign state")
)
