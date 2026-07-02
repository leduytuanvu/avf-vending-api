# 2026-07-02 repository cleanup baseline

**Branch at baseline:** `chore/sync-local-main-20260702`  
**Commit:** `c69a5995df3b7fc2298da32504cee56549824213` — Consolidate production Postman suite and add deploy artifact metadata.  
**Cleanup branch created:** `cleanup/repo-structure-and-junk-files`  
**Date:** 2026-07-02

## Repository status

| Check | Result |
|-------|--------|
| `git status --short` | Clean working tree |
| Tracked files (`git ls-files`) | 1796 |
| Migrations (`migrations/`) | 15 files |
| GitHub workflows | 20 files |

## Root directory (tracked)

Standard project entrypoints only: `.dockerignore`, `.env.example`, `.env.local.example`, `.env.production.example`, `.env.staging.example`, `.gitattributes`, `.gitignore`, `.gitleaks.toml`, `.trivyignore`, `Makefile`, `README.md`, `go.mod`, `go.sum`, `repomix.config.json`, `sqlc.yaml`, `trivy.yaml`.

## Known pre-cleanup issues

| Issue | Evidence |
|-------|----------|
| Postman CI paths missing | `postman/environments/`, `postman/production/`, `postman/scripts/` removed in c69a5995; CI still references them |
| Committed deploy artifacts | `_deploy_artifacts/` (7 files) — local Security Release snapshot; zero external references |
| New full suite at `postman/` root | `postman/avf-vending-production.full.*` added; needs relocation to `postman/suites/production-full/` |

## Postman tracked files at baseline

```
postman/README.md
postman/avf-vending-production.full.postman_collection.json
postman/avf-vending-production.full.postman_environment.json
postman/collections/avf-vending-api-function-path.postman_collection.json
postman/collections/avf-vending-api.postman_collection.json
```

## `_deploy_artifacts/` at baseline

```
_deploy_artifacts/README.md
_deploy_artifacts/deploy-production-gh-command.sh
_deploy_artifacts/production-deploy-candidate-metadata.json
_deploy_artifacts/production-deploy-inputs.env
_deploy_artifacts/production-deploy-inputs.json
_deploy_artifacts/release-candidate/release-candidate.json
_deploy_artifacts/security-verdict/security-verdict.json
```

## Ignore policy summary

- `.gitignore` excludes local env, build artifacts, Newman output, `postman/generated/`, local deploy evidence, etc.
- `_deploy_artifacts/` is **not** yet ignored (to be added in this cleanup).
- `repomix.config.json` ignores heavy Postman JSON and local report paths.

## Baseline test results

| Command | Result | Notes |
|---------|--------|-------|
| `go test ./... -short` | **PASS** | exit 0 |
| `go vet ./...` | **PASS** | exit 0 |
| `make test-short` | Not run | Equivalent to `go test ./... -short` |
| `make postman-check` | **Expected FAIL** | Missing `postman/environments/` |
| `bash scripts/ci/verify_production_postman_parity.sh` | **Expected FAIL** | Missing `postman/production/` |
| `make api-contract-check` | Deferred | Requires bash, python3, buf, sqlc (Git Bash/WSL/CI) |
| `make verify-workflows` | Deferred | Requires actionlint + bash |
| `make verify-enterprise-release` | Deferred | Requires bash |
| `make check-migrations` | Deferred | Requires bash |

## Protected assets confirmed present

- `migrations/`, `db/queries/`, `db/schema/`
- `internal/gen/` (sqlc + protobuf generated)
- `.github/workflows/deploy-prod.yml`, `deploy-production.yml` (pointer)
- `deployments/prod/`, `deployments/staging/`, `deployments/docker/`
- `scripts/ci/`, `scripts/deploy/`, `docs/runbooks/`

## Prior cleanup reference

[`2026-06-repo-cleanup-manifest.md`](2026-06-repo-cleanup-manifest.md) — Phase 1–4 completed; P2 internal refactors deferred.
