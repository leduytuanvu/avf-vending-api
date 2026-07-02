# Deploy Result — MQTT Unblock

**UTC:** 20260702T210742Z  
**Status:** `PENDING_OPERATOR_DEPLOY`  
**Code SHA (local):** `3f021486155c827d48b7dfb181216ae9dec62a5f`

## Implemented in this change set

- `emqxadmin.UpsertUser` / `DeleteUser` with unit tests
- `machine_mqtt_credentials` sqlc metadata (no plaintext password in DB)
- Activation claim fail-closed + provision before commit
- Reattach MQTT rotation + REST mqtt fields
- Fleet compromised/revoke/rotate EMQX hooks
- gRPC `ClaimActivationResponse.mqtt_username` / `mqtt_password`
- Production harness: auth/ACL negatives, strict bootstrap, E2E flows A–I

## Operator deploy checklist

1. Merge to `main` through CI + security gates.
2. On **both app-node VPSes**, add to `.env.app-node`:
   - `EMQX_MANAGEMENT_URL=http://187.127.99.153:18083`
   - `EMQX_API_KEY` / `EMQX_API_SECRET` (from secrets manager — not committed)
3. From app container, verify: `curl -sf -u "$EMQX_API_KEY:$EMQX_API_SECRET" "$EMQX_MANAGEMENT_URL/api/v5/status"`
4. On **data-node**, enable file ACL (`deployments/prod/emqx/acl.conf.example`, `authorization.enable=true`), restart EMQX.
5. Trigger **Deploy Production** workflow (`.github/workflows/deploy-prod.yml`).
6. Post-deploy: `/health/live`, `/version`, claim smoke must return `mqttUsername` + `mqttPassword`.

## Rollback

Use deploy workflow rollback mode with prior manifest (`docs/runbooks/production-2-vps.md`).

## Retest command (after deploy)

```powershell
$env:PRODUCTION_FULL_TEST_UTC = (Get-Date -AsUTC -Format "yyyyMMddTHHmmssZ")
$env:PRODUCTION_FULL_TEST_STRICT = "1"
python tools/production_full_test/run_production_full_suite.py --base-url https://api.ldtv.dev --passes 3
```
