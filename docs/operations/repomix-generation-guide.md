# Repomix generation guide

Repomix packs repository text for LLM review. This repo keeps **generators and contracts** in git; **large generated JSON** (Postman collections, Newman reports, E2E run dirs) stays out of Repomix via [`repomix.config.json`](../../repomix.config.json).

## Recommended command

From repo root (requires [Repomix](https://repomix.com) CLI):

```bash
npx repomix@latest --config repomix.config.json
```

Output: `repomix-output.xml` (gitignored).

## What is excluded (and why)

| Pattern | Reason |
|---------|--------|
| `.git/`, `vendor/`, `node_modules/` | Not source; huge |
| `.e2e-runs/`, `.production-smoke-runs/`, `.production-latency-runs/`, `.test-runs/` | Local run evidence |
| `coverage/`, `coverage.out`, `ci-reports/`, `security-reports/` | CI scratch |
| `build/reports/` | Regenerable inventory JSON |
| `postman/**/*.postman_collection.json` | Large; regenerate via suite generators |
| `postman/generated/API_INVENTORY_CANONICAL.json` | Regenerable canonical inventory |
| `docs/testing/production-e2e/RESULTS_*.md`, `API_TRACE_*.md`, timestamped `POSTMAN_*` | Per-run E2E reports |
| `repomix-output*.xml`, `.repomix/` | Repomix output itself |

## What is included

- `cmd/`, `internal/`, `pkg/`, `api/`, `proto/`, `migrations/`, `db/`
- `.github/workflows/`, `deployments/`, Dockerfiles, compose files
- `scripts/`, `tools/`, `tests/` (including fixtures)
- `docs/` (canonical runbooks, architecture, API contracts)
- Postman **generators**, validators, matrices (CSV/MD), `postman/production/` E2E manifest outputs
- `go.mod`, `go.sum`, OpenAPI source (`tools/build_openapi.py`), committed `docs/swagger/swagger.json`

## Smaller focused packs

Use Repomix `--include` overrides for targeted review:

```bash
# Backend only
npx repomix@latest --include "cmd/**,internal/**,pkg/**,migrations/**,proto/**"

# API contract only
npx repomix@latest --include "api/**,docs/swagger/**,tools/build_openapi.py,tools/openapi_verify_release.py,postman/production/**"

# CI/CD only
npx repomix@latest --include ".github/**,scripts/ci/**,Makefile,docs/deployment/**,docs/cicd/**"
```

## After cleanup

Regenerate large Postman JSON when needed:

```bash
python postman/suites/full-production-suite/generate_full_postman_suite.py
python postman/suites/full-production-suite/validate_generated_assets.py
```

See also [`../audits/project-cleanup-audit.md`](../audits/project-cleanup-audit.md).
