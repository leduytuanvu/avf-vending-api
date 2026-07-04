# Project Cleanup Audit — Market Readiness Final

**UTC:** 20260704T002100Z

## Classification

| Class | Path / item | Action |
|-------|-------------|--------|
| **A — Keep tracked** | `db/`, `proto/`, `tools/market_readiness/`, `tools/production_full_test/`, `docs/reports/market-readiness-final/` | No delete |
| **A — Keep tracked** | Latest runtime-fleet verdict docs | Keep; archive older series later if owner agrees |
| **B — Safe gitignored cleanup** | `tmp/`, `ci-reports/`, `.pytest_cache/`, old `docs/testing/production-e2e/RESULTS_*` | `clean-local-artifacts.ps1 -Apply` |
| **C — Local working tree** | Deleted `reports/enterprise-flow-verification/20260703T013119Z/*` | Restore or commit intentional removal separately — not part of market harness |
| **D — Never delete** | Migrations 00017/00018, deploy scripts, harness, `.github/workflows/deploy-prod.yml` | Protected |

## Commands run

```text
git clean -ndX          → .env, tmp/, ci-reports/, deployments/prod/backups/, etc.
clean-local-artifacts   → 7 candidates (tmp/, ci-reports/, repomix xml, …)
find tmp/cache          → tmp/ exists
```

## Evidence policy

- Keep `docs/reports/market-readiness-final/` + newest `reports/production-market-readiness-final/{UTC}/` when produced.
- Do **not** delete superseded `docs/reports/machine-runtime-fleet/` without owner sign-off (plan: move to archive).

## Safe cleanup executed

Dry-run only in this session (`clean-local-artifacts.ps1` without `-Apply`). Operator may run `-Apply` for gitignored artifacts listed above.
