# Phase 3 — Dynamic setup/verify (multi-file + pilot)

**UTC:** 2026-06-03T14:10:36Z  
**Verdict:** **PASS**

## Summary

Generic machine layout harness supports **multi-file inputs** (cabinet/layout, slot assignments, inventory, payment profile, hardware profile, optional destructive scope) merged into a validated unified manifest. TCN cash-only **pilot wrapper** (`setup-tcn-cash-products-a1-a10.ps1` / `verify-tcn-cash-products-a1-a10.ps1`) uses split examples under `scripts/e2e/examples/` for machine `019e702c-11c6-7ab0-89c7-5eb32f0b12cb`, cabinet **A**, slots **A1–A10** only.

## Multi-file pilot inputs

| Role | File |
|------|------|
| Cabinet + layouts | `scripts/e2e/examples/pilot-cabinet-layout-a.json` |
| Slot products / prices | `scripts/e2e/examples/pilot-slot-assignments-a1-a10.json` |
| Inventory quantities | `scripts/e2e/examples/pilot-inventory-a1-a10.json` |
| Payment profile | `scripts/e2e/examples/payment-profile-cash-only.json` |
| Hardware profile | `scripts/e2e/examples/hardware-profile-tcn.json` |
| Destructive scope | `scripts/e2e/examples/destructive-scope-a1-a10.json` |
| Catalog defaults | `scripts/e2e/examples/pilot-catalog-defaults.json` |

SKUs: `TCN-E2E-A1` … `TCN-E2E-A10`. Destructive scope locked to **A / 1–10** (no slots outside pilot range).

## Tests

```
py -3 -m pytest scripts/e2e/tests -q
12 passed
```

Covers: unified layouts, two-cabinet example, disabled-slot rule, merge equivalence, pilot paths, inventory validation errors.

## Production runs

| Step | Result | Artifact dir |
|------|--------|----------------|
| Setup + verify | **PASS** | `reports/e2e/setup-machine-layout/20260603T140947Z` |
| Verify (standalone) | **PASS** | `market-readiness-runs/20260603T132538Z-tcn-cash-bill-tcn-init/backend/verify-layout-20260603T140923Z` |
| Idempotent re-setup (`-SkipVerify`) | **PASS** | `reports/e2e/setup-machine-layout/20260603T141023Z` |

### Verify metrics (pilot)

| Metric | Value |
|--------|------:|
| App-facing catalog items | 10 |
| Sellable slots (machine) | 10 |
| Destructive test slots | 10 |
| Cash-only runtime | true |
| Hidden rows | 0 |

## Auth / apply fixes (this phase)

- Load `tests/e2e/production/.env.production.e2e.local` overlay (admin creds) in apply script.
- Map `E2E_PROD_ADMIN_*` / `E2E_ADMIN_TOKEN` aliases.
- Admin operator session: `POST /v1/admin/machines/{id}/operator-sessions/start` (not machine JWT login path).
- Product upsert: list-by-SKU then POST; CRLF-safe SKU handling on Windows.
- Curl helpers return HTTP codes; jq null-safe for slots/stock.

## Gate

**Continue to Phase D:** **YES** (backend catalog/setup/verify PASS for pilot A1–A10; app Phase D remains next blocker in market readiness pipeline).
