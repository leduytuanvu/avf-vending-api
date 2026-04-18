// Package operator implements machine operator sessions, audit reads, and HTTP-facing workflows.
//
// HTTP surfaces (all require Authorization: Bearer JWT):
//   - GET /v1/machines/{machineId}/operator-sessions/current — active session (if any)
//   - GET /v1/machines/{machineId}/operator-sessions/history — past sessions
//   - GET /v1/machines/{machineId}/operator-sessions/auth-events — auth audit for the machine
//   - GET /v1/machines/{machineId}/operator-sessions/action-attributions — domain action rows linked to operators
//   - GET /v1/machines/{machineId}/operator-sessions/timeline — merged operational view (auth + attributions + session markers)
//   - POST …/login, …/logout, …/{sessionId}/heartbeat
//   - GET /v1/operator-insights/technicians/{technicianId}/action-attributions — cross-machine (tenant scoped)
//   - GET /v1/operator-insights/users/action-attributions?user_principal=… — same for USER actor
//
// Platform admins without organization_id on the JWT must pass organization_id as a query parameter on operator-insights routes.
package operator
