# Machine Code Format Audit

**UTC:** 20260706T050000Z  
**Canonical test/bootstrap format:** `^AVF[0-9]{6}$` (exactly six digits after `AVF`)

---

## Summary

Production test bootstrap now generates only six-digit machine codes. Activation admin resolver already enforced `{6}$`. Fleet CRUD continues to accept `{6,}$` without change. No DB migration.

---

## Generators and validators

| Location | Pattern | Role | Action |
|----------|---------|------|--------|
| `tools/production_full_test/_common.py` `production_machine_code()` | `^AVF[0-9]{6}$` | E2E/bootstrap machine create | **Fixed** — `AVF{n:06d}` only |
| `tools/production_full_test/_common.py` `PRODUCTION_MACHINE_CODE_RE` | `{6}$` | Test assertion regex | **Fixed** |
| `tools/production_full_test/bootstrap_test_data.py` | Uses `production_machine_code()` | Bootstrap chain | Inherits fix |
| `tools/production_full_test/run_machine_code_activation_prod.py` `ACTIVATION_CODE_RE` | `{6}$` | Isolated activation smoke machine | Already correct |
| `internal/app/activation/machineref.go` | `^AVF[0-9]{6}$` | Activation admin routes | **Unchanged** |
| `internal/app/machineruntime/overview.go` | `^AVF[0-9]{6,}$` | Fleet-wide normalization | **Unchanged** |
| `internal/app/fleet/service.go` | `{6,}$` error message | Fleet create/update | **Unchanged** |
| Production DB rows | Mixed (6+ digits possible) | Live fleet | **Audit only — no migration** |

---

## Divergence (intentional)

| Surface | Accepts |
|---------|---------|
| Activation admin (`/v1/admin/machine-codes/{code}/…`) | Exactly 6 digits |
| Fleet machine create (`POST /v1/admin/machines`) | 6 or more digits |

Machines with codes longer than six digits remain valid in fleet but cannot be addressed via activation admin machine-code paths until shortened or aliased.

---

## Test coverage

- `tools/production_full_test/test_production_machine_code.py` — asserts `production_machine_code()` matches `^AVF[0-9]{6}$` over 100 samples.

---

## References

- Prior workaround: activation smoke creates isolated 6-digit machine ([07_RETEST_AND_FIX_REPORT.md](07_RETEST_AND_FIX_REPORT.md))
- REST evidence of fleet reject for malformed bootstrap codes: `reports/production-full-api-grpc-mqtt/20260706T034900Z/rest_evidence/POST_v1_admin_machines.json`
