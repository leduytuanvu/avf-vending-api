// Package reliability hosts background-oriented reconciliation: stuck payments, stale commands,
// orphan vend sessions, and outbox republish planning.
//
// Callers are expected to be workers or reconciler binaries; there is no cron or HTTP here.
// Persistence discovery lives behind finder/repository ports; implement them in internal/modules/postgres.
//
// TODO: Add sqlc queries + postgres adapters for StuckPaymentFinder, StuckCommandFinder, OrphanVendFinder.
package reliability
