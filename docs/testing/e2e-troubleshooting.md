# E2E troubleshooting (local / staging)

Symptoms → likely causes → first checks. For remediation steps see **[`e2e-remediation-playbook.md`](e2e-remediation-playbook.md)**.

## Local API not ready

- **Symptom:** `curl` to `/health/ready` fails or times out.
- **Likely cause:** Process not started, wrong port, DB migration not applied.
- **Check:** `GET /health/live` then `GET /health/ready`; logs from API stdout; `TEST_DATABASE_URL` if migrations required.

## Missing admin token

- **Symptom:** HTTP **401** `unauthenticated` on `/v1/admin/**`.
- **Likely cause:** `Authorization: Bearer` missing; wrong `auth_type` in Postman; expired JWT.
- **Check:** `POST /v1/auth/login`; env `E2E_ADMIN_TOKEN` / Postman `admin_token`.

## Machine token invalid

- **Symptom:** gRPC **Unauthenticated** or REST machine routes reject token.
- **Likely cause:** Expired JWT, rotation, wrong audience, copied token truncated.
- **Check:** `MachineTokenService.RefreshMachineToken`; admin `rotate-credential` not yet on device.

## Activation code already claimed

- **Symptom:** Claim returns **already bound** / conflict.
- **Likely cause:** Code consumed; wrong org; reused seed file.
- **Check:** Issue new code via admin; use `--fresh-data`.

## gRPC reflection disabled

- **Symptom:** `grpcurl` list fails; unknown service.
- **Likely cause:** Reflection off in `APP_ENV=production`; need local proto path.
- **Check:** Pass `-proto` / `-import-path` to `grpcurl` pointing at `proto/`; see [`machine-grpc.md`](../api/machine-grpc.md) if present.

## Proto path not found

- **Symptom:** `protoc` / `grpcurl` import errors.
- **Likely cause:** Wrong working directory; missing `proto/avf/...` tree.
- **Check:** Run from repo root; include `proto` as import root.

## MQTT broker auth failed

- **Symptom:** Connect refused, **not authorized**, TLS handshake error.
- **Likely cause:** Wrong URL scheme, bad password, ACL mismatch, stale `credential_version`.
- **Check:** `MQTT_BROKER_URL`, TLS flags, broker ACL matrix in [`mqtt-contract.md`](../api/mqtt-contract.md).

## Payment mock not configured

- **Symptom:** `CreatePaymentSession` fails; webhook never arrives.
- **Likely cause:** PSP sandbox keys missing; `payment_env` mismatch (Postman blocks staging+live).
- **Check:** Local env `payment_env`; webhook tunnel (ngrok) for PSP callbacks.

## Idempotency conflict

- **Symptom:** HTTP **409** `illegal_transition` or idempotency mismatch; gRPC **Aborted** / status detail.
- **Likely cause:** Same key, different payload; out-of-order state transition.
- **Check:** Logs for `requestId`; rotate idempotency key **only** when safe per API contract.

## Inventory insufficient

- **Symptom:** Order or vend fails; insufficient stock detail.
- **Likely cause:** Planogram empty; prior vend without restock; wrong slot.
- **Check:** Admin inventory endpoints; refill suggestions per swagger.

## Command timeout

- **Symptom:** Command stuck **pending** past SLA; no MQTT ACK.
- **Likely cause:** Device offline; wrong topic; broker ACL; machine not `active`.
- **Check:** [`mqtt-command-stuck.md`](../runbooks/mqtt-command-stuck.md), `command_ledger` row + `route_key`.

## Offline replay sequence conflict

- **Symptom:** `PushOfflineEvents` rejected; gap / sequence error.
- **Likely cause:** Non-monotonic `offline_sequence`; duplicate `client_event_id` handling differs.
- **Check:** [`machine-grpc.md`](../api/machine-grpc.md), [`local-e2e.md`](local-e2e.md) offline tests.

## Related

- **[`e2e-remediation-playbook.md`](e2e-remediation-playbook.md)**
