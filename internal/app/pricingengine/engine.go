// Package pricingengine is the single runtime authority for vending list prices,
// machine price overrides, and promotion discounts. Catalog, checkout, payment
// sessions, and admin previews should delegate here to avoid amount drift.
package pricingengine

import (
	"github.com/jackc/pgx/v5/pgxpool"
)

// Engine evaluates prices using Postgres-backed catalog and promotion state.
type Engine struct {
	pool *pgxpool.Pool
}

// New returns an engine backed by pool.
func New(pool *pgxpool.Pool) *Engine {
	if pool == nil {
		return nil
	}
	return &Engine{pool: pool}
}
