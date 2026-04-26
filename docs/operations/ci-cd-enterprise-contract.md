# CI/CD enterprise contract (summary)

This page summarizes what this repository’s **workflows and contracts** are designed to guarantee for **enterprise** use, what **must stay manual**, what **evidence** operators should retain, and what is **out of scope** or **not guaranteed** by automation.

**Production is manual-only:** the **Deploy Production** workflow is **`workflow_dispatch`** on **`main`**. Pushes, merges, and successful **Security Release** runs do **not** auto-deploy to production. See [release-process.md](release-process.md).

**Two-VPS and downtime:** a **sequential** app-node rollout in GitHub Actions does **not** automatically guarantee **global** zero downtime. `TRAFFIC_DRAIN_MODE=none` records **`zero_downtime_claim: false`** for the global sense; **`caddy`** and **`external-lb`** document operator hooks and may still not replace a real load balancer or DNS design. See [two-vps-rolling-production-deploy.md](two-vps-rolling-production-deploy.md).

---

## What is “enterprise-ready” in this design

- **Deterministic chain on `develop` / `main`:** **CI** → **Security** (repo) → **Build and Push Images** → **Security Release** (verdict + artifacts), with offline contract checks in PR CI to prevent wiring regressions. See [../ci-cd/staging-production-gate.md](../ci-cd/staging-production-gate.md) and [../runbooks/cicd-release.md](../runbooks/cicd-release.md).
- **Immutability for deploy:** digest-pinned **`...@sha256:...`** app and goose images as validated through **Build** and **Security Release** (not `latest` alone).
- **Pre-production gate:** when enabled, **Staging Deployment Contract** with **`real_staging`** and **`promotion_eligible: true`** evidence before production’s strict path (see [staging-preprod-gate.md](staging-preprod-gate.md)).
- **Security verdict as the deploy authorization signal:** only **Security Release**’s **`security-verdict`**, not the repo **Security** scan workflow alone, is the **gate** for deploy-oriented workflows, as described in [../runbooks/security-release.md](../runbooks/security-release.md).
- **Auditable artifacts:** `security-verdict`, staging and production evidence JSON, SBOM and signing material from **Build** where configured—see [../runbooks/release-process.md](../runbooks/release-process.md) and [release-evidence-retention.md](release-evidence-retention.md).
- **Production manifest and post-deploy evidence** packages for **release audit** and operations triage, as produced by the **Deploy Production** job when completed.

## What must remain manual

- **GitHub org configuration:** branch protection, **environments** (`production` reviewers, allowed branches), secrets and variables, not driven by repo commits alone. See [github-governance.md](github-governance.md).
- **Starting production deploy** — the **Deploy Production** workflow form and **`production`** environment approval.
- **Field validation** of devices, cash hardware, and real PSP behavior (partially covered by smoke tests at the HTTP layer only). See [field-rollout-checklist.md](field-rollout-checklist.md).
- **Disaster recovery** beyond what backup drills and runbooks cover—**restore** to alternate regions or cold standby is operations work.
- **Payment / legal / regulatory** sign-off for live traffic—CI cannot replace compliance.

## What evidence is required (typical)

| Evidence | Role |
| --- | --- |
| **Build** / **release-candidate** coordinates | Ties a deploy to a specific commit and image digests |
| **Security Release** + **`verdict: pass`** | Authorizes the deploy path |
| **Staging deploy evidence** | Required for strict production promotion (unless an approved, documented bypass is used) |
| **Backup / drill id** (when policy requires) | For migrations; see [production-backup-restore-drill.md](production-backup-restore-drill.md) |
| **Deploy Production** artifacts (manifest, `production-deploy-evidence`, optional SLO JSON) | Post-deploy audit trail |

Record **run ids** and **artifact names**; never store **tokens** or **private keys** in tickets.

## What is not guaranteed by CI/CD

- **Zero customer-visible downtime** without correct **DNS**, **load balancing**, and **drain** configuration—see [two-vps-rolling-production-deploy.md](two-vps-rolling-production-deploy.md).
- **Database backward compatibility** of application-only rollback: **image rollback** does not run `goose down` or otherwise reverse all schema changes automatically.
- **End-to-end payment success** in production without operator or PSP test procedures—smoke tests are **tiered and bounded** (see [production-smoke-tests.md](production-smoke-tests.md)).
- **Fleet-wide device behavior** from HTTP checks alone; MQTT, MDB, and store-and-forward paths need **field** validation.
- **Third-party or paid monitoring** is optional; the repo may ship **SLO collection** scripts that do not require a SaaS account ([deploy-monitoring-slo.md](deploy-monitoring-slo.md)).

## Known limitations (explicit)

- **Verdict and timing:** **Security Release** has a **freshness** window for production; stale verdicts are rejected. Re-run the chain if needed.
- **Bypass paths:** `allow_missing_staging_evidence` and similar are **loud, exceptional**; they exist for org emergencies, not the default process.
- **Single-node topologies** require explicit `PRODUCTION_ALLOW_SINGLE_APP_NODE` (and are not the default two-VPS story).
- **Contract-only staging** (no real host) does **not** produce `promotion_eligible: true` for the strict path.

## Related documents

- [release-process.md](release-process.md) — end-to-end flow
- [production-release-checklist.md](production-release-checklist.md) — operator checklist
- [../cicd/CI_CD_FINAL_AUDIT.md](../cicd/CI_CD_FINAL_AUDIT.md) — deeper audit / gap list, if present in your branch
- [artifact-retention.md](artifact-retention.md) — how long artifacts may be retained in GitHub
