package commerce

import "errors"

var (
	ErrInvalidArgument   = errors.New("commerce: invalid argument")
	ErrNotConfigured     = errors.New("commerce: dependency not configured")
	ErrNotFound          = errors.New("commerce: not found")
	ErrOrgMismatch       = errors.New("commerce: organization mismatch")
	ErrIllegalTransition = errors.New("commerce: illegal state transition")
	ErrPaymentNotSettled = errors.New("commerce: payment not in a settled captured state for this operation")
)
