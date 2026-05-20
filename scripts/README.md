# Scripts

Repository automation grouped by purpose. Production deploy scripts under `deployments/` are **not** listed here — see [`deployments/prod/README.md`](../deployments/prod/README.md).

| Directory | Purpose |
|-----------|---------|
| [`local/`](local/) | Windows PowerShell local dev helpers |
| [`ci/`](ci/) | CI gates, governance, migration checks, workflow contracts |
| [`deploy/`](deploy/) | Release, security verdict, smoke, monitoring, staging deploy helpers |
| [`db/`](db/) | Database backup evidence, migration guards, schema checks |
| [`test/`](test/) | E2E, coverage, load/loadtest harnesses |
| [`postman/`](postman/) | Postman collection generation and validation |
| [`openapi/`](openapi/) | OpenAPI contract checks and API surface audit helpers |
| [`lib/`](lib/) | Shared shell/PowerShell helpers (`_pslib.ps1`) |

## Backwards-compatible root wrappers

These thin entrypoints delegate to canonical paths (safe for docs/bookmarks):

| Wrapper | Canonical |
|---------|-----------|
| `scripts/api-contract-check.sh` | `scripts/openapi/api-contract-check.sh` |
| `scripts/check_migrations.sh` | `scripts/ci/verify_migrations.sh` |
| `scripts/check_postman_artifacts.sh` | `scripts/postman/check_artifacts.sh` |
| `scripts/generate_postman_collection.sh` | `scripts/postman/generate_collection.sh` |
| `scripts/verify_database_environment.sh` | `scripts/db/verify_database_environment.sh` |
| `scripts/verify_workflow_contracts.sh` | `scripts/ci/verify_workflow_contracts.sh` |

## Makefile entrypoints

See root [`Makefile`](../Makefile): `ci-gates`, `verify-workflows`, `verify-enterprise-release`, loadtest targets, and prod smoke helpers reference paths under this tree.
