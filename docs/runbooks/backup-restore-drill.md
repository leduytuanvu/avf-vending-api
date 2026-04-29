# Backup restore drill (non-production)

This runbook covers **verifying** production backup **evidence** and recording a **non-production** restore drill outcome using the **Restore drill (non-production backup evidence)** workflow.

**Default policy:** restore drills use **staging**, **preprod**, or a **disposable** database (`temp_db` / `temp-db` in evidence JSON). **This workflow does not attach `environment: production` and is not a production database restore path.**

## Workflow

- File: `.github/workflows/restore-drill.yml` — **Restore drill (non-production backup evidence)**.
- Trigger: `workflow_dispatch` only (not automatic; CI enforces no `on.workflow_run` / `on.push`).
- Artifact: **`restore-drill-workflow-evidence`** (JSON under `restore-drill-evidence/`).

## Inputs

| Input | Description |
| --- | --- |
| `backup_evidence_id` | Same format as **Deploy Production** backup evidence: **numeric run id** of a **successful** workflow that uploaded **`production-db-backup-evidence`**, or **`path:…/backup-evidence.json`** in the checked-out ref. |
| `restore_target` | One of `staging`, `preprod`, `temp_db` (choice in the form). This must **match** the `restore_target` field in the backup JSON when that field is already set to a concrete value (other than `not_run`). |
| `dry_run` | Default `true`. When `true`, the workflow still validates the backup JSON and records `workflow_outcome: skipped` in the evidence (validation-only; no on-runner restore). When `false`, evidence records `pass` for successful validation/alignment (physical restore is still on the **host** or DB VM per your process). |
| `notes` | Optional single line for the ticket. |

## Validator

- `python3 scripts/ci/validate_backup_evidence.py` (not `--for-production-migration` here; this is a drill, not a migration gate).
- For production **migration** gating, see [../operations/production-backup-restore-drill.md](../operations/production-backup-restore-drill.md).

## Pass / fail / skipped

- **Validation failure** (malformed or expired JSON): workflow **fails**; no success artifact. Fix the evidence file or pick another run.
- **dry_run true:** `workflow_outcome: skipped` in `restore-drill-workflow.json` (honest “no live restore in CI”).
- **dry_run false:** `workflow_outcome: pass` when validation and `restore_target` alignment succeed.

## Relating to production migrations

- Production `run_migration` in **Deploy Production** still requires the stricter [backup evidence contract](../contracts/backup-evidence.schema.json) and `--for-production-migration` where documented.
- This drill workflow **does not** replace that gate; it records **operational** drill discipline for operators and compliance.

## Helper script

- Evidence emission: `scripts/db/emit_restore_drill_workflow_evidence.py` (used by the workflow; not a restore executor).

## Copy-paste verification

Git Bash:

```bash
python scripts/ci/validate_backup_evidence.py path:backup-evidence.json
```

PowerShell:

```powershell
python scripts/ci/validate_backup_evidence.py path:backup-evidence.json
```

For a real restore drill, record:

- source backup evidence ID or path
- restore target (`staging`, `preprod`, or disposable DB)
- expected and observed RPO/RTO
- `APP_REGION`, `APP_NODE_NAME`, and `APP_INSTANCE_ID` used by the restored app process
- migration/schema version after restore
- `go run ./cmd/cli -validate-config` result for the restored environment
- read-only smoke result from `docs/runbooks/field-smoke-tests.md`
- Redis state posture (`empty`, `restored`, or `recreated`) and confirmation that Redis was not treated as authoritative
- NATS/JetStream posture (`empty`, `recreated`, or `restored`) and whether replay was intentionally tested
- object storage bucket/versioning validation for at least one media/artifact object when artifacts are enabled

Never run a production restore from this workflow. Production restore remains an operator-controlled incident procedure.

Use [multi-region-dr-readiness.md](multi-region-dr-readiness.md) for the complete DR validation checklist.
