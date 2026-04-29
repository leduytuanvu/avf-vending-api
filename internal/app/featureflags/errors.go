package featureflags

import "errors"

var (
	ErrNotFound       = errors.New("featureflags: not found")
	ErrInvalidTarget  = errors.New("featureflags: invalid target")
	ErrInvalidRollout = errors.New("featureflags: invalid rollout")
	ErrConflict       = errors.New("featureflags: conflict")
)
