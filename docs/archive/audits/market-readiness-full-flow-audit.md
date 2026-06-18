# Market readiness — baseline audit (Phase 0)

**Date:** 2026-05-27  
**Branch:** `qa/market-readiness-full-flow-validation`  
**Base:** `develop` (local, clean working tree at branch creation)

## Git baseline

| Item | Value |
|------|--------|
| Branch | `qa/market-readiness-full-flow-validation` |
| Working tree | Clean at start |
| Remote | `origin` → `leduytuanvu/avf-vending-api` |

## Secret scan (tracked files)

Manual grep patterns applied for JWT blobs, live API keys, embedded `DATABASE_URL`/`REDIS_URL` with credentials, `CLOUDINARY_API_SECRET=`, `sk_live_`, PEM private keys in non-example paths.

| Check | Result |
|-------|--------|
| `.env.production` tracked | **Not present** (gitignored) |
| `tests/e2e/.env.production.destructive.local` | **Not tracked** |
| `*.local` env with secrets | **Examples only** use `CHANGE_ME` / placeholders |
| Postman env in repo | Production template uses `<fill locally>` placeholders |

**Note:** Full secret scan runs in CI (`Secret Scan` workflow). Local `git grep` for high-entropy tokens on `*.go`/`*.json` committed files: no obvious live secrets found in examples.

## Artifacts that must stay gitignored

- `.e2e-runs/`
- `.production-latency-runs/` (when present)
- `.production-smoke-runs/` (this task)
- `postman/environments/*.local.json`

## Uncertainties

- Live production credentials on VPS are **not** visible in git (expected).
- Whether PR #316 (perf/latency) is merged into `develop` on remote — local `develop` may be ahead of `origin/docs/phase6-merge-record`; inventory uses committed `swagger.json`.

## Next phases

1. `tools/generate_market_readiness_inventory.py` → full inventory MD/JSON  
2. Flow validation against E2E manifests + handlers  
3. Automated test gate → `market-readiness-test-results.md`  
4. Go-live checklist + production smoke script  
