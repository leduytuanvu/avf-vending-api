package outbox

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AdminService exposes platform-operator mutations on transactional outbox rows.
type AdminService struct {
	pool *pgxpool.Pool
}

func NewAdminService(pool *pgxpool.Pool) *AdminService {
	if pool == nil {
		return nil
	}
	return &AdminService{pool: pool}
}

func (s *AdminService) ReplayDeadLetter(ctx context.Context, id int64) (int64, error) {
	if s == nil || s.pool == nil {
		return 0, fmt.Errorf("outbox: admin service not configured")
	}
	return db.New(s.pool).AdminRetryOutboxDeadLetter(ctx, id)
}

func (s *AdminService) MarkManualDLQ(ctx context.Context, id int64, note string) (int64, error) {
	if s == nil || s.pool == nil {
		return 0, fmt.Errorf("outbox: admin service not configured")
	}
	return db.New(s.pool).AdminMarkOutboxManualDeadLetter(ctx, db.AdminMarkOutboxManualDeadLetterParams{
		ID:   id,
		Note: note,
	})
}

// ReplayDeadLetterTx retries a dead-letter row and records enterprise audit in the same PostgreSQL transaction.
func (s *AdminService) ReplayDeadLetterTx(ctx context.Context, id int64, audit compliance.EnterpriseRecorder, rec compliance.EnterpriseAuditRecord) (int64, error) {
	if s == nil || s.pool == nil {
		return 0, fmt.Errorf("outbox: admin service not configured")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	qtx := db.New(s.pool).WithTx(tx)
	n, err := qtx.AdminRetryOutboxDeadLetter(ctx, id)
	if err != nil {
		return 0, err
	}
	if n > 0 {
		if audit == nil {
			return 0, fmt.Errorf("outbox: enterprise audit is required for replay")
		}
		if err := audit.RecordCriticalTx(ctx, tx, rec); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return n, nil
}

// MarkManualDLQTx marks a row dead-lettered with an optional operator note and records enterprise audit in the same transaction.
func (s *AdminService) MarkManualDLQTx(ctx context.Context, id int64, note string, audit compliance.EnterpriseRecorder, rec compliance.EnterpriseAuditRecord) (int64, error) {
	if s == nil || s.pool == nil {
		return 0, fmt.Errorf("outbox: admin service not configured")
	}
	if audit == nil {
		return 0, fmt.Errorf("outbox: enterprise audit is required for manual DLQ")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	qtx := db.New(s.pool).WithTx(tx)
	n, err := qtx.AdminMarkOutboxManualDeadLetter(ctx, db.AdminMarkOutboxManualDeadLetterParams{
		ID:   id,
		Note: note,
	})
	if err != nil {
		return 0, err
	}
	if n > 0 {
		if err := audit.RecordCriticalTx(ctx, tx, rec); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return n, nil
}

func (s *AdminService) GetByID(ctx context.Context, id int64) (db.OutboxEvent, error) {
	if s == nil || s.pool == nil {
		return db.OutboxEvent{}, errors.New("outbox: admin service not configured")
	}
	return db.New(s.pool).AdminGetOutboxEventByID(ctx, id)
}

// ListPendingWindow returns unpublished, non-terminal rows with created_at in [createdAfter, createdBefore).
// statusFilter empty selects all such statuses; otherwise must match a single status value (e.g. pending, failed).
func (s *AdminService) ListPendingWindow(ctx context.Context, createdAfter, createdBefore time.Time, statusFilter string, limit int32) ([]db.OutboxEvent, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("outbox: admin service not configured")
	}
	if !createdBefore.After(createdAfter) {
		return nil, fmt.Errorf("outbox: createdBefore must be after createdAfter")
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	return db.New(s.pool).AdminListOutboxEventsPendingWindow(ctx, db.AdminListOutboxEventsPendingWindowParams{
		CreatedAfter:  createdAfter.UTC(),
		CreatedBefore: createdBefore.UTC(),
		StatusFilter:  statusFilter,
		Limit:         limit,
	})
}

// RequeuePendingByID clears worker lease and publish backoff on a stuck unpublished row (pending/failed/publishing).
// It does not reset Postgres dead-letter quarantine; use ReplayDeadLetter after reviewing poison payloads.
func (s *AdminService) RequeuePendingByID(ctx context.Context, id int64, operatorNote string) (int64, error) {
	if s == nil || s.pool == nil {
		return 0, fmt.Errorf("outbox: admin service not configured")
	}
	return db.New(s.pool).AdminRequeueOutboxPendingByID(ctx, db.AdminRequeueOutboxPendingByIDParams{
		ID:   id,
		Note: operatorNote,
	})
}

// ReplayDeadLetterAfterConfirm wraps ReplayDeadLetter with RequirePoisonReplayConfirmation.
func (s *AdminService) ReplayDeadLetterAfterConfirm(ctx context.Context, id int64, confirmed bool) (int64, error) {
	if err := RequirePoisonReplayConfirmation(confirmed); err != nil {
		return 0, err
	}
	return s.ReplayDeadLetter(ctx, id)
}
