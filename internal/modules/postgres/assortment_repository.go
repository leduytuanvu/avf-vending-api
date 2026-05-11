package postgres

import (
	"context"

	"github.com/avf/avf-vending-api/internal/app/assortmentapp"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AssortmentRepository implements assortmentapp.Repository.
type AssortmentRepository struct {
	pool *pgxpool.Pool
}

// NewAssortmentRepository returns a Postgres-backed assortment binder.
func NewAssortmentRepository(pool *pgxpool.Pool) *AssortmentRepository {
	if pool == nil {
		panic("postgres.NewAssortmentRepository: nil pool")
	}
	return &AssortmentRepository{pool: pool}
}

var _ assortmentapp.Repository = (*AssortmentRepository)(nil)

// BindMachineAssortment closes any active primary binding and inserts a new primary assortment for the machine.
func (r *AssortmentRepository) BindMachineAssortment(ctx context.Context, in assortmentapp.BindMachineAssortmentInput) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := db.New(tx)
	m, err := q.GetMachineByIDForUpdate(ctx, in.MachineID)
	if err != nil {
		if isNoRows(err) {
			return assortmentapp.ErrBindFailed
		}
		return err
	}

	if _, err := tx.Exec(ctx, `
UPDATE machine_assortment_bindings
SET valid_to = now()
WHERE organization_id = $1
  AND machine_id = $2
  AND is_primary
  AND valid_to IS NULL
`, m.OrganizationID, in.MachineID); err != nil {
		return err
	}

	tag, err := tx.Exec(ctx, `
INSERT INTO machine_assortment_bindings (
    organization_id,
    machine_id,
    assortment_id,
    is_primary,
    valid_from
)
SELECT
    $1,
    $2,
    a.id,
    TRUE,
    now()
FROM assortments a
WHERE a.id = $3
  AND a.organization_id = $1
`, m.OrganizationID, in.MachineID, in.AssortmentID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return assortmentapp.ErrBindFailed
	}

	if err := insertOperatorSessionAttribution(ctx, q, operatorAttributionSpec{
		MachineID:         in.MachineID,
		OrganizationID:    m.OrganizationID,
		OperatorSessionID: in.OperatorSessionID,
		ActionDomain:      "assortment",
		ActionType:        "machine.bind_primary",
		ResourceTable:     "machine_assortment_bindings",
		ResourceID:        in.AssortmentID.String(),
		CorrelationID:     in.CorrelationID,
		OccurredAt:        nil,
	}); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
