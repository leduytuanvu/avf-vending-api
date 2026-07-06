# Numeric Activation Code — Production Deploy Report

**Date:** 2026-07-06 UTC  
**Production URL:** https://api.ldtv.dev

## Deploy chain

| Step | Run ID | SHA | Result |
|------|--------|-----|--------|
| CI (main merge PR #438) | [28783852372](https://github.com/leduytuanvu/avf-vending-api/actions/runs/28783852372) | `999b9e93e4acc63dacb2c8087bc0a8ea47316a00` | success |
| Build and Push Images | [28784120662](https://github.com/leduytuanvu/avf-vending-api/actions/runs/28784120662) | `999b9e93...` | success |
| Security Release | [28784379868](https://github.com/leduytuanvu/avf-vending-api/actions/runs/28784379868) | `999b9e93...` | success |
| Deploy Production | [28784499441](https://github.com/leduytuanvu/avf-vending-api/actions/runs/28784499441) | `999b9e93...` | success |

## Image refs deployed

- App: `ghcr.io/leduytuanvu/avf-vending-api@sha256:fcfa5daf2205fd7eeb7d441334de31b09a956617931aaa72ae1b75c64a25be1f`
- Goose: `ghcr.io/leduytuanvu/avf-vending-api-goose@sha256:980ef0a4b5df0e90530991277f8a2e4d2864061c1d843157112a6c873107f555`

## Post-deploy health

| Endpoint | Result |
|----------|--------|
| `GET /health/live` | 200 `ok` |
| `GET /health/ready` | 200 `ok` |
| `GET /version` `git_sha` | **`999b9e93e4acc63dacb2c8087bc0a8ea47316a00`** (matches main) |

## Git merges

| Branch | Merge | SHA |
|--------|-------|-----|
| develop | PR #437 | `11c229f6` (includes `95d93cd1` implementation) |
| main | PR #438 | `999b9e93e4acc63dacb2c8087bc0a8ea47316a00` |

## Rollback

Previous production SHA: `2cc5569e1beebbff218848ab6ac42da952a489e5`. Redeploy via Deploy Production rollback mode if needed.

## Staging gate

`allow_missing_staging_evidence=true` — no successful Staging Deployment Contract run for this digest (consistent with prior production releases).
