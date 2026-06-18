> **Relocated from avf-vending-system** — canonical home: `avf-vending-api/docs/reports/git-deploy/`. System copy removed 2026-06-16.

# Git Push / Merge / Deploy Report

**Date:** 2026-06-12  
**Workstream:** PR #351 develop→main + Production Deploy  
**Verdict:** **`PR351_MERGED_TO_MAIN`** + **`PRODUCTION_DEPLOY_SUCCEEDED`**

---

## PR #351 unblock (develop BEHIND main)

Direct `gh pr update-branch 351` and push to `develop` were blocked by ruleset (`Changes must be made through a pull request`).

**Resolution:** Created sync PR **#352** (`chore/sync-main-into-develop-pr351`) to merge `main` into `develop`, merged when CI green. PR #351 then became `mergeStateStatus: CLEAN`.

| PR | Action | Merge commit |
|----|--------|--------------|
| **#352** | Sync main→develop | `e7be693d` |
| **#351** | develop→main | `55792c68` |

---

## Security Release blockers (fixed)

After PR #351 merge, **Security Release** failed Trivy scans:

| Issue | Fix PR | Merge |
|-------|--------|-------|
| Go stdlib CVE-2026-42504 (images built with Go 1.25.10) | **#353** — Dockerfile Go 1.25.11 | `bceb0f8f` |
| goose OpenSSL CVE-2026-45447 (libcrypto3 3.5.6-r0) | **#354** — `apk upgrade` on goose runtime | `04963b98` |

---

## Release chain (final successful)

| Step | Run ID | Result |
|------|--------|--------|
| CI (main) | `27438050495` | success |
| Build and Push Images | `27438268565` | success |
| Security Release | `27438492825` | **verdict: pass** |
| Deploy Production | `27438625783` | **success** |

**Source commit:** `04963b98a9ec900b760624cd25e59ec227e16eca`  
**Images:**
- app: `ghcr.io/leduytuanvu/avf-vending-api@sha256:d3fd8eeb4a5f2a370ca14018f10747e1a450e2410b09ea3c5d2957ddbb433a3a`
- goose: `ghcr.io/leduytuanvu/avf-vending-api-goose@sha256:8b31a4b997cb275c2af6133bef0d14674e3570eaac7b556888d1d63fa99fa716`

**Deploy inputs:**
- `release_tag`: `v20260612-pr351-develop-main`
- `allow_missing_staging_evidence`: `true` (operator-approved bypass)
- `run_migration`: `true`

**Deploy URL:** https://github.com/leduytuanvu/avf-vending-api/actions/runs/27438625783

Artifacts: `production-deployment-manifest`, `production-deploy-evidence`

---

## Live metadata repair (still pending)

Production deploy **does not** patch cabinet JSONB metadata. Operator still must run:

`avf-vending-api/scripts/repair/repair-machine-bootstrap-metadata.ps1` with `AVF_ADMIN_*` + write confirmation.

**Verdict:** `BOOTSTRAP_METADATA_REPAIR_READY_BUT_NOT_APPLIED`

---

## Invalid claims (do not use)

- `STOREFRONT_READY`, `PAYMENT_READY`, `SALE_READY`, `MARKET_READY`, `BACKEND_METADATA_REPAIRED`
