package layoutassignment

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrInvalidDimensions           = errors.New("layout dimensions out of range")
	ErrExceedsHardwareLaneCapacity = errors.New("layout exceeds hardware lane capacity")
	ErrRevisionConflict            = errors.New("expected revision conflict")
	ErrLayoutRevisionConflict      = errors.New("layout revision fingerprint conflict")
	ErrLayoutAssignmentNotFound    = errors.New("layout assignment not found for source")
	ErrLayoutVersionNotFound       = errors.New("layout version not found")
	ErrMachineMismatch             = errors.New("layout version machine mismatch")
	ErrUnknownDimensions           = errors.New("layout dimensions unknown")
)

// ValidateGridDimensions checks contract-wide row/column bounds.
func ValidateGridDimensions(rows, cols int32) error {
	if rows < 1 || rows > MaxGridRows {
		return fmt.Errorf("%w: rows=%d", ErrInvalidDimensions, rows)
	}
	if cols < 1 || cols > MaxGridCols {
		return fmt.Errorf("%w: columns=%d", ErrInvalidDimensions, cols)
	}
	return nil
}

// ValidateMachineTypeLaneBound rejects TCN layouts that exceed 80 addressable lanes.
func ValidateMachineTypeLaneBound(machineType string, rows, cols int32) error {
	t := strings.ToLower(strings.TrimSpace(machineType))
	if t != "tcn" && t != "tcn_hybrid" {
		return nil
	}
	if int(rows)*int(cols) > MaxTCNLanes {
		return fmt.Errorf("%w: rows*columns=%d > %d for machine_type=%s",
			ErrExceedsHardwareLaneCapacity, int(rows)*int(cols), MaxTCNLanes, t)
	}
	return nil
}

// DefaultCreationDimensions returns the fleet-default 6x10 grid for new layouts only.
func DefaultCreationDimensions() (rows, cols int32) {
	return DefaultGridRows, DefaultGridCols
}
