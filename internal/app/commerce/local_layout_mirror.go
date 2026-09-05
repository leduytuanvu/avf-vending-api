package commerce

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// LocalLayoutMirror is the machine-reported layout mirror used for pricing provenance.
type LocalLayoutMirror struct {
	Revision   int32
	SlotsJSON  []byte
	ReportedAt time.Time
}

// LocalLayoutMirrorReader loads machine-local layout mirrors.
type LocalLayoutMirrorReader interface {
	GetLocalLayoutMirror(ctx context.Context, machineID uuid.UUID) (LocalLayoutMirror, error)
}
