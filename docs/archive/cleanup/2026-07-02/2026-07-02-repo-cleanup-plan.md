# 2026-07-02 repository cleanup plan

> **Note (2026-07-04):** `repomix.config.json` was removed from the repo (PR #416). References below are historical snapshots.

**Branch:** `cleanup/repo-structure-and-junk-files`  
**Baseline commit:** `c69a5995df3b7fc2298da32504cee56549824213`

Related: [baseline](2026-07-02-repo-cleanup-baseline.md) · [inventory](2026-07-02-repo-cleanup-inventory.md)

---

## 1. Executive summary

Restore CI-critical Postman paths broken by c69a5995, remove committed `_deploy_artifacts/` local deploy evidence, relocate the new production full Postman suite to a clear subdirectory, archive completed investigation reports, update documentation indexes, and verify build/test/contract gates. No migrations, workflows, or deployment scripts are deleted.

---

## 2. Current problems

1. **Postman CI regression** — `postman/environments/`, `postman/production/`, `postman/scripts/` deleted; CI and Makefile still require them.
2. **Committed deploy artifacts** — `_deploy_artifacts/` is a local Security Release snapshot with zero repo references.
3. **Stale documentation** — Postman README and scripts index out of date; completed reports still in active `docs/reports/`.

---

## 3. Proposed final structure (changes only)

```text
postman/
├── collections/              # unchanged
├── environments/             # RESTORED
├── production/               # RESTORED (manifest E2E generator)
├── scripts/                  # RESTORED
├── suites/production-full/   # MOVED from postman/avf-vending-production.full.*
└── README.md                 # UPDATED

docs/archive/reports/
├── cash-authority/           # ARCHIVED from docs/reports/
└── protocol-hardening/       # ARCHIVED from docs/reports/

_deploy_artifacts/            # DELETED from git; added to .gitignore
```

All other paths unchanged.

---

## 4. Exact deletes

| Path | Reason |
|------|--------|
| `_deploy_artifacts/README.md` | Local CI artifact; not source |
| `_deploy_artifacts/deploy-production-gh-command.sh` | Same |
| `_deploy_artifacts/production-deploy-candidate-metadata.json` | Same |
| `_deploy_artifacts/production-deploy-inputs.env` | Same |
| `_deploy_artifacts/production-deploy-inputs.json` | Same |
| `_deploy_artifacts/release-candidate/release-candidate.json` | Same |
| `_deploy_artifacts/security-verdict/security-verdict.json` | Same |

---

## 5. Exact restores and moves

| From | To | Method |
|------|-----|--------|
| `HEAD~1:postman/environments/` | `postman/environments/` | `git restore --source=HEAD~1` |
| `HEAD~1:postman/production/` | `postman/production/` | `git restore --source=HEAD~1` |
| `HEAD~1:postman/scripts/` | `postman/scripts/` | `git restore --source=HEAD~1` |
| `postman/avf-vending-production.full.postman_collection.json` | `postman/suites/production-full/` | `git mv` |
| `postman/avf-vending-production.full.postman_environment.json` | `postman/suites/production-full/` | `git mv` |
| `docs/reports/cash-authority/*` | `docs/archive/reports/cash-authority/` | `git mv` |
| `docs/reports/protocol-hardening/*` | `docs/archive/reports/protocol-hardening/` | `git mv` |

---

## 6. Docs to update

| File | Change |
|------|--------|
| `.gitignore` | Add `_deploy_artifacts/`, `prod-deploy-candidate/`, `production-deploy-candidate/` |
| `repomix.config.json` | Ignore `_deploy_artifacts/**` |
| `.gitattributes` | Add `postman/suites/**/*.json text eol=lf` |
| `postman/README.md` | Document restored paths + suites/production-full |
| `scripts/README.md` | Add `repair/`, `governance/` rows |
| `docs/README.md` | Link 2026-07 cleanup reports; archive pointer updates |
| `docs/reports/README.md` | Note archived cash-authority and protocol-hardening |

---

## 7. References to update after moves

```bash
rg -n "docs/reports/cash-authority|docs/reports/protocol-hardening|_deploy_artifacts|postman/avf-vending-production.full" .
```

Update any broken links found (primarily `docs/README.md`, `docs/reports/README.md`).

---

## 8. Risk per change

| Change | Risk | Mitigation |
|--------|------|------------|
| Delete `_deploy_artifacts/` | LOW | Zero external refs; CI generates dynamically |
| Restore Postman from HEAD~1 | LOW | Exact prior commit; run parity script |
| Move full suite JSON | LOW | git mv; README update |
| Archive reports | LOW | git mv; fix doc links |
| Workflow/deploy changes | NONE | Explicitly excluded |

---

## 9. Rollback plan

```bash
git checkout chore/sync-local-main-20260702
git branch -D cleanup/repo-structure-and-junk-files

# Per-path:
git restore --source=HEAD~1 -- postman/environments postman/production postman/scripts
git restore -- _deploy_artifacts/
git mv docs/archive/reports/cash-authority docs/reports/
git mv docs/archive/reports/protocol-hardening docs/reports/
```

---

## 10. Verification commands

```bash
git status --short
go test ./...
go vet ./...
make postman-check
bash scripts/ci/verify_production_postman_parity.sh
make check-migrations
make api-contract-check          # Git Bash / CI
make verify-workflows            # requires actionlint
make verify-enterprise-release   # Git Bash
rg -n "_deploy_artifacts" .      # must be empty or gitignore-only
```

---

## 11. DO NOT TOUCH

- `migrations/`, `db/queries/`, `db/schema/`
- `internal/gen/` (regenerate only via sqlc/buf)
- All `.github/workflows/*.yml` (especially `deploy-prod.yml`, `deploy-production.yml`)
- `deployments/prod/**`, `deployments/staging/**`, `deployments/docker/**`
- `docs/runbooks/**`
- Root wrapper scripts under `scripts/*.sh`
- Legacy compatibility endpoints / protobuf / MQTT topics

---

## 12. Requires owner confirmation (deferred)

- Track vs gitignore for `postman/suites/production-full/` long-term
- Bulk archive of remaining `docs/reports/verification/*`
- P2 internal package moves (`internal/modules/postgres`, transport splits)
- Align `generate_production_full_suite.py` OUT_DIR with new suite path

---

## 13. Implementation order

1. Baseline report (done)
2. Inventory report (done)
3. This plan (done)
4. Restore Postman CI paths + relocate full suite
5. Delete `_deploy_artifacts/` + gitignore
6. Archive stale reports
7. Update READMEs
8. Verification + final report
