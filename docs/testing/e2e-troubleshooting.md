# E2E troubleshooting (local / staging)

Symptoms → likely causes → first checks. For remediation steps see **[`e2e-remediation-playbook.md`](e2e-remediation-playbook.md)**.

## E2E harness: missing jq, python3, or curl

- **Symptom:** `FATAL: required command not found: jq` (or `python3` / `curl`) when running `run-all-local.sh`, `run-rest-local.sh`, or preflight.
- **Likely cause:** Host PATH does not include dev tools (common on minimal Windows shells).
- **Check:** Install **jq** and **Python 3**; use **Git Bash** or WSL so `curl` behaves like GNU curl. Re-run with `command -v jq python3 curl`.

## E2E harness: HTTP 000 or connection refused on health checks

- **Symptom:** `rest/*/meta.json` shows `httpStatus` **0** or non-**200** on required GETs; preflight or `--readonly` smoke fails.
- **Likely cause:** API not listening on `BASE_URL`, wrong port, or firewall.
- **Check:** `curl -sS -o /dev/null -w '%{http_code}' "$BASE_URL/health/live"` from the same shell; start the API and fix `BASE_URL`.

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

## gRPC machine tests (Phase 6): server unreachable

- **Symptom:** `run-grpc-local.sh` fails immediately with **gRPC unreachable at `${GRPC_ADDR}`**; empty **`grpc-contract-results.jsonl`** or only partial rows.
- **Likely cause:** API process has no gRPC listener on that host/port; TLS mismatch (harness uses **`-plaintext`**); firewall; wrong `GRPC_ADDR`.
- **Check:** Confirm gRPC is enabled in local config; `GRPC_ADDR` matches server bind; if the server uses TLS, grpcurl needs TLS flags (adjust harness or use a plaintext dev port). With **reflection**, `grpcurl -plaintext "${GRPC_ADDR}" list` must succeed; without reflection, **TCP** to `${GRPC_ADDR}` must accept connections. See **`tests/e2e/run-grpc-local.sh -h`** and **`GRPC_PROTO_ROOT`** (defaults to repo **`proto/`**).

## gRPC reflection disabled

- **Symptom:** `grpcurl` list fails; unknown service.
- **Likely cause:** Reflection off in `APP_ENV=production`; need local proto path.
- **Check:** Pass `-proto` / `-import-path` to `grpcurl` pointing at `proto/`; see [`machine-grpc.md`](../api/machine-grpc.md) if present.

## Proto path not found

- **Symptom:** `protoc` / `grpcurl` import errors.
- **Likely cause:** Wrong working directory; missing `proto/avf/...` tree.
- **Check:** Run from repo root; include `proto` as import root.

## MQTT Phase 7: broker unreachable

- **Symptom:** **`run-mqtt-local.sh`** exits early with TCP connect failure; **`30_mqtt_connect.sh`** fails; no **`mqtt/connect.log`** success lines.
- **Likely cause:** Broker not running; wrong **`MQTT_HOST`** / **`MQTT_PORT`**; firewall; Docker network not exposed to the host.
- **Check:** From the same shell, reach **`MQTT_HOST:MQTT_PORT`** (default **1883**) with a TCP client. Start **mosquitto** (or your stack) and re-run. Install **mosquitto** clients if **`run-mqtt-local.sh`** skipped with “mosquitto missing”.

## MQTT broker auth failed

- **Symptom:** Connect refused, **not authorized**, TLS handshake error.
- **Likely cause:** Wrong credentials, ACL mismatch, TLS disabled in harness while broker requires TLS (or the opposite), stale `credential_version`.
- **Check:** **`MQTT_USERNAME`**, **`MQTT_PASSWORD`**, **`MQTT_USE_TLS`**, **`MQTT_CA_CERT`**, and broker ACL matrix in [`mqtt-contract.md`](../api/mqtt-contract.md).

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
