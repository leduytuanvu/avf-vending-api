# Production E2E automation window

**Purpose:** Time-bounded, reversible relaxation of **PR bypass for a dedicated automation actor** and **production environment deploy approval** so automated production E2E can merge → deploy → test → fix → redeploy without manual review gates — while **never** permanently weakening `develop` / `main` protection.

**Canonical production deploy:** [`.github/workflows/deploy-prod.yml`](../../.github/workflows/deploy-prod.yml) (**Deploy Production**) only. The legacy [`deploy-production.yml`](../../.github/workflows/deploy-production.yml) pointer is **not** a deploy path.

**Related:** [github-governance.md](./github-governance.md), [production-e2e-input-checklist.md](../testing/production-e2e-input-checklist.md)

---

## What this changes (and what it does not)

| Area | During window | After restore |
|------|---------------|---------------|
| Branch rulesets for `main` / `develop` | Adds **one bypass actor** (GitHub App or automation user) | Removes that bypass actor from snapshotted rulesets |
| Classic branch protection | **Untouched** — enable fails if rulesets cannot be used for bypass | N/A |
| Environment `production` required reviewers | **Temporarily removed** (snapshotted first) | Recreated from snapshot |
| Environment `production-e2e-automation` | Created if missing (main-only, no reviewers) — optional alternate path | Left in place (harmless); deploy-prod still uses `production` |
| Workflow auto-triggers | **None** — governance workflow is `workflow_dispatch` only | — |

This tooling **does not** delete rulesets, disable required status checks, or allow force-push. If bypass actors cannot be configured via the rulesets API, **enable aborts** and leaves protection unchanged.

---

## Prerequisites

1. **Repository variables** (Settings → Secrets and variables → Actions → Variables):
   - `AVF_E2E_AUTOMATION_BYPASS_ACTOR_ID` — numeric actor id (GitHub App **Integration** id or automation **User** id)
   - `AVF_E2E_AUTOMATION_BYPASS_ACTOR_TYPE` — `Integration` (recommended for GitHub App) or `User`

2. **Repository secret** (write-capable, never logged):
   - `GH_AUTOMATION_TOKEN` — fine-grained PAT or classic PAT with **Administration: Read and write** on this repo (rulesets + environments). Fallback: `GOVERNANCE_AUDIT_TOKEN` if it has write scope (read-only audit tokens **cannot** enable).

3. **Active branch rulesets** covering `main` and `develop` (Settings → Rules → Rulesets). Classic-only protection without rulesets is **not** supported for bypass — enable will fail loudly.

4. **E2E secrets** (for test runs only; placeholders in docs — set in GitHub Secrets, not in repo):
   - `E2E_PROD_ADMIN_EMAIL`, `E2E_PROD_ADMIN_PASSWORD`, `E2E_PROD_BASE_URL`, `E2E_PROD_GRPC_TARGET`, `E2E_PROD_MQTT_HOST`, `E2E_PROD_MQTT_USERNAME`, `E2E_PROD_MQTT_PASSWORD`, `E2E_PROD_PAYMENT_WEBHOOK_SECRET`

---

## Enable the window

### Option A — GitHub Actions (recommended)

1. Actions → **Production E2E automation window** → **Run workflow**
2. **action:** `enable`
3. **ttl_minutes:** `120` (default; min 15, max 480)
4. **confirmation:** type exactly `I_ACCEPT_TEMPORARY_PRODUCTION_AUTOMATION_RISK`

### Option B — Local / controlled runner

```bash
export GITHUB_REPOSITORY=owner/repo
export GH_AUTOMATION_TOKEN=...   # write-capable; never echo or commit
export AVF_E2E_AUTOMATION_BYPASS_ACTOR_ID=123456
export AVF_E2E_AUTOMATION_BYPASS_ACTOR_TYPE=Integration
export AVF_E2E_AUTOMATION_WINDOW_CONFIRM=I_ACCEPT_TEMPORARY_PRODUCTION_AUTOMATION_RISK

bash scripts/governance/enable-production-e2e-automation-window.sh --ttl-minutes 120
```

**Artifacts:** timestamped JSON under `.e2e-runs/governance/governance-snapshot-<UTC>.json` and `.e2e-runs/governance/active-window.json` (gitignored).

**TTL:** default **2 hours**. When `expires_at` passes, **restore is mandatory** before the next production change window.

---

## Status / dry-run restore check

Before E2E, confirm snapshot exists and inspect protection gaps:

```bash
export GITHUB_REPOSITORY=owner/repo
export GH_AUTOMATION_TOKEN=...

bash scripts/governance/restore-production-protections.sh --status --dry-run
```

Or run the workflow with **action:** `status`.

---

## Restore protections (rollback of governance window)

### Option A — GitHub Actions

Actions → **Production E2E automation window** → **Run workflow** → **action:** `restore`

### Option B — Shell

```bash
export GITHUB_REPOSITORY=owner/repo
export GH_AUTOMATION_TOKEN=...

bash scripts/governance/restore-production-protections.sh
# Or a specific snapshot:
bash scripts/governance/restore-production-protections.sh --snapshot .e2e-runs/governance/governance-snapshot-20260523T120000Z.json
```

Restore will:

1. Remove the temporary bypass actor from snapshotted rulesets
2. Recreate `production` **required_reviewers** deployment rules from the snapshot
3. **Fail loudly** if `main` / `develop` are unprotected or production reviewers are still missing

Verify after restore:

```bash
make verify-governance
bash scripts/governance/restore-production-protections.sh --status
```

---

## End-to-end automation sequence (after this PR merges)

Do **not** start destructive production E2E until **all** of the following are true:

1. **Protection snapshot exists** (enable succeeded; file under `.e2e-runs/governance/`)
2. **Restore tested** in `--status` / dry-run mode
3. **Rollback documented** — this runbook + [production-rollback.md](./production-rollback.md)
4. **Code on `main`** with all required checks green
5. **Production deployed** via **Deploy Production** (`deploy-prod.yml`) successfully

Suggested flow:

```text
merge PR → develop (auto-merge if policy allows)
merge develop → main (after checks)
workflow_dispatch: Deploy Production (deploy-prod.yml)
enable automation window (if not already open)
run production E2E (secrets from GitHub Secrets only)
restore automation window (mandatory; also if TTL expired)
```

---

## Risks

- **Temporary production deploy without human approval** while the window is open
- **Automation actor can bypass PR review** on protected branches (status checks still apply unless the actor is configured to bypass those too — configure bypass mode **Pull request** only in GitHub UI if offered; this tool sets `bypass_mode: always` on the ruleset API — review org policy)
- **Expired window without restore** leaves production without required reviewers until restore runs
- **Secrets exposure in E2E logs** — never print admin passwords, JWTs, refresh tokens, webhook secrets, SSH keys, registry tokens, database URLs, or payment secrets

---

## Troubleshooting

| Symptom | Action |
|---------|--------|
| Enable fails: rulesets forbidden | Use `GH_AUTOMATION_TOKEN` with Administration write |
| Enable fails: no ruleset covers main/develop | Configure branch rulesets per [github-governance.md](./github-governance.md) |
| Restore fails: production reviewers missing | Manually re-add required reviewers in Settings → Environments → `production`, then re-run restore |
| Active window marker blocks re-enable | Run restore, or `--force` only after manual verification |
| Deploy still waits for approval | Confirm enable removed `required_reviewers` rules; re-run status |

---

## Files

| Path | Role |
|------|------|
| `scripts/governance/enable-production-e2e-automation-window.sh` | Enable entrypoint |
| `scripts/governance/restore-production-protections.sh` | Restore / status entrypoint |
| `tools/production_e2e_governance_window.py` | Snapshot / API implementation |
| `.github/workflows/production-e2e-automation-window.yml` | Manual workflow wrapper |
| `scripts/ci/verify_governance_protection_window.sh` | Static contract tests |

Contract check: `bash scripts/ci/verify_governance_protection_window.sh`
