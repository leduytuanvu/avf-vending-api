# Field rollout checklist (vending machines)

Use this list **after** a production API deploy (or in a coordinated release that includes **device/edge** changes). It is **not** a substitute for the GitHub [production-release-checklist.md](production-release-checklist.md); it covers **on-machine** and **operator** validation for a vending control-plane rollout.

**Who owns evidence:** name an **evidence owner** (role + name) in the change ticket. That person ensures screenshots, log excerpts (non-secret), and sign-off are stored per your document retention policy.

**Secrets:** do not paste MQTT passwords, API keys, payment provider secrets, or SSH credentials into chat or uncontrolled tickets. Reference **internal secret stores** and **redacted** log lines only.

---

## Preconditions

- [ ] **Machine online** (or a representative set) **before** the deploy window: connectivity to the public API, healthy check-in path, and expected firmware/app version if your fleet tracks one.
- [ ] **Maintenance window** and **rollback owner** are agreed (API image rollback is distinct from long-running DB operations—see [two-vps-rolling-production-deploy.md](two-vps-rolling-production-deploy.md)).

## Product and catalog

- [ ] **Product list sync** — machines show expected catalog/price book after the release (or after the next allowed sync), per your planogram process. Spot-check a **low-urgency** machine and a **high-SKU** machine if applicable.
- [ ] **Inventory reconciliation** — known stock and reservations align with the operator’s view after a test transaction cycle (or per your **reconciler** runbook, if you run manual reconciliation jobs).

## Payments

- [ ] **QR / cashless (sandbox or live, per policy)** — a **small-value** test payment completes successfully where regulations and PSP rules allow; decline paths behave as before.
- [ ] **Cash** — if the deployment touches cash handling, bill/coin path, or MDB bridges, run the **manufacturer-allowed** test matrix (may require **hardware in lab**, not on every street unit).

## Dispense

- [ ] **Dispense success** — at least one controlled test vend per critical machine model (or a statistically sound sample) completes end-to-end with correct telemetry/ACK if your product requires it.
- [ ] **Dispense failure recovery** — intentional or simulated failure paths (jam, timeout, out-of-stock) result in a **defined operator outcome**: retry, refund/cancel, or service ticket, without orphan payments per [commerce / temporal](../architecture/current-architecture.md) behavior.

## Connectivity

- [ ] **MQTT reconnect** — after a **brief** network flap (or in lab), the device recovers without manual reboot (subject to your broker policy and keepalive settings). See [mqtt-contract.md](../api/mqtt-contract.md) for the contract in this repo.
- [ ] **Offline / retry** — a machine that was offline during deploy later **replays** or **reconciles** per your client design; no duplicate settlement when idempotency keys are honored.

## Exception handling and money movement

- [ ] **Refund or manual reconciliation path** — for stuck orders, the operator can complete **refund**, **cancel**, or **manual review** per your PSP and **Temporal** / admin flows (see API handoff docs and runbooks, not this checklist alone).
- [ ] **Evidence owner** confirms **non-secret** evidence is attached: timestamps, machine ids, test transaction ids, and links to **Deploy Production** / **smoke** artifacts where required.

## Sign-off

- [ ] **Field or pilot lead** signs off, or a **staged rollout** list is complete before fleet-wide enablement.
- [ ] If anything fails, follow **incident** and **rollback** procedures before expanding scope ([../runbooks/deploy-failure.md](../runbooks/deploy-failure.md) if present, [../runbooks/production-rollback.md](../runbooks/production-rollback.md)).

---

## Related

- [release-process.md](release-process.md) — when production deploys happen in the pipeline
- [production-smoke-tests.md](production-smoke-tests.md) — what CI may probe vs what must be field-proven
- [../runbooks/telemetry-production-rollout.md](../runbooks/telemetry-production-rollout.md) — telemetry-specific rollout notes, if in use
