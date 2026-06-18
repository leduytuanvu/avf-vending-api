// Package modules currently hosts infrastructure adapters that predate the
// bounded-context layout described in docs/audits/REPO_STRUCTURE_P2_REFACTOR_PROPOSAL.md.
//
// Today the only package here is internal/modules/postgres — the monolithic
// Postgres/sqlc-backed persistence adapter (Store and repository methods).
// Future vertical feature modules may live under internal/<context>/app once
// the P2 strangler migration proceeds; do not add new domain logic here.
package modules
