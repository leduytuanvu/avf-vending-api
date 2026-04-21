package assortmentapp

import (
	"context"

	"github.com/google/uuid"
)

// BindMachineAssortmentInput binds the primary assortment for a machine (closes any prior primary binding).
type BindMachineAssortmentInput struct {
	MachineID         uuid.UUID
	AssortmentID      uuid.UUID
	OperatorSessionID *uuid.UUID
	CorrelationID     *uuid.UUID
}

// Repository persists assortment bindings for machines.
type Repository interface {
	BindMachineAssortment(ctx context.Context, in BindMachineAssortmentInput) error
}
