# CI scripts

Offline gates run locally via `make ci-gates`, `make verify-workflows`, and `.github/workflows/ci.yml`.

Key entrypoints:

- `verify_workflow_contracts.sh` — workflow contract + supply chain + deployment secrets
- `verify_migrations.sh` — goose migration safety
- `verify_github_governance.sh` — branch/environment governance (live GitHub API)
- `verify_enterprise_release.sh` — enterprise release verification bundle
- `check_production_placeholders.sh`, `check_feature_wiring.sh`, `check_stale_p0_docs.sh`
