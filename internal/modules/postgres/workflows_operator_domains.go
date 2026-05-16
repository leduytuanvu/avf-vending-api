package postgres

import (
	"context"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// CreateCashCollectionWithAttribution inserts cash_collections and, when operator_session_id is set,
// machine_action_attributions in one transaction.
type CreateCashCollectionWithAttributionInput struct {
	MachineID         uuid.UUID
	CollectedAt       time.Time
	AmountMinor       int64
	Currency          string
	Metadata          []byte
	OperatorSessionID *uuid.UUID
	CorrelationID     *uuid.UUID
}

func (s *Store) CreateCashCollectionWithAttribution(ctx context.Context, in CreateCashCollectionWithAttributionInput) (db.CashCollection, error) {
	meta := in.Metadata
	if len(meta) == 0 {
		meta = []byte("{}")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return db.CashCollection{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)

	_, err = q.GetMachineByIDForUpdate(ctx, in.MachineID)
	if err != nil {
		return db.CashCollection{}, err
	}

	row, err := q.InsertCashCollection(ctx, db.InsertCashCollectionParams{
		MachineID:           in.MachineID,
		CollectedAt:         in.CollectedAt,
		OpenedAt:            in.CollectedAt,
		ClosedAt:            pgtype.Timestamptz{Time: in.CollectedAt, Valid: true},
		LifecycleStatus:     "closed",
		AmountMinor:         in.AmountMinor,
		ExpectedAmountMinor: in.AmountMinor,
		VarianceAmountMinor: 0,
		RequiresReview:      false,
		CloseRequestHash:    nil,
		Currency:            in.Currency,
		Metadata:            meta,
		OperatorSessionID:   optionalUUIDToPg(in.OperatorSessionID),
	})
	if err != nil {
		return db.CashCollection{}, err
	}
	occ := in.CollectedAt
	if err := insertOperatorSessionAttribution(ctx, q, operatorAttributionSpec{
		MachineID:         in.MachineID,
		OperatorSessionID: in.OperatorSessionID,
		ActionDomain:      "cash",
		ActionType:        "cash.collection",
		ResourceTable:     "cash_collections",
		ResourceID:        row.ID.String(),
		CorrelationID:     in.CorrelationID,
		OccurredAt:        &occ,
	}); err != nil {
		return db.CashCollection{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return db.CashCollection{}, err
	}
	return row, nil
}

// CreateRefillSessionWithAttribution inserts refill_sessions plus optional attribution.
type CreateRefillSessionWithAttributionInput struct {
	MachineID         uuid.UUID
	StartedAt         time.Time
	EndedAt           *time.Time
	Metadata          []byte
	OperatorSessionID *uuid.UUID
	CorrelationID     *uuid.UUID
}

func (s *Store) CreateRefillSessionWithAttribution(ctx context.Context, in CreateRefillSessionWithAttributionInput) (db.RefillSession, error) {
	meta := in.Metadata
	if len(meta) == 0 {
		meta = []byte("{}")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return db.RefillSession{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)

	_, err = q.GetMachineByIDForUpdate(ctx, in.MachineID)
	if err != nil {
		return db.RefillSession{}, err
	}

	row, err := q.InsertRefillSession(ctx, db.InsertRefillSessionParams{
		MachineID:         in.MachineID,
		StartedAt:         in.StartedAt,
		EndedAt:           optionalTimeToPgTimestamptz(in.EndedAt),
		OperatorSessionID: optionalUUIDToPg(in.OperatorSessionID),
		Metadata:          meta,
	})
	if err != nil {
		return db.RefillSession{}, err
	}
	occ := in.StartedAt
	if err := insertOperatorSessionAttribution(ctx, q, operatorAttributionSpec{
		MachineID:         in.MachineID,
		OperatorSessionID: in.OperatorSessionID,
		ActionDomain:      "refill",
		ActionType:        "refill.session.start",
		ResourceTable:     "refill_sessions",
		ResourceID:        row.ID.String(),
		CorrelationID:     in.CorrelationID,
		OccurredAt:        &occ,
	}); err != nil {
		return db.RefillSession{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return db.RefillSession{}, err
	}
	return row, nil
}

// RecordMachineConfigApplicationWithAttribution inserts machine_configs plus optional attribution.
type RecordMachineConfigApplicationWithAttributionInput struct {
	MachineID         uuid.UUID
	AppliedAt         time.Time
	ConfigRevision    int32
	ConfigPayload     []byte
	Metadata          []byte
	OperatorSessionID *uuid.UUID
	CorrelationID     *uuid.UUID
}

func (s *Store) RecordMachineConfigApplicationWithAttribution(ctx context.Context, in RecordMachineConfigApplicationWithAttributionInput) (db.MachineConfig, error) {
	meta := in.Metadata
	if len(meta) == 0 {
		meta = []byte("{}")
	}
	payload := in.ConfigPayload
	if len(payload) == 0 {
		payload = []byte("{}")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return db.MachineConfig{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)

	_, err = q.GetMachineByIDForUpdate(ctx, in.MachineID)
	if err != nil {
		return db.MachineConfig{}, err
	}

	row, err := q.InsertMachineConfigApplication(ctx, db.InsertMachineConfigApplicationParams{
		MachineID:         in.MachineID,
		AppliedAt:         in.AppliedAt,
		ConfigRevision:    in.ConfigRevision,
		ConfigPayload:     payload,
		OperatorSessionID: optionalUUIDToPg(in.OperatorSessionID),
		Metadata:          meta,
	})
	if err != nil {
		return db.MachineConfig{}, err
	}
	occ := in.AppliedAt
	if err := insertOperatorSessionAttribution(ctx, q, operatorAttributionSpec{
		MachineID:         in.MachineID,
		OperatorSessionID: in.OperatorSessionID,
		ActionDomain:      "config",
		ActionType:        "machine_config.apply",
		ResourceTable:     "machine_configs",
		ResourceID:        row.ID.String(),
		CorrelationID:     in.CorrelationID,
		OccurredAt:        &occ,
	}); err != nil {
		return db.MachineConfig{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return db.MachineConfig{}, err
	}
	return row, nil
}

// CreateIncidentWithAttribution inserts incidents plus optional attribution.
type CreateIncidentWithAttributionInput struct {
	MachineID         uuid.UUID
	Status            string
	Title             string
	OpenedAt          time.Time
	Metadata          []byte
	OperatorSessionID *uuid.UUID
	CorrelationID     *uuid.UUID
}

func (s *Store) CreateIncidentWithAttribution(ctx context.Context, in CreateIncidentWithAttributionInput) (db.Incident, error) {
	meta := in.Metadata
	if len(meta) == 0 {
		meta = []byte("{}")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return db.Incident{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)

	_, err = q.GetMachineByIDForUpdate(ctx, in.MachineID)
	if err != nil {
		return db.Incident{}, err
	}

	now := in.OpenedAt
	row, err := q.InsertIncident(ctx, db.InsertIncidentParams{
		MachineID:         in.MachineID,
		Status:            in.Status,
		Title:             in.Title,
		OpenedAt:          in.OpenedAt,
		UpdatedAt:         now,
		OperatorSessionID: optionalUUIDToPg(in.OperatorSessionID),
		Metadata:          meta,
	})
	if err != nil {
		return db.Incident{}, err
	}
	occ := in.OpenedAt
	if err := insertOperatorSessionAttribution(ctx, q, operatorAttributionSpec{
		MachineID:         in.MachineID,
		OperatorSessionID: in.OperatorSessionID,
		ActionDomain:      "incidents",
		ActionType:        "incident.open",
		ResourceTable:     "incidents",
		ResourceID:        row.ID.String(),
		CorrelationID:     in.CorrelationID,
		OccurredAt:        &occ,
	}); err != nil {
		return db.Incident{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return db.Incident{}, err
	}
	return row, nil
}

// UpdateIncidentFromOperatorWithAttribution updates an incident from operator workflow and records attribution.
type UpdateIncidentFromOperatorWithAttributionInput struct {
	IncidentID        uuid.UUID
	Status            string
	Title             string
	Metadata          []byte
	OperatorSessionID *uuid.UUID
	CorrelationID     *uuid.UUID
}

func (s *Store) UpdateIncidentFromOperatorWithAttribution(ctx context.Context, in UpdateIncidentFromOperatorWithAttributionInput) (db.Incident, error) {
	meta := in.Metadata
	if len(meta) == 0 {
		meta = []byte("{}")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return db.Incident{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)

	row, err := q.UpdateIncidentFromOperator(ctx, db.UpdateIncidentFromOperatorParams{Status: in.Status,
		Title:             in.Title,
		Metadata:          meta,
		OperatorSessionID: optionalUUIDToPg(in.OperatorSessionID),

		ID: in.IncidentID,
	})
	if err != nil {
		return db.Incident{}, err
	}

	occ := time.Now().UTC()
	if err := insertOperatorSessionAttribution(ctx, q, operatorAttributionSpec{
		MachineID:         row.MachineID,
		OperatorSessionID: in.OperatorSessionID,
		ActionDomain:      "incidents",
		ActionType:        "incident.update",
		ResourceTable:     "incidents",
		ResourceID:        row.ID.String(),
		CorrelationID:     in.CorrelationID,
		OccurredAt:        &occ,
	}); err != nil {
		return db.Incident{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return db.Incident{}, err
	}
	return row, nil
}
