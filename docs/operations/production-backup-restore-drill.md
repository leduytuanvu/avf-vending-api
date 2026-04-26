# Production database backup, restore drill, and migration evidence

Production schema migrations (goose `Up` during **Deploy Production**) are gated on **verifiable** backup and restore-drill evidence. A free-text ticket id alone is **not** enough when `run_migration` is `true`.

## When backup and drill evidence is required

- **`run_migration=true`** in the **Deploy Production** workflow: you must supply `backup_evidence_id` and pass automated validation **before** any SSH to production or migration step.
- **`run_migration=false`**: image-only deploy. `backup_evidence_id` is optional; if you set it, it is **not** schema-validated (informational only, shown in the job summary).

## Contract

- **JSON schema (reference)**: [docs/contracts/backup-evidence.schema.json](../contracts/backup-evidence.schema.json)  
- **Validator (authoritative)**: `scripts/ci/validate_backup_evidence.py`  
- **Evidence type**: `evidence_type` must be `production-db-backup`; `schema_version` is `backup-evidence-v1`.  
- **Key rules** (see script for the full policy):
  - `source_environment` must be `production`.
  - `backup_sha256` and either `backup_artifact_id` or `backup_location_redacted` (non-secret) are required.
  - `expires_at_utc` must be in the future at validation time.
  - If `restore_drill_required` is `true`, `restore_drill_result` must be `pass`, drill timestamps/target must be set, and `schema_validation_result` must be `pass` for a production migration.
  - If `restore_drill_required` is `false`, `restore_drill_result` must be `skipped` and `restore_target` must be `not_run` (and drill completion left empty).

`repository` in the file must match the GitHub repository running **Deploy Production** (enforced in CI).

## What `backup_evidence_id` means in Deploy Production

`backup_evidence_id` must be one of:

1. **A numeric GitHub Actions run id** (digits only) of a **successful** workflow run in this repository that uploaded an artifact named:

   - **`production-db-backup-evidence`**
   - containing **`backup-evidence/backup-evidence.json`** (or a root-level `backup-evidence.json` in that zip).

2. A **repository file** checked out with the workflow, using the form:

   - `path:RELATIVE/PATH/TO/backup-evidence.json`  
   - Example: `path:ops/evidence/2026-04/prod-backup.json`

Other formats (ticket-only, opaque text) are **rejected** for `run_migration=true`.

## How to create backup evidence (operators)

1. **Take a real production backup** using existing automation (e.g. `deployments/prod/scripts/backup_postgres.sh` on the host, which writes a `*.manifest.json` with size and SHA-256).  
2. **Optionally** run `scripts/db/merge_postgres_backup_manifest_to_evidence.py` to start from a manifest — output is always **`evidence_draft: true`** until you complete all fields and remove the draft flag.  
3. **Or** use `scripts/db/emit_backup_evidence_template.py` to print an empty template (also fails validation until completed).  
4. Complete restore-drill and schema-check fields, set **`evidence_draft` to `false`**, and verify locally:
   - `python3 scripts/ci/validate_backup_evidence.py --for-production-migration --expect-repository ORG/REPO your.json`
5. **Upload** the file as a workflow artifact in this repo:
   - artifact name: **`production-db-backup-evidence`**
   - file path in the zip: **`backup-evidence/backup-evidence.json`**

A minimal GitHub Actions job can download nothing public; the evidence run can be a manual workflow you add later that only uploads the JSON.

## How to run a restore drill

- Restore the backup to a **non-production** target (`staging`, `preprod`, or a disposable `temp-db`), run your agreed schema sanity checks, and record:
  - `restore_drill_result: pass`
  - `restore_drill_completed_at_utc`
  - `restore_target` (e.g. `temp-db` or `staging`)
  - `schema_validation_result: pass` when a migration is planned

Nightly or manual restore drill automation (e.g. `nightly-ops.yml`) can inform practices but **production** migration gating uses the JSON file above, not a vague id.

## Where to store evidence

- **Preferred for CI**: upload **`production-db-backup-evidence`** on a successful workflow run; pass that **run id** as `backup_evidence_id`.  
- **Alternate**: commit JSON under a controlled path (e.g. `ops/evidence/…`) and use `path:…` in `backup_evidence_id` (branch must be the one you run **Deploy Production** from, usually `main`).

Never commit secrets, connection strings, or presigned URLs. Use `backup_location_redacted` (category + bucket id style) and artifact ids only.

## How to use it in Deploy Production

1. Open **Actions → Deploy Production** on `main`.  
2. Set **`run_migration`**: `true` only with a vetted backup + drill.  
3. Set **`backup_evidence_id`**: the evidence workflow **run id**, or `path:…` to the JSON in the repo.  
4. Complete other required inputs (build, security, staging evidence, digests, confirmation).  
5. Run the workflow. Validation runs **before** SSH; failures stop the job with a clear `FAIL` from `validate_backup_evidence.py`.

## Interpreting validation output

- **`PASS:`** The JSON satisfies the contract for the flags you passed (`--for-production-migration` is strictest).  
- **`FAIL:`** See the message (missing field, expired evidence, restore drill not `pass` when required, wrong repository, and so on).

## Related

- `docs/contracts/backup-evidence.schema.json`  
- `deployments/prod/scripts/backup_postgres.sh` (on-host logical backup and manifest)  
- `nightly-ops.yml` (optional restore-drill and backup readiness reports)
