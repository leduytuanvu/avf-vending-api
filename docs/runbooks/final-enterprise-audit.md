# Final enterprise audit (strict)

**Purpose:** Single checklist for executives and release managers. **Docs-only** routes or roadmap bullets are **not** treated as implemented. **Scale-1000** requires **1000×500** storm evidence with strict accounting fields — not documentation claims.

**Evidence:** Run `make verify-enterprise-release` and capture `emit_verify_enterprise_result_json.sh`. Storm tiers: [production-release-readiness.md](./production-release-readiness.md).

---

## A. READY_FOR_PILOT

**Answer:** **YES** when `make verify-enterprise-release` passes on the release SHA, services are wired for the pilot slice, smoke tests pass, and operators accept [Known risks](#e-pop1p2-remaining-risks). Otherwise **NO**.

---

## B. READY_FOR_100

**Answer:** **YES** only when **A** is YES **and** staging/production evidence includes a **100×100** storm PASS with `critical_lost=0`, `duplicate_critical_effects=0`, `db_pool_result=health_result=restart_result=pass`, and monitoring readiness PASS. Otherwise **NO**.

---

## C. READY_FOR_500

**Answer:** **YES** only when **B**’s bar is met for the org’s prior tier **and** **500×200** storm evidence meets the same strict fields. Otherwise **NO**.

---

## D. READY_FOR_1000

**Answer:** **YES** only when **C** is satisfied **and** **1000×500** storm evidence meets the same strict fields. If only 100×100 (or weaker) evidence exists, answer **NO** — partial storms **do not** authorize 1000-machine readiness.

---

## E. P0/P1/P2 remaining risks

| Area | Risk |
| --- | --- |
| P0 | Nil wiring (503) on activation, commerce, telemetry store if not configured — expected but must be explicit per deploy. |
| P1 | MQTT/JetStream mis-sizing causes backlog; HTTP poll used as primary command transport — operational anti-pattern. |
| P2 | Evidence pack or manifest drift (wrong `fleet_scale_target`, unpinned images) blocks audit even when code is sound. |

---

## F. API missing

Treat as **missing** until present in **`docs/swagger/swagger.json`** **and** mounted in `internal/httpserver`:

- Any path listed only in [roadmap.md](../api/roadmap.md).
- “Future” client flows that reference HTTP surfaces not in OpenAPI.

---

## G. API duplicated / redundant

| Item | Note |
| --- | --- |
| Device vend path | Commerce `/vend/*` vs `POST /v1/device/.../vend-results` — integration fallback; clients must pick one primary story per deployment. |
| Command delivery | MQTT primary vs `POST .../commands/poll` **fallback** — document per site. |

---

## H. Security risks

- **Public Swagger** when `HTTP_SWAGGER_UI_ENABLED=true` without edge ACL.
- **Metrics** on public bind without scrape auth, or **ops** `/metrics` accidentally scraped without bearer when `METRICS_SCRAPE_TOKEN` is set.
- **Webhook HMAC** misconfiguration, clock skew, or **provider** mismatch vs stored payment.
- **Machine JWT** scope bugs → cross-tenant data exposure (mitigated by middleware; verify per release).

---

## I. Runtime scaling risks

- Postgres pool exhaustion under storm reconnect.
- JetStream retention / max bytes vs peak telemetry.
- Reconnect storms from kiosk fleet after outage ([telemetry-production-rollout.md](./telemetry-production-rollout.md)).

---

## J. Exact files to fix (when audit fails)

| Gate | Fix location |
| --- | --- |
| Static verify | Failing `go test`, `tools/openapi_verify_release.py`, or `scripts/verify_enterprise_release.sh` phase |
| Swagger drift | `tools/build_openapi.py`, handlers, then `make swagger` |
| Storm gate | `deployments/prod/shared/scripts/validate_production_scale_storm_evidence.py` inputs; staging workflow |
| Evidence pack | `deployments/prod/scripts/build_release_evidence_pack.sh`; manifest JSON |

---

## K. Final production release recommendation

Use this sentence on the ticket:

1. **Pilot:** “**Approve pilot** when Section **A** is YES and rollback is rehearsed.”
2. **Scale tier:** “**Approve scale-{N}** only when Sections **A–D** through the target tier are YES with attached storm JSON and digest-pinned manifest.”

If storm evidence is incomplete for the claimed tier, the recommendation must be **“Do not promote”** regardless of green unit tests.

---

## Related

- [Production release readiness](./production-release-readiness.md)
- [API surface audit](../api/api-surface-audit.md)
