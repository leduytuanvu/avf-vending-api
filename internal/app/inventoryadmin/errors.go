package inventoryadmin

import "errors"

var (
	// ErrMachineNotFound is returned when the machine id does not exist.
	ErrMachineNotFound = errors.New("machine not found")
	// ErrForbidden indicates the principal cannot read this machine.
	ErrForbidden = errors.New("forbidden")
)
