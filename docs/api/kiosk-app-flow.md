# Kiosk app end-to-end flow (numbered)

**Audience:** Android / kiosk engineers and integration QA. Pair with [kiosk-app-implementation-checklist.md](kiosk-app-implementation-checklist.md), [mqtt-contract.md](mqtt-contract.md), and [api-surface-audit.md](api-surface-audit.md).

**Repo static gate:** `make verify-enterprise-release` (see [production-release-readiness.md](../runbooks/production-release-readiness.md)).

**Telemetry:** High-volume events and **primary command delivery** use **MQTT** (JetStream, idempotency in payload). HTTP under `/v1/device/...` is **fallback** (e.g. `POST .../commands/poll` is not the primary command path).

**Examples:** OpenAPI `docs/swagger/swagger.json` (regenerate: `make swagger`). Contract samples: `testdata/telemetry/*.json`, [device-offline-replay-samples.md](examples/device-offline-replay-samples.md).

**Operator / release arc (pilot → scale):** first install → activation → sale catalog cache → cash sale → online (PSP) sale → vend-failure refund path → telemetry reconcile for critical MQTT → **field** cash settlement (admin cashbox / collections; ledger vs physical count) → **staging storm evidence** (100×100 / 500×200 / 1000×500) before claiming matching fleet-scale readiness ([production-release-readiness.md](../runbooks/production-release-readiness.md)).

---

## Conventions (all steps)

| Field | Meaning |
| --- | --- |
| **Auth** | `Bearer` = `Authorization: Bearer <machine or org JWT>` unless noted. |
| **Request ref** | OpenAPI `example` on the operation, or linked JSON under `testdata/`. |
| **Idempotency** | Use `Idempotency-Key` on mutating routes that require it (see OpenAPI). |
| **Retry / offline** | Queue writes when offline; retry with backoff; reuse keys for logical writes. |

---

### 1. First install

| Field | Value |
| --- | --- |
| Endpoint | _(none — client-side)_ |
| Method | — |
| Auth | — |
| Request ref | OEM / MDM install flow |
| Response ref | — |
| Idempotency | n/a |
| Retry / offline | n/a |

Install APK, permissions, secure storage provisioning. No HTTP until activation.

---

### 2. Activation code claim

| Field | Value |
| --- | --- |
| Endpoint | `/v1/setup/activation-codes/claim` |
| Method | `POST` |
| Auth | none (public) |
| Request ref | OpenAPI example (activationCode, deviceFingerprint) |
| Response ref | OpenAPI 200 (machineToken, bootstrap hints) |
| Idempotency | not via header; treat as one-shot provisioning |
| Retry / online | Invalid codes return `400` without leaking tenant existence; do not brute-force |

Admin pre-step: `POST /v1/admin/machines/{machineId}/activation-codes` (Bearer; not kiosk).

---

### 3. Store machine token

| Field | Value |
| --- | --- |
| Endpoint | _(client storage)_ |
| Method | — |
| Auth | — |
| Request ref | — |
| Response ref | — |
| Idempotency | n/a |
| Retry / offline | Keystore / EncryptedSharedPreferences; never log raw tokens |

---

### 4. Bootstrap setup

| Field | Value |
| --- | --- |
| Endpoint | `/v1/setup/machines/{machineId}/bootstrap` |
| Method | `GET` |
| Auth | Bearer (machine tenant) |
| Request ref | OpenAPI parameters |
| Response ref | OpenAPI 200 (topology + catalog snapshot) |
| Idempotency | n/a (GET) |
| Retry / offline | Safe to retry; cache for offline UI |

---

### 5. Runtime sale catalog sync

| Field | Value |
| --- | --- |
| Endpoint | `/v1/machines/{machineId}/sale-catalog` |
| Method | `GET` |
| Auth | Bearer (machine tenant) |
| Request ref | OpenAPI (`include_images`, `if_none_match_config_version`, …) |
| Response ref | OpenAPI 200 / **304** |
| Idempotency | n/a |
| Retry / offline | Use `configVersion` / ETag-style headers; fall back to bootstrap snapshot offline |

---

### 6. Image download / cache (contentHash)

| Field | Value |
| --- | --- |
| Endpoint | _(CDN / image URLs from sale-catalog or bootstrap)_ |
| Method | `GET` (HTTP/S) |
| Auth | As returned by API (often signed URL or public CDN path) |
| Request ref | Catalog product image descriptors + `contentHash` |
| Response ref | Binary image |
| Idempotency | n/a |
| Retry / offline | Dedupe on `contentHash`; disk cache; backoff |

---

### 7. Technician operator session

| Field | Value |
| --- | --- |
| Endpoints | `POST /v1/auth/login`, `POST /v1/auth/refresh`, `GET /v1/auth/me`, `POST /v1/auth/logout` (portal); `POST /v1/machines/{machineId}/operator-sessions/login`, `POST .../logout`, `POST .../{sessionId}/heartbeat` |
| Method | varies |
| Auth | Bearer (technician); machine-scoped session routes |
| Request ref | OpenAPI for each |
| Response ref | OpenAPI |
| Idempotency | n/a for most; follow session contract for logout |
| Retry / offline | Heartbeats queue if needed; do not start inventory writes without ACTIVE session |

---

### 8. Topology / planogram setup

| Field | Value |
| --- | --- |
| Endpoints | `PUT /v1/admin/machines/{machineId}/topology`, `PUT .../planograms/draft`, `POST .../planograms/publish`, optional `POST .../sync` |
| Method | `PUT` / `POST` |
| Auth | Bearer (org/platform admin) |
| Request ref | OpenAPI |
| Response ref | OpenAPI |
| Idempotency | **Required** on `planograms/publish` and `sync` (`Idempotency-Key`) |
| Retry / offline | Draft/publish are online operations; kiosk picks up via MQTT command or poll fallback |

---

### 9. Stock adjustment

| Field | Value |
| --- | --- |
| Endpoint | `POST /v1/admin/machines/{machineId}/stock-adjustments` |
| Method | `POST` |
| Auth | Bearer + operator session |
| Request ref | OpenAPI |
| Response ref | OpenAPI (`replay` when replayed) |
| Idempotency | **Required** |
| Retry / offline | Same key on retry |

---

### 10. Cash sale

| Field | Value |
| --- | --- |
| Endpoints | `POST /v1/commerce/cash-checkout` (and/or `POST /v1/commerce/orders` flow) |
| Method | `POST` |
| Auth | Bearer (org-scoped) |
| Request ref | OpenAPI |
| Response ref | OpenAPI |
| Idempotency | **Required** |
| Retry / offline | Queue with stable keys; server resolves totals where applicable |

---

### 11. Online QR / wallet sale

| Field | Value |
| --- | --- |
| Endpoints | `POST /v1/commerce/orders`, `POST .../payment-session`, `GET .../reconciliation` or `GET .../order` |
| Method | `POST` / `GET` |
| Auth | Bearer; webhook is server-side |
| Request ref | OpenAPI |
| Response ref | OpenAPI |
| Idempotency | **Required** on writes |
| Retry / offline | Poll payment state; PSP calls `.../webhooks` with HMAC |

---

### 12. Vend start / success / failure

| Field | Value |
| --- | --- |
| Endpoints | `POST .../vend/start`, `.../vend/success`, `.../vend/failure`; optional `POST /v1/device/machines/{machineId}/vend-results` |
| Method | `POST` |
| Auth | Bearer |
| Request ref | OpenAPI; `testdata/telemetry/valid_vend_success.json`, `valid_vend_failed.json` |
| Response ref | OpenAPI (failure may include refund hints) |
| Idempotency | **Required** on commerce vend routes |
| Retry / offline | Device bridge route is **fallback**; prefer in-band commerce outcomes when online |

---

### 13. Refund after paid vend failure

| Field | Value |
| --- | --- |
| Endpoints | `POST .../refunds`, `GET .../refunds`, `GET .../refunds/{refundId}`; `POST .../cancel` (pre-capture) |
| Method | `POST` / `GET` |
| Auth | Bearer |
| Request ref | OpenAPI |
| Response ref | OpenAPI (cash vs PSP semantics) |
| Idempotency | **Required** on `refunds` and `cancel` |
| Retry / offline | Queue refund/cancel; distinguish local cash handling vs PSP |

---

### 14. Offline telemetry outbox

| Field | Value |
| --- | --- |
| Endpoint | **MQTT** publish (primary); optional HTTP batch `POST /v1/device/machines/{machineId}/events/reconcile` for critical ACK |
| Method | MQTT / `POST` |
| Auth | Machine credentials from claim + TLS |
| Request ref | [mqtt-contract.md](mqtt-contract.md), `testdata/telemetry/*.json` |
| Response ref | MQTT ACK; HTTP reconcile response envelope |
| Idempotency | Payload idempotency keys per contract (not always `Idempotency-Key` header) |
| Retry / offline | Buffer with caps/jitter; avoid storms on reconnect |

---

### 15. Critical event reconcile

| Field | Value |
| --- | --- |
| Endpoints | `POST .../events/reconcile`, `GET .../events/{idempotencyKey}/status` |
| Method | `POST` / `GET` |
| Auth | Bearer |
| Request ref | OpenAPI; `testdata/telemetry/duplicate_replay_vend.json` |
| Response ref | OpenAPI (processed / failed_retryable / …) |
| Idempotency | Batch keys inside body |
| Retry / offline | Drive outbox from status responses |

---

### 16. Cash collection / settlement

| Field | Value |
| --- | --- |
| Endpoints | `GET .../cashbox`, `POST .../cash-collections`, `GET .../cash-collections`, `POST .../close` |
| Method | `GET` / `POST` |
| Auth | Bearer (admin / field) |
| Request ref | OpenAPI |
| Response ref | OpenAPI (`expectedCloudCashMinor` / cloud ledger vs `countedPhysicalCashMinor` / operator count; `varianceMinor`; `reviewState` when closed) |
| Idempotency | **Required** on mutating collection routes |
| Retry / offline | Field workflows; not kiosk runtime sale path |

---

### 17. Normal startup

| Field | Value |
| --- | --- |
| Endpoints | `GET /health/live`, `GET /health/ready`, `GET /version` (ops); then `GET .../bootstrap` or `GET .../sale-catalog`, `GET .../shadow` |
| Method | `GET` |
| Auth | none for health; Bearer for `/v1` |
| Request ref | OpenAPI |
| Response ref | OpenAPI |
| Idempotency | n/a |
| Retry / offline | Exponential backoff to API; use cached catalog |

---

### 18. Offline mode

| Field | Value |
| --- | --- |
| Endpoint | _(client behavior)_ |
| Method | — |
| Auth | — |
| Request ref | Cached catalog, shadow, local ledger |
| Response ref | — |
| Idempotency | Queue keyed writes |
| Retry / offline | Read-only UX; block paid flows that require PSP unless policy allows |

---

### 19. Reconnect storm behavior

| Field | Value |
| --- | --- |
| Endpoint | MQTT reconnect + optional `POST .../events/reconcile` |
| Method | — |
| Auth | — |
| Request ref | [telemetry-jetstream-resilience.md](../runbooks/telemetry-jetstream-resilience.md), [mqtt-contract.md](mqtt-contract.md) |
| Response ref | — |
| Idempotency | Strict payload keys |
| Retry / offline | Jittered flush; respect rate limits; **do not** stampede `commands/poll` as primary transport |

---

## Related

- [API surface audit](api-surface-audit.md)
- [Machine runtime](machine-runtime.md)
- [Production release readiness](../runbooks/production-release-readiness.md)
