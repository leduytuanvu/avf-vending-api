# Generated reports

Phase reports, verification outputs, and gate evidence. These are **documentation artifacts** — not runtime configuration.

## Product media / offline cache

[`product-media-offline-cache/`](product-media-offline-cache/) — phase 0–6 reports, regression/consistency gates, server migration verification templates.

Operator runbook: [`../runbooks/product-media-offline-cache-production-migration.md`](../runbooks/product-media-offline-cache-production-migration.md).

## Test / merge verification

[`test/`](test/) — develop merge verification, scope-id cleanup reports, readonly smoke output paths.

## Repository cleanup gates

- [`final-gate-audit/`](final-gate-audit/) — final gate classification and grep evidence
- [`final-single-scope-audit/`](final-single-scope-audit/) — single-scope zero-hit verification

**Note:** Root `reports/` was consolidated here in Phase 2 (2026-05-20). Update bookmarks from `reports/` → `docs/reports/`.

## Duplicate doc removed (Phase 2)

| Removed | Canonical copy | Reason |
|---------|----------------|--------|
| `postman/suites/full-production-suite/05_PRODUCTION_TEST_EXECUTION_ORDER.md` | `docs/testing/05_PRODUCTION_TEST_EXECUTION_ORDER.md` | Byte-identical duplicate (SHA256 match) |

Postman collection JSON descriptions may still mention the old path in embedded text; regenerate suites with `postman/suites/full-production-suite/generate_full_postman_suite.py` when convenient.
