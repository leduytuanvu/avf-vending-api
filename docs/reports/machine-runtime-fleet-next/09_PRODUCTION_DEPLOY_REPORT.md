# Production Deploy Report — Runtime Fleet

**UTC:** 20260703T230500Z

## Deploy history

| Run | SHA | Result | Notes |
|-----|-----|--------|-------|
| `28686916171` | `277a3ad4` | SUCCESS | Migrations 00017/00018 applied |
| `28688099702` | `51485f55` | SUCCESS | Timeline SQL hotfix (PR #411); `run_migration=false`; SLO recovery bypass |

## Primary deploy (runtime fleet + migrations)

| Field | Value |
|-------|-------|
| Run ID | `28686916171` |
| URL | https://github.com/leduytuanvu/avf-vending-api/actions/runs/28686916171 |
| First attempt | `28686892063` — **FAILED** at promotion gate (`source_commit_sha` short prefix `277a3ad4` rejected) |
| Successful attempt | `28686916171` with full SHA |

## Inputs used

| Input | Value |
|-------|-------|
| `action_mode` | `deploy` |
| `build_run_id` | `28686709336` |
| `security_release_run_id` | `28686850966` |
| `release_tag` | `runtime-fleet-20260703T225800Z` |
| `source_commit_sha` | `277a3ad4dbe34f204704ed4c3d713ec49bff4ec2` |
| `app_image_ref` | `ghcr.io/leduytuanvu/avf-vending-api@sha256:a29ac35626a45829ee8bc085e8c024a092685723d74cf857f14a1281f446ae4b` |
| `goose_image_ref` | `ghcr.io/leduytuanvu/avf-vending-api-goose@sha256:6d7ea92c549707b607911c23d6295af4226d50dc6c811dd5c0b8757a4ec5375e` |
| `run_migration` | `true` |
| `allow_missing_staging_evidence` | `true` |
| Staging bypass reason | Runtime fleet production deploy; staging evidence bypass per operator approval |

## Deployed artifacts

| Item | Value |
|------|-------|
| SHA | `277a3ad4dbe34f204704ed4c3d713ec49bff4ec2` |
| Previous SHA | `cbdfebeed8d034926b834e5a1dcc32f1309ce5bc` |
| App digest | `sha256:a29ac35626a45829ee8bc085e8c024a092685723d74cf857f14a1281f446ae4b` |
| Goose digest | `sha256:6d7ea92c549707b607911c23d6295af4226d50dc6c811dd5c0b8757a4ec5375e` |

## Migration outcome

| Step | Result |
|------|--------|
| Version before | 16 |
| Version after | 18 |
| `00017_machine_runtime_fleet.sql` | OK (479ms) |
| `00018_machine_runtime_fleet_fixes.sql` | OK (300ms) |
| Inline backup | `backup-20260703T230257Z.dump` — PASS |

## Rollout summary

| Node | Result |
|------|--------|
| app-node A (`72.62.244.94`) | deploy/readiness/smoke **pass** |
| app-node B | skipped (single-node rollout) |
| Final cluster smoke | pass |
| Final public smoke | pass |
| `final_deployment_verdict` | pass |

Evidence artifact: `production-deploy-evidence` from run `28686916171`.
