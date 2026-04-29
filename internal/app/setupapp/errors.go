package setupapp

import "errors"

var (
	// ErrNotFound is returned when the machine or a referenced cabinet row is missing.
	ErrNotFound = errors.New("setup: not found")
	// ErrMachineNotEligibleForBootstrap is returned when the machine cannot serve technician bootstrap (e.g. retired).
	ErrMachineNotEligibleForBootstrap = errors.New("setup: machine not eligible for bootstrap")
	// ErrCabinetNotFound is returned when a layout references a cabinet code that does not exist on the machine.
	ErrCabinetNotFound = errors.New("setup: cabinet not found for machine")
	// ErrSlotLayoutNotFound is returned when SaveDraftOrCurrentSlotConfigs cannot resolve a layout revision.
	ErrSlotLayoutNotFound = errors.New("setup: slot layout not found")
)
