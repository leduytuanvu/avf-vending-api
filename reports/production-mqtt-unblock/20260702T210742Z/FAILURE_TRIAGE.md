# Failure Triage — MQTT Unblock (pre-deploy)

**UTC:** 20260702T210742Z  
**Verdict:** `BLOCKED_BY_DEPLOY` (code ready; production API not yet provisioned)

## Blocker

Production EMQX per-machine user provisioning requires:

1. Deploy code SHA ≥ `3f021486155c827d48b7dfb181216ae9dec62a5f`
2. App-node env: `EMQX_MANAGEMENT_URL`, `EMQX_API_KEY`, `EMQX_API_SECRET`
3. Data-node ACL enabled (`authorization.enable=true`)

Until deploy completes, claim responses will not include `mqttPassword` and strict bootstrap (`PRODUCTION_FULL_TEST_STRICT=1`) will fail by design.

## Expected post-deploy retest

```powershell
$env:PRODUCTION_FULL_TEST_UTC = (Get-Date -AsUTC -Format "yyyyMMddTHHmmssZ")
$env:PRODUCTION_FULL_TEST_STRICT = "1"
$env:PROD_TEST_ADMIN_EMAIL = "admin@avf.com"
$env:PROD_TEST_ADMIN_PASSWORD = "<session-only>"
python tools/production_full_test/run_production_full_suite.py --base-url https://api.ldtv.dev --passes 3
```

## Fix loop

| Step | Action |
|------|--------|
| 1 | Operator deploy + EMQX env + ACL |
| 2 | Rerun full suite with new UTC |
| 3 | If MQTT ACL negatives fail → verify `acl.conf` on data-node |
| 4 | If claim 503 → verify EMQX management reachability from app container |
| 5 | Loop until `PRODUCTION_REST_GRPC_MQTT_100_PERCENT_PASS` |
