> **Relocated from avf-vending-system** — canonical home: `avf-vending-api/docs/reports/git-deploy/`. System copy removed 2026-06-16.

# PR Review #349 and #350

**UTC:** 2026-06-12  
**Repository:** `leduytuanvu/avf-vending-api` only  
**Out of scope:** PR #49, PR #50 (do not review or merge)

---

## PR #349 — Dependabot go-modules

| Field | Value |
|-------|-------|
| URL | https://github.com/leduytuanvu/avf-vending-api/pull/349 |
| Title | chore(deps): bump the go-modules group across 1 directory with 18 updates |
| State | OPEN |
| Base → Head | `main` ← `dependabot/go_modules/go-modules-c96e929d56` |
| mergeStateStatus | CLEAN |
| reviewDecision | APPROVED |
| Files | `go.mod`, `go.sum` only |
| + / − | +131 / −128 |

### Determination

| Question | Answer |
|----------|--------|
| Repository | `avf-vending-api` |
| In scope for metadata-repair workstream? | **No** — separate dependency bump to `main` |
| Overlap with local branch? | **No** |
| CI | All checks pass (`gh pr checks 349`) |
| Recommended action | **Document only — do not merge** as part of this workstream |

---

## PR #350 — Sellable layout + metadata repair branch

| Field | Value |
|-------|-------|
| URL | https://github.com/leduytuanvu/avf-vending-api/pull/350 |
| Title | test(e2e): sellable-layout + TCN cash-product setup scripts; hermetic bootstrap test |
| State | OPEN |
| Base → Head | `develop` ← `chore/e2e-sellable-layout-setup` |
| mergeStateStatus | UNSTABLE |
| reviewDecision | APPROVED |
| + / − | +2091 / −350 (prior push) |

### Key changed paths (prior PR head)

- `scripts/e2e/*` — layout schema, setup/verify scripts, examples
- `postman/production/avf-production-e2e.postman_collection.json` — modified (−312 lines in PR diff)
- `tests/e2e/production/*` — route matrix / run script updates

### Local additions (not yet on remote)

Metadata-repair commit will add:

- `scripts/repair/repair-machine-bootstrap-metadata.ps1`
- TCN cabinet metadata validation in `layout_config_schema.py` / apply script
- `scripts/e2e/tests/test-metadata-contract.ps1` + fixtures
- Updated `pilot-cabinet-layout-a.json` with contract keys

### CI status (`gh pr checks 350`)

| Check | Result |
|-------|--------|
| Go CI Gates | PASS |
| Linux race and contract gates | PASS |
| Workflow and Script Quality | PASS |
| Secret Scan | PASS |
| Migration Safety Check | PASS |
| Legacy Production Asset Contract | PASS |
| **Production E2E Postman parity** | **FAIL** |

Failure URL: https://github.com/leduytuanvu/avf-vending-api/actions/runs/27293322168/job/80619399688

Likely related to Postman collection / REST route matrix changes already in PR #350 head.

### Determination

| Question | Answer |
|----------|--------|
| Repository | `avf-vending-api` |
| In scope for metadata-repair workstream? | **Yes** — same branch as local work |
| Overlap with local dirty tree? | **Yes** |
| Merge ready? | **No** — Postman parity check failing |
| Recommended action | Push metadata-repair commit; investigate/fix Postman parity; merge to `develop` when all checks green |

---

## Verdict

| PR | Verdict |
|----|---------|
| #349 | `PR349_DOCUMENT_ONLY` — not merged in this workstream |
| #350 | `PR350_CI_BLOCKED` — merge blocked on Production E2E Postman parity until fixed |
