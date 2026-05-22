# Repomix slim export for source review

Use Repomix to produce a **smaller bundle for ChatGPT/code review** without deleting source files from the repository.

## Full export (everything tracked, minus gitignore)

```bash
npx repomix
# or
repomix -o repomix-output.xml
```

Uses `repomix.config.json`, `.gitignore`, and `.repomixignore`.

## Slim source-review export (recommended)

```bash
npx repomix -c repomix.config.slim.json -o repomix-output-slim.xml
```

### Included (core review surface)

- `cmd/`, `internal/`, `pkg/`, `api/`, `db/`, `migrations/`, `proto/`
- `deployments/prod/` (production topology)
- `.github/workflows/`
- `postman/production-full-suite/` (current production Postman flow)
- `docs/runbooks/`, `docs/testing/`
- `README.md`, `Makefile`, `go.mod`, `sqlc.yaml`, `trivy.yaml`

### Excluded from slim export (still in git)

| Path | Why |
|------|-----|
| `postman/suites/`, `postman/generated/` | Large generated Postman/OpenAPI inventory (~20MB+) |
| `docs/swagger/swagger.json` | Generated OpenAPI; rebuild via `tools/build_openapi.py` |
| `docs/reports/`, `docs/audits/` | Historical verification/audit archives |
| Local scratch | `.tmp-*`, `tmp/`, `ci-reports/`, coverage, logs (see `.gitignore`) |

## Important

- **Do not delete** migrations, proto, workflows, or `postman/production-full-suite/` just to shrink Repomix.
- Regenerate large Postman/OpenAPI artifacts with existing scripts when needed.
- Never commit `repomix-output*.xml` or local env files.

## Validate after export

```bash
python postman/production-full-suite/validate_product_flow_suite.py
go test ./internal/app/mediaadmin/... ./internal/httpserver/...
```

See also: `docs/reports/REPO_SLIM_CLEANUP_REPORT.md`.
