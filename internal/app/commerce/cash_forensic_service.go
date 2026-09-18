package commerce

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ReportCashMovements ingests hardware cash movement evidence from the machine runtime.
func (s *Service) ReportCashMovements(ctx context.Context, in RecordCashMovementsInput) (RecordCashMovementsResult, error) {
	if s.forensic == nil {
		return RecordCashMovementsResult{}, ErrNotConfigured
	}
	if in.MachineID == uuid.Nil {
		return RecordCashMovementsResult{}, errors.Join(ErrInvalidArgument, errors.New("machine_id required"))
	}
	if strings.TrimSpace(in.IdempotencyKey) == "" {
		return RecordCashMovementsResult{}, errors.Join(ErrInvalidArgument, errors.New("idempotency_key required"))
	}
	return s.forensic.RecordCashMovements(ctx, in)
}

// GetMachinePhysicalCashPosition returns the four-balance machine cash read model.
func (s *Service) GetMachinePhysicalCashPosition(ctx context.Context, machineID uuid.UUID, currency string, asOf *time.Time) (MachinePhysicalCashPosition, error) {
	if s.forensic == nil {
		return MachinePhysicalCashPosition{}, ErrNotConfigured
	}
	return s.forensic.GetMachinePhysicalCashPosition(ctx, machineID, currency, asOf)
}

// ListCashLedger returns paginated forensic ledger rows for a machine.
func (s *Service) ListCashLedger(ctx context.Context, in ListCashLedgerInput) (ListCashLedgerResult, error) {
	if s.forensic == nil {
		return ListCashLedgerResult{}, ErrNotConfigured
	}
	return s.forensic.ListCashLedger(ctx, in)
}

// GetCashLedgerEventDetail returns one ledger event with metadata.
func (s *Service) GetCashLedgerEventDetail(ctx context.Context, eventID uuid.UUID, movementClass string) (CashLedgerEventDetail, error) {
	if s.forensic == nil {
		return CashLedgerEventDetail{}, ErrNotConfigured
	}
	return s.forensic.GetCashLedgerEventDetail(ctx, eventID, movementClass)
}

// GetOrderCashForensics returns order-level cash forensic detail.
func (s *Service) GetOrderCashForensics(ctx context.Context, orderID uuid.UUID) (OrderCashForensicsView, error) {
	if s.forensic == nil {
		return OrderCashForensicsView{}, ErrNotConfigured
	}
	return s.forensic.GetOrderCashForensics(ctx, orderID)
}

// InsertCashAdjustment records an additive operator adjustment.
func (s *Service) InsertCashAdjustment(ctx context.Context, in InsertCashAdjustmentInput) (CashAdjustmentView, error) {
	if s.forensic == nil {
		return CashAdjustmentView{}, ErrNotConfigured
	}
	return s.forensic.InsertCashAdjustment(ctx, in)
}
