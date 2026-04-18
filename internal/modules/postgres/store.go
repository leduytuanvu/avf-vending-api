package postgres

import (
	"github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/domain/device"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store is the Postgres-backed implementation surface for platform workflows.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore returns a Store backed by pool.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Pool exposes the underlying pool for health checks and tests.
func (s *Store) Pool() *pgxpool.Pool {
	return s.pool
}

var (
	_ commerce.OrderVendWorkflow     = (*Store)(nil)
	_ commerce.PaymentOutboxWorkflow = (*Store)(nil)
	_ device.CommandShadowWorkflow   = (*Store)(nil)
)
