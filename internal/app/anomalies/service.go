// Package anomalies implements P2.4 operational anomaly detectors and unified admin listing (inventory_anomalies).
package anomalies

import (
	"context"
	"errors"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/inventoryadmin"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultManualAdjustmentAbsThreshold = 50
	defaultAdjustmentLookbackDays       = 365
)

// Service runs detectors and serves unified anomaly + org-scoped restock suggestions.
type Service struct {
	q               *db.Queries
	pool            *pgxpool.Pool
	inventory       *inventoryadmin.Service
	adjustThreshold int32
	adjustLookback  int64
}

// NewService wires anomaly detection; inventory admin is required for restock suggestions.
func NewService(pool *pgxpool.Pool, inv *inventoryadmin.Service) (*Service, error) {
	if pool == nil {
		return nil, errors.New("anomalies: nil pool")
	}
	if inv == nil {
		return nil, errors.New("anomalies: nil inventory admin")
	}
	return &Service{
		q:               db.New(pool),
		pool:            pool,
		inventory:       inv,
		adjustThreshold: defaultManualAdjustmentAbsThreshold,
		adjustLookback:  defaultAdjustmentLookbackDays,
	}, nil
}

// Sync runs legacy inventory detectors plus P2.4 operational detectors (idempotent / deduped).
func (s *Service) Sync(ctx context.Context, organizationID uuid.UUID) error {
	if s == nil || s.q == nil {
		return errors.New("anomalies: nil service")
	}
	if _, err := s.q.AdminOpsInsertDetectedNegativeStockAnomalies(ctx, organizationID); err != nil {
		return err
	}
	if _, err := s.q.AdminOpsInsertDetectedManualAdjustmentAnomalies(ctx, db.AdminOpsInsertDetectedManualAdjustmentAnomaliesParams{
		OrganizationID:         organizationID,
		AdjustmentAbsThreshold: s.adjustThreshold,
		LookbackDays:           s.adjustLookback,
	}); err != nil {
		return err
	}
	if _, err := s.q.AdminOpsInsertStaleInventorySyncAnomalies(ctx, organizationID); err != nil {
		return err
	}
	if _, err := s.q.AnomaliesInsertMachineOfflineTooLong(ctx, organizationID); err != nil {
		return err
	}
	if _, err := s.q.AnomaliesInsertRepeatedVendFailure(ctx, organizationID); err != nil {
		return err
	}
	if _, err := s.q.AnomaliesInsertRepeatedPaymentFailure(ctx, organizationID); err != nil {
		return err
	}
	if _, err := s.q.AnomaliesInsertStockMismatch(ctx, organizationID); err != nil {
		return err
	}
	if _, err := s.q.AnomaliesInsertNegativeStockAttempt(ctx, organizationID); err != nil {
		return err
	}
	if _, err := s.q.AnomaliesInsertHighCashVariance(ctx, organizationID); err != nil {
		return err
	}
	if _, err := s.q.AnomaliesInsertCommandFailureSpike(ctx, organizationID); err != nil {
		return err
	}
	if _, err := s.q.AnomaliesInsertTelemetryMissing(ctx, organizationID); err != nil {
		return err
	}
	if _, err := s.q.AnomaliesInsertLowStockThreshold(ctx, organizationID); err != nil {
		return err
	}
	_, err := s.q.AnomaliesInsertSoldOutSoonEstimate(ctx, organizationID)
	return err
}

// List returns open and historical anomaly rows for the tenant (newest first).
func (s *Service) List(ctx context.Context, organizationID uuid.UUID, machineID *uuid.UUID, limit, offset int32, refreshDetectors bool) ([]db.AdminOpsListInventoryAnomaliesByOrgRow, error) {
	if s == nil || s.q == nil {
		return nil, errors.New("anomalies: nil service")
	}
	if refreshDetectors {
		if err := s.Sync(ctx, organizationID); err != nil {
			return nil, err
		}
	}
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	filterMachine := machineID != nil && *machineID != uuid.Nil
	return s.q.AdminOpsListInventoryAnomaliesByOrg(ctx, db.AdminOpsListInventoryAnomaliesByOrgParams{
		OrganizationID: organizationID,
		FilterMachine:  filterMachine,
		MachineID:      derefUUID(machineID),
		LimitVal:       limit,
		OffsetVal:      offset,
	})
}

// Get returns one anomaly row with machine labels.
func (s *Service) Get(ctx context.Context, organizationID, anomalyID uuid.UUID) (db.AnomaliesGetByOrgAndIDRow, error) {
	if s == nil || s.q == nil {
		return db.AnomaliesGetByOrgAndIDRow{}, errors.New("anomalies: nil service")
	}
	return s.q.AnomaliesGetByOrgAndID(ctx, db.AnomaliesGetByOrgAndIDParams{
		OrganizationID: organizationID,
		AnomalyID:      anomalyID,
	})
}

// Resolve closes an open anomaly (operator acknowledgement).
func (s *Service) Resolve(ctx context.Context, organizationID, anomalyID, actorAccountID uuid.UUID, note string) error {
	if s == nil || s.q == nil {
		return errors.New("anomalies: nil service")
	}
	_, err := s.q.AdminOpsResolveInventoryAnomaly(ctx, db.AdminOpsResolveInventoryAnomalyParams{
		ID:             anomalyID,
		OrganizationID: organizationID,
		ResolvedBy:     uuidToPgUUID(actorAccountID),
		ResolutionNote: pgtype.Text{String: strings.TrimSpace(note), Valid: strings.TrimSpace(note) != ""},
	})
	return MapIgnoreError(err)
}

// Ignore marks an open anomaly as ignored (still audited at the HTTP boundary).
func (s *Service) Ignore(ctx context.Context, organizationID, anomalyID, actorAccountID uuid.UUID, note string) error {
	if s == nil || s.q == nil {
		return errors.New("anomalies: nil service")
	}
	_, err := s.q.AdminOpsIgnoreInventoryAnomaly(ctx, db.AdminOpsIgnoreInventoryAnomalyParams{
		ID:             anomalyID,
		OrganizationID: organizationID,
		ResolvedBy:     uuidToPgUUID(actorAccountID),
		ResolutionNote: pgtype.Text{String: strings.TrimSpace(note), Valid: strings.TrimSpace(note) != ""},
	})
	return MapIgnoreError(err)
}

// RestockSuggestions returns explainable refill rows (current stock, velocity, thresholds) for the organization.
func (s *Service) RestockSuggestions(ctx context.Context, p inventoryadmin.RefillForecastParams) (*inventoryadmin.RefillForecastResponse, error) {
	if s == nil || s.inventory == nil {
		return nil, errors.New("anomalies: nil service")
	}
	return s.inventory.ListRefillForecast(ctx, p)
}

func derefUUID(p *uuid.UUID) uuid.UUID {
	if p == nil {
		return uuid.Nil
	}
	return *p
}

func uuidToPgUUID(u uuid.UUID) pgtype.UUID {
	if u == uuid.Nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{Bytes: u, Valid: true}
}

// ErrNotFoundOpen wraps pgx.ErrNoRows for resolve/ignore.
var ErrNotFoundOpen = errors.New("anomalies: not found or not open")

// MapIgnoreError normalizes pgx.ErrNoRows from ignore/update.
func MapIgnoreError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFoundOpen
	}
	return err
}
