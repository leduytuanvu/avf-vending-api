# 2026-07-02 deep clean plan

**Branch:** `cleanup/repo-deep-clean-docs-reports-scripts`  
**Baseline:** [deep-clean-baseline](2026-07-02-deep-clean-baseline.md) · [inventory](2026-07-02-deep-clean-inventory.md)

## Execution checklist

1. Archive `docs/reports/postman/` → `docs/archive/reports/postman/`
2. Archive verification reports (keep README index) → `docs/archive/reports/verification/`
3. Archive `enterprise-api-backend-audit-report.md`, `p0-hardening-report.md` → `docs/archive/audits/`
4. Archive `docs/release/evidence/` → `docs/archive/release/evidence/`
5. Archive prior cleanup working docs → `docs/archive/cleanup/2026-07-02/`
6. Align `generate_production_full_suite.py` to `postman/suites/production-full/avf-vending-production.full.*`
7. Add `scripts/repair/README.md`, `deployments/loadtest/README.md`
8. Update docs indexes and fix broken links
9. Add `postman/production-full-suite/` to `.gitignore`
10. Verify + write final report

## DO NOT TOUCH

Migrations, workflows, deployments, Postman CI paths, runbook procedures (link updates only), `rest-route-matrix.json`, production-deploy reports.

## Rollback

`git revert` deep-clean commits or `git mv` archive paths back to active locations.
