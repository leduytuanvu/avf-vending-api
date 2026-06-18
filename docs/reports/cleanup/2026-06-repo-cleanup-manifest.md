# 2026-06 repository cleanup manifest

**Status:** Phase 1 executed; Phases 2–4 doc/script/CI hygiene; Phases 5–6 deferred (KEEP).  
**Branch baseline:** `feat/production-self-hosted-deploy-runner`  
**Canonical architecture roadmap:** [`../../audits/REPO_STRUCTURE_P2_REFACTOR_PROPOSAL.md`](../../audits/REPO_STRUCTURE_P2_REFACTOR_PROPOSAL.md)

## Deletions (Phase 1)

| Path | Action | Reason | Rollback |
|------|--------|--------|----------|
| `pkg/doc.go` | DELETE | Doc-only placeholder; zero Go imports; no external public library yet | `git restore pkg/doc.go` |
| `internal/service/cash/doc.go` | DELETE | Doc-only; cash settlement lives in `internal/modules/postgres` + admin HTTP | `git restore internal/service/cash/doc.go` |
| `internal/repository/cash/doc.go` | DELETE | Doc-only; SQL in `db/queries/cash.sql` via `internal/gen/db` | `git restore internal/repository/cash/doc.go` |

## Rewrites (Phase 1)

| Path | Action | Reason |
|------|--------|--------|
| `internal/modules/doc.go` | REWRITE | Clarify `internal/modules/postgres` is persistence adapter, not vertical feature modules |
| `docs/audits/REPO_STRUCTURE_P2_REFACTOR_PROPOSAL.md` | UPDATE | Record placeholder removal; PR 2 scope adjusted |

## Documentation (Phase 2)

**No operational runbooks, production checklists, or verification artifacts were deleted.**

| Path | Action | Reason |
|------|--------|--------|
| `docs/README.md` | UPDATE | Canonical vs non-canonical navigation |
| `docs/audits/README.md` | UPDATE | Active vs archived audit guidance |
| `docs/archive/README.md` | UPDATE | Link to this manifest |
| `docs/archive/cleanup/README.md` | UPDATE | Fix broken links to non-existent audit paths |
| `docs/reports/README.md` | UPDATE | Link cleanup reports |
| `docs/runbooks/README.md` | UPDATE | Canonical runbook pointer |

## Explicitly kept (Phases 3–6)

| Path | Decision | Reason |
|------|----------|--------|
| Root `scripts/*.sh` wrappers | KEEP | Backwards-compatible entrypoints; CI/docs references |
| `.github/workflows/deploy-production.yml` | KEEP | Notice-only pointer; required by governance script |
| `.github/workflows/deploy-prod.yml` | KEEP | Canonical production deploy |
| `deployments/prod/legacy/**` | KEEP | Rollback/operator reference |
| `internal/modules/postgres` | KEEP (Phase 5 deferred) | 58 importers; move only in dedicated high-risk PR |
| `internal/httpserver`, `internal/grpcserver` | KEEP (Phase 6 deferred) | Single-package transport; split only with import-graph proof |

## Verification commands run

```bash
go list ./...
go build ./...
go vet ./...
go test ./... -short
```

`make ci-gates`, `make verify-workflows`, `make api-contract-check` — run in CI or Git Bash on Linux/macOS (requires bash, python3, buf, sqlc, actionlint).

## Phase 3 (scripts / Makefile)

| Path | Action | Notes |
|------|--------|-------|
| Root `scripts/*.sh` wrappers | KEEP | Documented in `scripts/README.md`; do not delete |
| `Makefile` | UPDATE | Windows/bash note on `PY` and script targets |
| `scripts/README.md` | UPDATE | Windows/Git Bash guidance |

Local `bash scripts/ci/verify_governance_protection_window.sh` — **BLOCKED BY LOCAL TOOLING** on this Windows host (WSL bash unavailable). Run in CI.

## Phase 4 (CI/CD / deployment)

| Path | Action | Notes |
|------|--------|-------|
| `.github/workflows/deploy-production.yml` | KEEP | Notice-only; contains `NO REAL DEPLOY`; required by governance script |
| `.github/workflows/deploy-prod.yml` | KEEP | Canonical `name: Deploy Production` |
| `deployments/prod/legacy/**` | KEEP | Rollback reference |

Static verification (2026-06): `deploy-production.yml` header and `verify_governance_protection_window.sh` grep contracts confirmed by file inspection. Full script pass deferred to CI.

## Phase 5 (internal/modules/postgres)

**Decision: KEEP path.** Move to `internal/adapters/postgres` or context adapters deferred to P2 PR 4 (58 importers; high blast radius).

## Phase 6 (transport packages)

**Decision: KEEP single package.** Added `internal/httpserver/doc.go` route-group index. `internal/grpcserver` already documents service layout in `server.go` package comment. No file splits.

## Phase 7 — final verification (2026-06-19)

| Check | Result |
|-------|--------|
| `gofmt` on changed Go files | OK |
| `go list ./...` | 108 packages (was 111; −3 placeholder packages) |
| `go build ./...` | exit 0 |
| `go vet ./...` | exit 0 |
| `go test ./... -short` | exit 0 |
| `make ci-gates` | BLOCKED BY LOCAL TOOLING (bash/WSL unavailable on Windows host) |
| `make verify-workflows` | defer to CI |
| `make api-contract-check` | defer to CI |

### Files changed summary

**Deleted:** `pkg/doc.go`, `internal/service/cash/doc.go`, `internal/repository/cash/doc.go`

**Added:** `internal/httpserver/doc.go`, `docs/reports/cleanup/2026-06-repo-cleanup-manifest.md`

**Updated:** `internal/modules/doc.go`, `docs/audits/REPO_STRUCTURE_P2_REFACTOR_PROPOSAL.md`, `docs/README.md`, `docs/audits/README.md`, `docs/archive/README.md`, `docs/archive/cleanup/README.md`, `docs/reports/README.md`, `docs/runbooks/README.md`, `Makefile`, `scripts/README.md`

### Rollback

```bash
git restore .
# or per-phase:
git revert <commit>
```

### Follow-up (not in this cleanup)

- P2 PR 2: pure helpers + port interfaces
- P2 PR 4: split `internal/modules/postgres` into context adapters (58 importers)
- Optional httpserver/grpcserver package split (import-graph proof required)
- Run `make ci-gates` and `make verify-workflows` in CI before merge
