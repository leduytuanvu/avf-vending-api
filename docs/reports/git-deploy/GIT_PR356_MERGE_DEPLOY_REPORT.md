> **Relocated from avf-vending-system** — canonical home: `avf-vending-api/docs/reports/git-deploy/`. System copy removed 2026-06-16.

# Git PR #356 Merge / Deploy Report

**Date:** 2026-06-12  
**Workstream:** PR #356 develop→main + Production Deploy  
**Verdict:** **`PR356_MERGED_TO_MAIN`** + **`PRODUCTION_DEPLOY_SUCCEEDED`**

---

## PR #356 unblock (develop BEHIND main)

| PR | Action | Result |
|----|--------|--------|
| **#357** | Squash sync main→develop (file content) | Merged — insufficient for GitHub BEHIND gate |
| **#358** | Merge commit sync main→develop | Merged — links main ancestry |
| **#356** | develop→main (script hardening) | **Merged** `2cebfa68` |

**Merge commit:** `2cebfa68682d4ab468f3d113ae7499c2ecd8ffca`  
**PR URL:** https://github.com/leduytuanvu/avf-vending-api/pull/356

**Files on main:**
- `scripts/repair/repair-machine-bootstrap-metadata.ps1`
- `scripts/e2e/tests/test-metadata-contract.ps1`

---

## Release chain

| Step | Run ID | Result |
|------|--------|--------|
| CI (main) | `27444822139` | success |
| Build and Push Images | `27445014464` | success |
| Security Release | `27445233564` | **verdict: pass** |
| Deploy Production | `27445331104` | **success** |

**Source commit:** `2cebfa68682d4ab468f3d113ae7499c2ecd8ffca`  
**Images:**
- app: `ghcr.io/leduytuanvu/avf-vending-api@sha256:264fdc08cde289d4d41eb301348732cdbb71b508ef273ef4f7ed46e9f30adb01`
- goose: `ghcr.io/leduytuanvu/avf-vending-api-goose@sha256:1762ee3f2709c62f7e5c4696ccdf4b830dee8733eb97cd689349f3c6b3e80b49`

**Deploy inputs:**
- `release_tag`: `v20260612-pr356-develop-main`
- `allow_missing_staging_evidence`: `true` (operator-approved bypass)
- `run_migration`: `true`

**Deploy URL:** https://github.com/leduytuanvu/avf-vending-api/actions/runs/27445331104

Artifacts: `production-deployment-manifest`, `production-deploy-evidence`

---

## Notes

- Script-only change — production images rebuilt from same Go base; repair scripts live in repo for operator runs.
- CAB-A metadata already repaired live (Phase 6); deploy does not re-apply cabinet JSONB.
- Next device blocker remains `STALE_TOKEN_RUNTIME_LOCKED` (separate workstream).

## Log-secrecy

Report scanned — no JWT/password material.
