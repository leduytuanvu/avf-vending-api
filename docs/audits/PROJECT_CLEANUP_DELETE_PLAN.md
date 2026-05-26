# Project cleanup delete plan

**Status:** DELETE PLAN READY — executed in Phase 7 after validation gates.

| Path | Tracked | Size (approx) | Class | Ref check | Action | Risk |
|------|---------|---------------|-------|-----------|--------|------|
| `.tmp-deploy-candidate/` | untracked | small | SAFE_DELETE | none | rm -rf | low |
| `.tmp-deploy-inputs/` | untracked | small | SAFE_DELETE | none | rm -rf | low |
| `.tmp-last-deploy/` | untracked | small | SAFE_DELETE | none | rm -rf | low |
| `.tmp-*.md`, `.tmp-*.msg`, `.tmp-*.txt`, `.tmp-*.sh`, `.tmp-*.json`, `.tmp-*.log` | untracked | ~2M | SAFE_DELETE | none | rm -f | low |
| `docs/testing/production-e2e/RESULTS_*.md` (untracked only) | untracked | ~2M | SAFE_DELETE | not in `git ls-files` | rm | low |
| `docs/testing/production-e2e/*072623*` MANUAL/API/POSTMAN | untracked | small | SAFE_DELETE | superseded by tracked `192300Z` | rm | low |
| `.e2e-runs/production/*` except `20260525T192300Z-1196-5901` | gitignored | ~80M | SAFE_DELETE | regenerable | rm -rf | low |
| `.e2e-runs/live-run*.log` | gitignored | small | SAFE_DELETE | none | rm -f | low |
| `.e2e-runs/debug-postman.py`, `ssh-*.py` | gitignored | tiny | SAFE_DELETE | none | rm -f | low |
| Tracked `RESULTS_20260523T081121Z` | tracked | small | REVIEW_REQUIRED | historical | **keep** | — |
| Tracked canonical `192300Z` docs | tracked | ~9M | KEEP_REQUIRED | CI/docs | **keep** | — |
| `postman/production-full-suite/` 4.2M JSON | tracked | 4.2M | REVIEW_REQUIRED | suite generator | **keep** | — |

**Not deleted:** migrations, source, workflows, manifests, proto, Docker, tracked Postman production E2E JSON.
