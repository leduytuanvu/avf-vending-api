# Field rollout checklist (machines · P1 widen · P2 fleet)

Apply **after** a production API deploy coordinates with **edge/kiosk firmware** where relevant. Does **not** replace **[`production-release-checklist.md`](production-release-checklist.md)** — that governs **`workflow_dispatch`** to production; **this** checklist governs **field validation** widening toward **100–1000** machines.

**Normative contract:** **[`../architecture/production-final-contract.md`](../architecture/production-final-contract.md)** (Admin REST · machine gRPC · MQTT · payments · media).

**Infrastructure:** Default production story is **2 × app VPS** (+ optional data node / managed MQTT) per **[`../runbooks/production-2-vps.md`](../runbooks/production-2-vps.md)** — **managed PostgreSQL, Redis, object storage**, and either **managed MQTT** or **data-node** EMQX; **contradicts** documenting single-host Compose as primary. Rolling traffic + smoke: **[`two-vps-rolling-production-deploy.md`](two-vps-rolling-production-deploy.md)**.

**Secrets:** No MQTT passwords / API keys / PSP secrets / SSH keys in tickets — reference vault paths; logs **redacted**.

**Priority**

| Tier | Applies here |
|------|----------------|
| **P1** | Every widen tranche |
| **P2** | Additional when claiming **scale-100 / 500 / 1000** — storm JSON + **`production-release-readiness.md`** gates |

Every section below: record **Evidence owner**, **Evidence link/id**, **Date (UTC)**.

---

## Preconditions (P1)

| Done | Checkpoint | Evidence | Owner |
|------|------------|---------|-------|
| ☐ | Representative machines **online** pre-change; JWT / cert policy understood | | |
| ☐ | **Maintenance window**, **communication plan**, **rollback owner** documented — digest rollback **`!=`** DB downgrade — **[`../runbooks/production-cutover-rollback.md`](../runbooks/production-cutover-rollback.md)** | | |

---

## Product and catalog (P1)

| Done | Checkpoint | Evidence | Owner |
|------|------------|---------|-------|
| ☐ | **Catalog / pricebook** projection matches planogram expectation after sync (**`MachineCatalogService/GetCatalogSnapshot`** on gRPC; HTTP sale-catalog only if legacy enabled) — **[`../api/kiosk-app-flow.md`](../api/kiosk-app-flow.md)** | **`FT-CAT-01`** row | |
| ☐ | **Inventory alignment** vs operator dashboard after test cycle (**reconciler** read or manual audit per ops policy) | | |

---

## Payments (P1)

| Done | Checkpoint | Evidence | Owner |
|------|------------|---------|-------|
| ☐ | **QR / PSP** — small-value **approved** sandbox or live test completes; PSP dashboard matches server order (**`FT-PAY-01`**). Webhook replay idempotent (**HMAC**) — **[`../runbooks/payment-webhook-debug.md`](../runbooks/payment-webhook-debug.md)** | | |
| ☐ | **Cash** — `FT-PAY-02` lane if treasury policy mandates MDB/hardware proof (may be lab-only subset) | | |

---

## Dispense / commerce (P1)

| Done | Checkpoint | Evidence | Owner |
|------|------------|---------|-------|
| ☐ | **Vend success** — `FT-VND-01` on **each critical model family** sampled | | |
| ☐ | **Vend failure + refund** — `FT-PAY-03` (paid + vend fail; refund/ticket path); success replay is `FT-VND-01` | | |
| ☐ | **Power loss mid-vend** — `FT-VND-03` on at least one pilot device | | |

---

## Connectivity and offline (P1)

| Done | Checkpoint | Evidence | Owner |
|------|------------|---------|-------|
| ☐ | **MQTT reconnect** transient flap — **[`../api/mqtt-contract.md`](../api/mqtt-contract.md)** | | |
| ☐ | **Mid-payment network loss** — `FT-PAY-04` | | |
| ☐ | **Offline replay** — `FT-OFF-01` — duplicate client event id **REPLAYED** | | |
| ☐ | **Telemetry** — `FT-TEL-01` | | |

---

## Commands (P1)

| Done | Checkpoint | Evidence | Owner |
|------|------------|---------|-------|
| ☐ | **Duplicate command ACK** — `FT-MQT-02` | | |

---

## Money movement and exceptions (P1)

| Done | Checkpoint | Evidence | Owner |
|------|------------|---------|-------|
| ☐ | **Payment reconciliation** — **`FT-PAY-05`**: server totals vs PSP settlement for pilot sample | | Treasury / backend |
| ☐ | **Outbox / worker drain** — **`FT-OBX-01`** when rollout touches payment or commerce paths | | SRE / backend |
| ☐ | Stuck order / refund / manual reconciliation path exercised per org policy (Temporal/admin — not this doc alone) | | |

---

## Backup and rollback evidence (P0 / P2)

| Done | Checkpoint | Evidence | Owner |
|------|------------|---------|-------|
| ☐ | **`FT-BKP-01`** — backup id before migration if schema move in rollout | | DBA |
| ☐ | **`FT-RLB-01`** — previous **known-good** image digests + rollback procedure tested or table-top’d | | SRE |

---

## P2 fleet gate (100–1000 claim)

| Done | Checkpoint | Evidence | Owner |
|------|------------|---------|-------|
| ☐ | Storm suite dimension matches **`fleet_scale_target`** (**`100×100`**, **`500×200`**, **`1000×500`**) per **[`../runbooks/production-release-readiness.md`](../runbooks/production-release-readiness.md)** | JSON artifact attach | |
| ☐ | **Monitoring readiness** script pass when required (`check_monitoring_readiness.sh`) | JSON attach | |

---

## Sign-off

| Outcome | Name | Role | UTC date | Evidence bundle id |
|---------|------|------|----------|-------------------|
| Approved widen / Freeze / Rollback initiate | | | | |

---

## Related

- [`release-process.md`](release-process.md) — cadence (**do not confuse** **`deploy-production.yml` pointer-only** vs **`deploy-prod.yml`** — see **`production-release-readiness`**).
- [`production-smoke-tests.md`](production-smoke-tests.md) — CI **`GET`** smoke tiers.
- [`../testing/field-test-cases.md`](../testing/field-test-cases.md) — spreadsheet-style matrix.
