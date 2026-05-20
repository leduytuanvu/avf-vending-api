# Repository junk cleanup — final report (2026-05-20)

**Branch:** `chore/cleanup-junk-docs`  
**Scope:** Remove stale grep artifacts, harden `.gitignore`, add audit documentation. No Go code, migrations, deploy scripts, or workflow changes.

---

## Summary

- Audited **281** tracked Markdown files and local junk directories (`.tmp-phase*/`, `tmp/`)
- Deleted **5** stale empty/placeholder grep capture files under `docs/reports/final-gate-audit/` and `final-single-scope-audit/`
- Updated gate **final-report.md** files to preserve zero-hit evidence without empty artifact files
- Hardened **`.gitignore`** for OS junk, coverage, temp dirs, local keys, Newman output
- Created **`REPO_JUNK_CLEANUP_AUDIT.md`** (this pass inventory)

**Not deleted:** `docs/operations/**` CI stubs, `docs/api/device-offline-replay-samples.md` (unique gRPC content, unlinked), all production/deploy/test docs.

---

## Deleted files

| Path | Reason |
|------|--------|
| `docs/reports/final-gate-audit/all-hits-before.txt` | Zero-byte stale grep capture; 0 references |
| `docs/reports/final-gate-audit/all-hits-final.txt` | Zero-byte stale grep capture; 0 references |
| `docs/reports/final-gate-audit/path-hits-before.txt` | Sanitized placeholder superseded by `classification.md` |
| `docs/reports/final-gate-audit/path-hits-final.txt` | Zero-byte stale grep capture; 0 references |
| `docs/reports/final-single-scope-audit/final-zero-hit-grep.txt` | Zero-byte stale grep capture; result documented in `final-report.md` |

---

## Moved files

None this pass.

---

## Merged Markdown files

None this pass.

---

## .gitignore updates

| Pattern | Why |
|---------|-----|
| `.DS_Store` | macOS junk |
| `*.coverage` | Local coverage artifacts |
| `temp/` | Local scratch alongside `tmp/` |
| `db-backups/` | Local DB backup folders |
| `deployment-evidence/local/` | Local deploy evidence scratch |
| `*.env.local` | Local env overrides (`.env.example` still tracked) |
| `id_rsa`, `*.pem`, `*.key` | Local private keys (no tracked cert files in repo) |
| `newman/`, `reports/newman/` | Local Newman CLI output |

---

## Files intentionally not touched

- `migrations/**`, `deployments/**`, `.github/workflows/**`
- Dockerfiles, docker-compose files
- Postman/OpenAPI canonical JSON and generators
- `tests/**`, `scripts/ci/validate-production-deploy.sh`, deploy/release scripts
- `docs/operations/**` CI contract stubs
- Production runbooks, security docs, deployment/migration docs

---

## Validation results

| Command | Result |
|---------|--------|
| `gofmt -w .` | PASS |
| `go test ./...` | PASS |
| `go vet ./...` | PASS |
| `go list ./...` | PASS |
| `bash scripts/ci/validate-production-deploy.sh` | PASS |
| UUID v7 checks | PASS |
| JSON validation (Python) | PASS |
| `bash -n` on shell scripts | PASS (Git Bash) |

---

## Remaining risks

| Item | Notes |
|------|-------|
| `docs/api/device-offline-replay-samples.md` | Unique gRPC offline-sync doc; no inbound links (MQTT samples use `examples/`). Consider linking from `machine-grpc.md` in a follow-up. |
| `docs/operations/` stubs | Still required by CI; removal needs coordinated workflow update (P2). |
| Local `.tmp-phase*/` dirs | Gitignored; operators may delete manually. |

---

## Next recommended cleanup (optional)

1. Update workflows + `verify_workflow_contracts.sh` to canonical `docs/deployment/` paths; remove `docs/operations/` stubs
2. Add cross-link for `docs/api/device-offline-replay-samples.md` from machine gRPC docs
