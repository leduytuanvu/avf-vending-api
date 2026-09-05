package postgres

import (
	"context"
	"errors"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
)

// GetLocalLayoutMirror implements commerce.LocalLayoutMirrorReader.
func (s *Store) GetLocalLayoutMirror(ctx context.Context, machineID uuid.UUID) (appcommerce.LocalLayoutMirror, error) {
	if s == nil || s.pool == nil {
		return appcommerce.LocalLayoutMirror{}, errors.New("postgres: nil store")
	}
	row, err := db.New(s.pool).GetMachineLocalLayoutMirror(ctx, machineID)
	if err != nil {
		return appcommerce.LocalLayoutMirror{}, err
	}
	return appcommerce.LocalLayoutMirror{
		Revision:   row.Revision,
		SlotsJSON:  row.Slots,
		ReportedAt: row.ReportedAt,
	}, nil
}
