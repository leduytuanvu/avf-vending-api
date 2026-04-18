// Package fleet implements application-layer workflows for deployed machines and technician coverage.
//
// Callers pass plain value inputs (organization scope, identifiers, metadata patches); there is no
// coupling to HTTP or transport types. Persistence is behind FleetRepository; adapters should live
// in internal/modules/postgres (sqlc-backed queries).
//
// TODO: Implement FleetRepository in internal/modules/postgres and regenerate sqlc for insert/update/list/assignment.
package fleet
