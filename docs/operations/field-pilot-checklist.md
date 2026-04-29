# Field pilot checklist (P1 pilot · P2 evidence inputs)

Expand from lab/staging to a **small field pilot** (tens of machines typically) **before** broad fleet rollout (**100–1000** tier evidence is in **[`../runbooks/production-release-readiness.md`](../runbooks/production-release-readiness.md)** storm + monitoring gates). This checklist is **orthogonal** to **[`production-release-checklist.md`](production-release-checklist.md)** (pre-deploy governance) — run both.

**Architecture alignment (normative):** **[`../architecture/production-final-contract.md`](../architecture/production-final-contract.md)** — Admin **`/v1` User JWT / RBAC**; kiosk **`avf.machine.v1`** **Machine JWT** + idempotency + offline queues; **MQTT TLS** commands + Postgres command ledger ACK; **payment webhooks** HMAC + idempotency; **media** via object-storage URLs + local cache keys; infra **PostgreSQL / Redis / NATS·JetStream** as wired per environment (**2-VPS app nodes + managed PostgreSQL / Redis / MQTT / bucket** preferred — see **[`../runbooks/production-2-vps.md`](../runbooks/production-2-vps.md)**; legacy single Compose is rehearsal-only per **[`../runbooks/production-cutover-rollback.md`](../runbooks/production-cutover-rollback.md)**).

**Priority labels**

| Tag | Means |
|-----|--------|
| **P0** | Blocker — do not pilot if failing (safety/data/tenant). |
| **P1** | Required for pilot completion (this checklist). |
| **P2** | Fleet-scale inputs — finalize field matrix rows **[`../testing/field-test-cases.md`](../testing/field-test-cases.md)** and attach **[`production-release-readiness.md`](../runbooks/production-release-readiness.md)** scale tier artifact if claiming > pilot. |

**Evidence rule:** Each row needs **Evidence** (artifact link / workflow run / ticket id / UTC timestamp of pilot window) — **never** Bearer tokens, activation codes, webhook secrets, MQTT passwords, private keys.

---

## Before the pilot window

| Done | Priority | Checkpoint | Evidence link / id | Owner (name · role) |
|------|-----------|------------|---------------------|---------------------|
| ☐ | P0 | Release commit **`SHA`**, **`verify-enterprise-release`** equivalent green (or waived per org with ticket), migrations plan documented | | |
| ☐ | P0 | **Production ≠ staging DB** DNS / env verified (see **`production-release-readiness`** § Databases). | | |
| ☐ | P1 | **`/health/live`**, **`/health/ready`**, **`/version`**, OpenAPI **`/swagger/doc.json`** (if mounted) reachable on **pilot cluster** | | |
| ☐ | P1 | Dependencies match profile: **PostgreSQL**, **Redis**, **NATS/JetStream**, **MQTT broker TLS**, **object storage**, webhook HMAC secrets — no `tcp://` MQTT in staging/prod | | |
| ☐ | P1 | Explicit **Pilot owner** + **Rollback owner** + **Timezone / maintenance window** in change ticket | | |
| ☐ | P1 | Feature flags recorded: **`MACHINE_GRPC_ENABLED`**, MQTT command dispatch enabled, webhook secrets present, **`ENABLE_LEGACY_MACHINE_HTTP=false`** for production pilot **or** explicit migration waiver ticket | | |
| ☐ | P1 | Smoke strategy: **`production-smoke-tests.md`** read-only tiers for CI/CD; **`../runbooks/field-smoke-tests.md`** for operator smoke; **`field-test-cases.md`** for mutating proofs | | |
| ☐ | P1 | Acknowledge **`production-smoke`** does **NOT** perform capture, vend, inventory mutate, MQTT publish — field mutating paths use staging/pilot only | | |

---

## During pilot (observation window)

| Done | Priority | Checkpoint | Evidence link / id | Owner |
|------|-----------|------------|---------------------|-------|
| ☐ | P1 | Read-only **`scripts/deploy/smoke_test.sh`** or **`smoke_prod.sh`** path executed post-deploy (**JSON** archived) — see **`field-smoke-tests.md`** | | |
| ☐ | P1 | **`local_field_smoke`** (staging/pilot sandbox) executed **once** minimum — **`field-smoke-tests.md`**; redacted log | | |
| ☐ | P1 | **Machine JWT** exercised on **`GetBootstrap`** + **`GetCatalogSnapshot`** (record **methods**, not tokens) | | **`field-test-cases`** `FT-CAT-01` |
| ☐ | P1 | **Admin RBAC / media** smoke rows **`FT-ADM-01`**, **`FT-ADM-03`**, **`FT-MED-02`** progressing (`FT-PAY-04` mid-payment, `FT-OFF-01` replay if offline in scope) | | |
| ☐ | P1 | Optional: **MQTT ACK** rehearsal per **`mqtt-command-debug.md`** (**lab/staging** simulator) — **`FT-MQT-01`** | | |
| ☐ | P1 | Log/metric watch includes: request IDs, webhook **HMAC failures**, MQTT publish latency, **`avf_worker_outbox_*`**, **`avf_grpc_requests_handled_total`**, command ACK timeouts | | |
| ☐ | P1 | Grafana/Prometheus dashboards (if deployed): HTTP/gRPC 5xx, machine offline gauges, webhook result label, Redis errors, NATS backlog | | |

---

## After pilot window (gate)

| Done | Priority | Checkpoint | Evidence link / id | Owner |
|------|-----------|------------|---------------------|-------|
| ☐ | P0 | **No open P0** production incidents unexplained | | Incident mgmt owner |
| ☐ | P1 | No **duplicate financial** side-effects from webhook or idempotency bug (reconciliation spreadsheet or PSP report) | | |
| ☐ | P1 | **`field-test-cases`** matrix pilot rows filled or explicit **waived** with risk acceptance | | Field lead |
| ☐ | P1 | **`backup`/restore drill** reference current if pilots touched migrations — **`FT-BKP-01`** ([**`production-backup-restore-dr.md`](../runbooks/production-backup-restore-dr.md)**) | | DBA/SRE |
| ☐ | P2 | Decision recorded: **Expand** scaling tier OR **stay pilot** OR **freeze** — if expanding to scale-100+, follow **`production-release-readiness`** storm tier | | Eng manager |

---

## Exit criteria summary

Pilot **pass** only when: **P0** clear, **`field-test-cases`** material complete for **`FT-*` rows attempted**, evidence attachments in ticket, field lead signs **Expand / Hold / Freeze**.

---

## Related (no contradictions)

| Document | Role |
|---------|------|
| [`production-release-checklist.md`](production-release-checklist.md) | Pre-**Deploy Production** checkbox |
| [`field-rollout-checklist.md`](field-rollout-checklist.md) | Post-deploy **machine / operator** widening |
| [`production-smoke-tests.md`](production-smoke-tests.md) | Automated **GET-only** smoke tiers |
| [`../runbooks/field-smoke-tests.md`](../runbooks/field-smoke-tests.md) | Operator smoke commands |
| [`../testing/field-test-cases.md`](../testing/field-test-cases.md) | **Paste-friendly** case matrix (FT-*) |
| [`production-final-contract.md`](../architecture/production-final-contract.md) | Normative transport contract (pilot → fleet) |
