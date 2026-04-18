// Package operator models machine-bound operator sessions, authentication audit, and action attribution.
//
// Machine identity vs operator identity:
// The vending device and its shadow/commands are keyed by machine_id — that is fleet hardware identity.
// Operators (technicians or org users) are separate principals; a session binds one human actor to one
// machine for a bounded time so field actions can be attributed without conflating device credentials
// with human credentials.
//
// Why sessions are temporary:
// Operator sessions are short-lived login contexts (similar to kiosk mode). They expire or are closed
// explicitly so unattended machines do not retain an open human context, and so audit trails have clear
// start/end boundaries for compliance and support.
//
// Session exclusivity: at most one ACTIVE machine_operator_sessions row per machine_id (DB partial
// unique index + transactional start). Same-principal login resumes the ACTIVE row (crash/reconnect).
// A different principal receives ErrActiveSessionExists (HTTP 409) until the prior session is idle
// beyond the stale reclaim window, an authorized admin issues force_admin_takeover, or the session
// is ended explicitly (logout / revoke).
//
// How to use attribution in future modules:
// When a domain mutation is performed on behalf of a logged-in operator, persist operator_session_id on
// the business row when the schema supports it, and insert one machine_action_attributions row in the
// same transaction (see internal/modules/postgres/operator_attribution.go). Use resource_type/resource_id
// as stable pointers back to the business table; put cross-cutting fields (action_domain, actor_type,
// organization_id, etc.) in metadata JSON for inspection APIs without widening the attributions table
// for every new workflow.
package operator
