# Staging-to-production release gate (CI/CD contract)

**Staging Deployment Contract** (`.github/workflows/deploy-develop.yml`) must upload the artifact **`staging-release-evidence`** containing `deployment-evidence/staging-release-evidence.json` with `schema_version: "staging-release-evidence-v1"`.

- **Real pre-prod** (`vars.ENABLE_REAL_STAGING_DEPLOY == 'true'`): the JSON uses `deployment_mode: "real_staging"` and sets `meets_production_staging_gate` when health and smoke complete successfully. Image refs are digest-pinned; `app_image_digest` / `goose_image_digest` are included.
- **Contract-only** (real staging not enabled, or org policy): the same artifact name is still produced with `deployment_mode: "contract_only"` and `meets_production_staging_gate: false` — it **must not** satisfy **Deploy Production**’s strict path.

**Deploy Production** (`.github/workflows/deploy-prod.yml`) defaults to a **strict** staging gate in `action_mode: deploy`: operators pass **`staging_evidence_id`** (the GitHub Actions run id of a successful *Staging Deployment Contract* run). The workflow downloads the `staging-release-evidence` artifact, checks `meets_production_staging_gate`, requires `deployment_mode: "real_staging"`, and matches **app/goose image digests** to the production image inputs (develop vs main build id may differ; digests are authoritative).

A temporary organizational bypass exists: **`allow_missing_staging_evidence: true`** with a non-empty **`missing_staging_evidence_reason`**. The run emits warnings and records the bypass in the job summary and `production-deployment-manifest.json`.

**Offline contract checks** (run in PR CI): `bash scripts/ci/verify_workflow_contracts.sh` and `python tools/verify_github_workflow_cicd_contract.py` assert the staging artifact, deploy-prod inputs, and in-workflow validation logic remain present.

## Local / PR acceptance (enterprise readiness)

From repo root, these should pass before merge (same substance as the workflow quality job in CI, plus an explicit Python pass optional when debugging):

- `actionlint`
- `bash scripts/ci/verify_workflow_contracts.sh` (includes the Python tool below; do not skip)
- `python tools/verify_github_workflow_cicd_contract.py` (also invoked by the shell script)
- `make verify-workflows` or `make ci-workflows` — runs `actionlint` then the shell contract script; requires `make` and `actionlint` on `PATH`
- `git diff --check`

`Deploy Production` (`.github/workflows/deploy-prod.yml`) must be plain UTF-8 without BOM, LF line endings, and the first line must be exactly `name: Deploy Production` (BOM/CRLF break offline checks).
