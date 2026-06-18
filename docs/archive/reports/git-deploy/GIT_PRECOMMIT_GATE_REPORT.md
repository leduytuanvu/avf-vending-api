> **Relocated from avf-vending-system** — canonical home: `avf-vending-api/docs/reports/git-deploy/`. System copy removed 2026-06-16.

# Git Pre-Commit Gate Report

**Date:** 2026-06-11  
**Repo:** `avf-vending-api` (`leduytuanvu/avf-vending-api`)  
**Branch:** `chore/e2e-sellable-layout-setup` (PR #350)  
**Verdict:** **PRE_COMMIT_GATES_PASS**

---

## Gate 1 — Metadata contract tests

**Command:** `scripts/e2e/tests/test-metadata-contract.ps1`

| # | Test | Result |
|---|------|--------|
| 1 | TCN cash layout without metadata | PASS (validation FAIL as expected) |
| 2 | TCN cash layout with contract keys | PASS |
| 3 | Pilot merge includes contract keys | PASS |
| 4 | Placeholder machine ID rejected | PASS |
| 5 | `setup-tcn-cash-products-a1-a10.ps1 -DryRun` | PASS (exit 0) |
| 6 | Repair dry-run without admin creds | PASS (exit 4, `BLOCKED_ADMIN_ENV_MISSING`) |

**Result:** 6/6 PASS

---

## Gate 2 — Repair script dry-run

**Command:** `scripts/repair/repair-machine-bootstrap-metadata.ps1 -DryRun -NonInteractive`

**Verdict:** `BLOCKED_ADMIN_ENV_MISSING` (exit 4) — expected without `AVF_ADMIN_EMAIL` / `AVF_ADMIN_PASSWORD`.

---

## Gate 3 — Secret scan (staged diff)

Manual scan of `git diff --cached` for bearer tokens, API keys, and hardcoded passwords.

**Finding:** Only env var name references (`AVF_ADMIN_PASSWORD` null/clear/restore) in repair script — no secret values.

**Result:** PASS

---

## Gate 4 — Go tests (staged Go change)

**Staged:** `internal/grpcserver/machine_grpc_production_contract_test.go`

**Command:** `go test ./internal/grpcserver/... -run Bootstrap -count=1`

**Result:** PASS (`ok github.com/avf/avf-vending-api/internal/grpcserver`)

---

## Gate 5 — Staged scope audit

**Command:** `git diff --cached --name-status`

```
M  internal/grpcserver/machine_grpc_production_contract_test.go
M  scripts/e2e/examples/pilot-cabinet-layout-a.json
M  scripts/e2e/layout_config_schema.py
M  scripts/e2e/setup-machine-sellable-layout-apply.sh
M  scripts/e2e/setup-tcn-cash-products-a1-a10.ps1
A  scripts/e2e/tests/fixtures/layout-tcn-cash-missing-metadata.json
A  scripts/e2e/tests/fixtures/layout-tcn-cash-valid-metadata.json
A  scripts/e2e/tests/test-metadata-contract.ps1
A  scripts/repair/repair-machine-bootstrap-metadata.ps1
```

No `postman/**` deletions, no `reports/e2e/**` deletions, no out-of-scope paths.

**Result:** PASS — safe to commit

---

## Summary

| Gate | Status |
|------|--------|
| Contract tests 6/6 | PASS |
| Repair dry-run | PASS (`BLOCKED_ADMIN_ENV_MISSING`) |
| Secret scan | PASS |
| Go test (Bootstrap) | PASS |
| Staged scope | PASS |

**Proceed:** commit and push to update PR #350.
