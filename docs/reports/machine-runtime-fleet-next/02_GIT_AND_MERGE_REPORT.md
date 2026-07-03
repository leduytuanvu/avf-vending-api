# Git and Merge Report — Machine Runtime Fleet (fix pass)

**Date:** 2026-07-04

## Commits

| SHA | Message |
|-----|---------|
| `8c7b806c` | feat(fleet): machine runtime app sessions and device attachments |
| `48481a59` | fix(fleet): close runtime fleet correctness gaps before production gate |

## Pull request

- **PR:** https://github.com/leduytuanvu/avf-vending-api/pull/409
- **Head:** `feature/machine-runtime-fleet`
- **Base:** `develop`
- **Status:** Open — awaiting CI green before merge

## develop → main parity

**Not yet merged.** `develop→main` promotion blocked until PR #409 merges and CI passes.

## Files in fix commit (high level)

- `migrations/00018_machine_runtime_fleet_fixes.sql`
- SQLC queries + generated Go
- `internal/app/machineruntime/*`, `activation/reattach.go`, `fleet/*`
- `internal/httpserver/admin_machine_enterprise_http.go`
- `internal/grpcserver/machine_runtime_session_grpc.go`
- `proto/avf/machine/v1/runtime_session.proto`
- `tools/production_full_test/*`
- `docs/reports/machine-runtime-fleet-next/00-01`
