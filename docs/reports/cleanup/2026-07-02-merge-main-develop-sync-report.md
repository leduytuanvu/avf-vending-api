# 2026-07-02 merge main/develop sync report

**Date:** 2026-07-02

## 1. Starting branch

`cleanup/local-artifacts-and-tmp-files` @ `73a06c3b` (before tmp-artifact commit)

## 2. Commit SHA before work

| Ref | SHA |
|-----|-----|
| `origin/main` (pre-merge) | `3feef938` |
| `origin/develop` (pre-sync) | `51ccb8c5` |
| Local cleanup branch | `73a06c3b` |

## 3. Pre-work branch divergence

- `git log origin/main..origin/develop` — **empty** (develop had no unique commits)
- `git log origin/develop..origin/main` — **36 commits** (main ahead of develop)
- **Sync case:** Case A (fast-forward safe), executed via PR due to develop branch protection

## 4. Files changed (final PR #388 squash)

Consolidated cleanup across docs archive, Postman CI paths, `.gitignore`, cleanup scripts/reports, gitleaks allowlist, and production-full Postman suite. See [PR #388](https://github.com/leduytuanvu/avf-vending-api/pull/388).

## 5. Tests/checks run (local pre-commit)

| Command | Result |
|---------|--------|
| `go test ./... -short` | PASS |
| `go vet ./...` | PASS |
| `bash scripts/ci/verify_migrations.sh` | PASS |
| `bash scripts/ci/verify_workflow_contracts.sh` | PASS |
| `bash scripts/ci/verify_production_postman_parity.sh` | PASS |
| `python tools/check_postman_artifacts.py` | PASS |
| `python tools/check_markdown_links.py` | PASS |
| `make postman-check` / `make api-contract-check` | Not run (`make` unavailable on Windows Git Bash) |

## 6. Commits created on feature branch

| SHA | Message |
|-----|---------|
| `97f80b4e` | `chore(repo): clean local artifacts and repository cleanup reports` |
| `e8a3fe47` | `chore(security): allowlist Postman full suite openapiOperationId in gitleaks` |

(Squashed into single commit on `main` via PR #388)

## 7. Branch pushed

`cleanup/local-artifacts-and-tmp-files` → `origin/cleanup/local-artifacts-and-tmp-files`

## 8. Merge method for `main`

- **PR #388** — squash merge (superseded closed **PR #387**)
- URL: https://github.com/leduytuanvu/avf-vending-api/pull/388
- All CI checks green (including Secret Scan after gitleaks allowlist fix)

## 9. Sync method for `develop`

- Direct push rejected (develop branch rules: PR required, status checks)
- **PR #389** — `main` → `develop` merge commit
- URL: https://github.com/leduytuanvu/avf-vending-api/pull/389
- CI green on PR #389 before merge

## 10. Develop unique commits before sync

**None.** `origin/main..origin/develop` was empty throughout.

## 11. Final SHAs

| Ref | SHA |
|-----|-----|
| `origin/main` | `8af0198eedc02150c96bc5b5f9f8877da15507de` |
| `origin/develop` | `3c849bfb769bb28310f5ca911477cdd859fce384` |

## 12. Final tree hashes

| Ref | Tree |
|-----|------|
| `origin/main^{tree}` | `c7946860783aadea71dcb95beeade00721bf70cd` |
| `origin/develop^{tree}` | `c7946860783aadea71dcb95beeade00721bf70cd` |

## 13. Equality verification

```text
git diff origin/main..origin/develop  → empty
origin/main^{tree} == origin/develop^{tree}  → true
```

Commit SHAs differ (develop has merge commit for PR #389); **code trees are identical**.

## 14. Safety confirmations

- No production deploy was run
- No production DB mutation was run
- No secrets were introduced
- No force push was used
- PR #387 closed as superseded (not merged separately)
