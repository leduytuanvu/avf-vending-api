# Production Deploy Result

**UTC:** 20260702T193700Z

## Attempt 1 (failed)

| Field | Value |
|-------|-------|
| Run ID | [28616100078](https://github.com/leduytuanvu/avf-vending-api/actions/runs/28616100078) |
| Conclusion | **failure** |
| Root cause | `pg_dump` failed: Supabase session pooler `max clients reached` (pool_size 15) during pre-migration backup |
| Rollback | Automatic rollback executed; production remained on prior digest |

## Attempt 2 (success)

| Field | Value |
|-------|-------|
| Run ID | [28616475685](https://github.com/leduytuanvu/avf-vending-api/actions/runs/28616475685) |
| Conclusion | **success** |
| Release tag | `v20260702-enterprise-flow-retry1` |
| Build run ID | `28615687682` |
| Security Release run ID | `28615969226` |
| Main SHA | `156fc468fa3c5fec7042e1f656f78b6ea94c2639` |
| App image | `ghcr.io/leduytuanvu/avf-vending-api@sha256:17785fabd82a7cfe8c4e9dcc7d8f2693a03ff186a5294d69a97c7b5e890fddae` |
| Goose image | `ghcr.io/leduytuanvu/avf-vending-api-goose@sha256:ab9abcb8cce2fe7d2136305da4603c5e2ec4c55eaa098aaf26bc11b31587aae6` |
| Migration | `run_migration=true` (00016 enterprise_flow_accountability) |
| Staging gate | Bypass (`allow_missing_staging_evidence=true`) — no staging contract run for SHA 156fc468 |

## Verdict

**DEPLOY_SUCCESS** on retry 2.
