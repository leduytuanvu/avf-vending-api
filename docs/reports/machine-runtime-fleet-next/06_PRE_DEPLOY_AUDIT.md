# Pre-Deploy Audit — Machine Runtime Fleet

**UTC:** 20260703T225800Z  
**Plan phase:** 0 (readonly gate)

## 1. Local branch and working tree

| Check | Result |
|-------|--------|
| Current branch | `feature/machine-runtime-fleet` |
| Working tree | Modified report files only (`02`, `03`, `05_FINAL_VERDICT.json`) — no product code changes |
| Gate | **PASS** — clean enough to proceed (report edits expected during this deploy cycle) |

## 2. `origin/main` SHA

```
277a3ad4dbe34f204704ed4c3d713ec49bff4ec2
```

| Check | Result |
|-------|--------|
| Required minimum | `277a3ad4` (PR #410 merge) |
| Gate | **PASS** — main at target SHA |

## 3. develop ↔ main parity

```
git log origin/main..origin/develop   → (empty)
git diff origin/develop..origin/main  → (empty)
```

| Check | Result |
|-------|--------|
| `origin/develop` SHA | `8991f526d883fd6cfbc996ba5a4affbd558ae02d` |
| `origin/main` SHA | `277a3ad4dbe34f204704ed4c3d713ec49bff4ec2` (merge commit atop develop content) |
| File-tree diff | **empty** |
| Gate | **PASS** |

## 4. Migration files present

| File | Status |
|------|--------|
| `migrations/00017_machine_runtime_fleet.sql` | present |
| `migrations/00018_machine_runtime_fleet_fixes.sql` | present |

Gate: **PASS**

## 5. Secret scan (staged/unstaged diff)

Modified paths are report markdown/JSON only under `docs/reports/machine-runtime-fleet-next/`. No `.env`, tokens, or credentials in diff.

Gate: **PASS**

## 6. Deploy mechanism

Production deploy is **manual only** via `.github/workflows/deploy-prod.yml` `workflow_dispatch` on branch **`main`**. No push/schedule triggers.

Gate: **PASS**

## 7. Rollback path

Documented in `docs/deployment/two-vps-rolling-production-deploy.md`: same workflow with `action_mode=rollback` and digest-pinned `rollback_app_image_ref` / `rollback_goose_image_ref`.

Gate: **PASS** (path documented)

## 8. Inline backup when `run_migration=true`

Per `docs/deployment/PRODUCTION_AUTO_MIGRATION.md`: deploy runs `pg_dump` before goose `up` to:

`/opt/avf-vending-api/deployments/prod/logs/migrations/backup-*.dump`

Gate: **PASS** (mechanism confirmed in docs; evidence captured post-deploy in `08_PRODUCTION_BACKUP.md`)

## 9. Production endpoints

| Surface | Target |
|---------|--------|
| REST | `https://api.ldtv.dev` |
| gRPC | `machine-api.ldtv.dev:443` |
| MQTT | `mqtt.ldtv.dev:8883` |

## 10. Staging evidence bypass (operator-approved)

Deploy will use:

- `allow_missing_staging_evidence=true`
- `missing_staging_evidence_reason="Runtime fleet production deploy; staging evidence bypass per operator approval"`

## Overall gate

**PROCEED** — all Phase 0 readonly checks pass. Next: resolve Build + Security Release inputs (`07_DEPLOY_INPUTS.md`).
