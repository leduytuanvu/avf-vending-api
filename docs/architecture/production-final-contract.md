# Production final contract (normative)

This document is the **binding integration contract** for AVF vending **production**. It is the single normative summary for **Android**, **backend**, **QA field**, **operations/on-call**, and **pilot → fleet rollout (10 → 100 → 1000 machines)**. Detailed RPC fields, JSON examples, and wire formats live in linked docs and protos; this file states **non‑negotiable boundaries** only.

**Related:** [`transport-boundary.md`](transport-boundary.md) (full rationale), [`deployment-secrets.md`](../operations/deployment-secrets.md), [`deployment-secrets-contract.yml`](../contracts/deployment-secrets-contract.yml), field evidence [`../testing/field-test-cases.md`](../testing/field-test-cases.md), pilot [`../operations/field-pilot-checklist.md`](../operations/field-pilot-checklist.md), scale [`../runbooks/production-release-readiness.md`](../runbooks/production-release-readiness.md).

---

## 1. Final production contract (checklist)

| # | Rule | Verify |
|---|------|--------|
| 1 | **Admin REST only** for operator/admin/technician **HTTP** surfaces: `/v1/...` with **User JWT** + RBAC. | Admin calls use REST; [`machine-grpc.md`](../api/machine-grpc.md) documents Admin JWT is **not** accepted on machine gRPC. |
| 2 | **Machine gRPC only** for kiosk/vending runtime: **`avf.machine.v1`** with **Machine JWT** (metadata `authorization: Bearer …`), **idempotency ledger** on listed mutations. | Field rows **FT-MAC-01**, **FT-CAT-01**; Android checklist §2–3 in [`kiosk-app-implementation-checklist.md`](../api/kiosk-app-implementation-checklist.md). |
| 3 | **Backend → machine MQTT only** for command **delivery** (TLS); HTTP command poll is **not** the primary path. | **FT-MQT-01**, **FT-MQT-02**; [`mqtt-contract.md`](../api/mqtt-contract.md). |
| 4 | **Payment: backend-owned PSP sessions** for card/QR: kiosk calls **`CreatePaymentSession`**; **display/QR URL** comes **only** from server response; PSP → AVF via **HMAC webhooks**. | **FT-PAY-01**–**03**, **FT-PAY-05**; [`machine-grpc.md`](../api/machine-grpc.md) (backend-owned sessions). |
| 5 | **Media: object storage + app local cache**; **no** binary product media on gRPC. URLs + **`checksum_sha256` / `etag` / `expires_at`** (per catalog/manifest) drive **durable file cache** and **hash verification**. | **FT-MED-01**, **FT-MED-02**; [`media-sync.md`](media-sync.md). |
| 6 | **Legacy machine HTTP disabled** in production (`ENABLE_LEGACY_MACHINE_HTTP=false` default; production config requires explicit allow to enable). Kiosk must not rely on `/v1/machines/{id}/sale-catalog` or `/v1/commerce/...` **on the device** in prod. | Config + [`machine-grpc.md`](../api/machine-grpc.md) (HTTP → gRPC migration). |

---

## 2. Contract matrix (planes)

| Plane | Transport & identity | Durability & safety |
| --- | --- | --- |
| **Admin web** | REST `/v1` + OpenAPI; **user JWT** + RBAC | Audit for sensitive admin mutations |
| **Vending app** | **`avf.machine.v1` gRPC** + **machine JWT** | **Idempotency ledger** + offline batch semantics per [`machine-grpc.md`](../api/machine-grpc.md) |
| **Legacy vending HTTP** | **Disabled in production by default** (`ENABLE_LEGACY_MACHINE_HTTP=false`); optional only with documented override | Must not be the primary integration in production; see [`transport-boundary.md`](transport-boundary.md) |
| **Backend → machine** | **MQTT over TLS**; command topics per [`mqtt-contract.md`](../api/mqtt-contract.md); EMQX ACL per deployment examples | **Command ledger** in PostgreSQL; **application ACK** correlated by `(command_id, machine_id, sequence)`; duplicate **`dedupe_key`** idempotent; wrong-machine ACK rejected with audit + metrics |
| **Payment (provider → AVF)** | HTTPS **REST webhook** | **AVF HMAC**, timestamp/replay window, **idempotent** provider event keys |
| **Payment (AVF → customer/PSP)** | Backend **owns** checkout/session creation and amounts shown for card/APM flows | Session creation **server-side**; clients **must not** construct PSP QR/pay URLs from unverified input |
| **Media** | **Object storage** + **HTTPS** URLs in APIs; **no binary media on gRPC** | Kiosk **local cache** keyed per catalog/media epoch ([`media-sync.md`](media-sync.md), runbooks) |
| **Platform** | Modular monolith **processes** | **PostgreSQL** (SoR), **Redis**, **NATS/JetStream** for telemetry/outbox fan-out; **outbox row** is SoR until external consumers ack |

**Canonical machine API surface:** `proto/avf/machine/v1/machine_runtime.proto` and companions in the same directory.

**Secrets and deployment safety:** [`deployment-secrets-contract.yml`](../contracts/deployment-secrets-contract.yml), [`deployment-secrets.md`](../operations/deployment-secrets.md).

---

## 3. Rollout tiers (10 → 100 → 1000)

| Phase | Machines (typical) | Documentation pack |
| ----- | ------------------ | -------------------- |
| **Pilot** | ~10 | Complete **P0/P1** rows in [`field-test-cases.md`](../testing/field-test-cases.md); [`field-pilot-checklist.md`](../operations/field-pilot-checklist.md) |
| **Widen** | 10–100 | [`field-rollout-checklist.md`](../operations/field-rollout-checklist.md); per-tranche MQTT + payment + catalog evidence |
| **Fleet** | 100–1000 | [`production-release-readiness.md`](../runbooks/production-release-readiness.md) storm JSON + monitoring tiers; expanded **FT-*** matrix for the release id |

Automated DB correctness suites ([`local-e2e.md`](../testing/local-e2e.md)) **do not** replace field rows (hardware, broker TLS, real PSP, operator sign-off).
