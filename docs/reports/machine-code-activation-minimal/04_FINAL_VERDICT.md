# Machine-code activation — final verdict

Date: 2026-07-06

## Acceptance checklist

| Question | Answer |
|----------|--------|
| Admin can create activation code by `machineCode` path? | **Yes** — `POST /v1/admin/machine-codes/{machineCode}/activation-codes` |
| Admin can list activation codes by `machineCode` path? | **Yes** — `GET /v1/admin/machine-codes/{machineCode}/activation-codes` |
| Admin can revoke by `machineCode` path? | **Yes** — `DELETE /v1/admin/machine-codes/{machineCode}/activation-codes/{activationCodeId}` |
| Legacy UUID machine routes still work? | **Yes** — same handlers; path segment accepts UUID or code |
| Catalog create accepts `machineCode` in body? | **Yes** — `machineCode` / `machine_code`; conflict error when mismatch |
| Create/list responses include `machineCode`? | **Yes** |
| List never exposes plaintext or `codeHash`? | **Yes** — service mapping + HTTP tests assert |
| Runtime claim still UUID-only request? | **Yes** — no request change |
| Claim response may include `machineCode`? | **Yes** — additive when `machines.code` set |
| JWT/MQTT/proto unchanged? | **Yes** — out of scope, not modified |
| Board replacement keeps same machine id + code? | **Yes** — extended integration test (requires DB) |
| OpenAPI documents new routes? | **Yes** — swagger regen + spec test |
| Postman/runbook updated? | **Yes** — e2e + production-full catalog body + env `machineCode` |

## Risks / notes

- Activation resolver uses strict `^AVF[0-9]{6}$`; fleet-wide machine create still allows `^AVF[0-9]{6,}$`. Machines with longer codes cannot be targeted by activation admin code path until code is six digits.
- DB integration tests not executed in this environment (`TEST_DATABASE_URL` unset).

## Recommendation

**Ship** for activation admin minimal scope. Run full integration tests with `TEST_DATABASE_URL` before production deploy.
