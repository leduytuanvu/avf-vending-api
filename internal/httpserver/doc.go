// Package httpserver is the REST transport adapter for cmd/api (Chi router, /v1 mounts).
//
// Intentionally a single flat package (~100 files) to avoid import cycles and cross-package
// helper export churn. Future splits (one route group at a time) require import-graph analysis;
// see docs/audits/REPO_STRUCTURE_P2_REFACTOR_PROPOSAL.md Phase 6.
//
// Route group index (file naming convention):
//
//   - admin_*_http.go     — Admin REST (/v1/admin/*): catalog, inventory, fleet, commerce, cash, OTA, audit, outbox, finance, …
//   - auth_*.go, operator_http.go, activation_http.go, setup — auth, operator, activation, setup flows
//   - commerce_*.go       — public commerce HTTP and payment webhooks
//   - machine_*.go, device_*.go — deprecated machine REST (legacy); prefer avf.machine.v1 gRPC
//   - sale_catalog_http.go, reporting_http.go, reports_http.go, telemetry_*.go — read/report surfaces
//   - server.go, router.go, readiness.go, swagger*.go — wiring, health, OpenAPI embed
//
// OpenAPI annotations: swagger_operations.go (registry) + per-handler comments.
package httpserver
