package postgres

import (
	"github.com/avf/avf-vending-api/internal/app/pricingengine"
	"github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/domain/device"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store is the Postgres-backed implementation surface for platform workflows.
type Store struct {
	pool            *pgxpool.Pool
	enterpriseAudit compliance.EnterpriseRecorder
	pricing         *pricingengine.Engine
}

// StoreOption configures Store construction.
type StoreOption func(*Store)

// WithEnterpriseAudit wires mandatory audit_events writes for transactional workflows (e.g. payment webhooks).
func WithEnterpriseAudit(rec compliance.EnterpriseRecorder) StoreOption {
	return func(s *Store) {
		s.enterpriseAudit = rec
	}
}

// NewStore returns a Store backed by pool.
func NewStore(pool *pgxpool.Pool, opts ...StoreOption) *Store {
	s := &Store{pool: pool, pricing: pricingengine.New(pool)}
	for _, o := range opts {
		o(s)
	}
	return s
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
