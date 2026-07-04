# Develop / Main Parity Report

**UTC:** 20260704T012500Z (updated after PR #413 merge)

## Result

| Check | Status |
|-------|--------|
| PR #411 timeline hotfix on `origin/develop` | **PASS** — `51485f55` is ancestor of `origin/develop` |
| Commits on `main` missing from `develop` | **None** — `git log origin/develop..origin/main` empty |
| Timeline SQL parity | **PASS** — no diff on `machine_ops_timeline.sql` / generated Go |
| Remote `develop` == remote `main` (full tree) | **Pending** — develop ahead with market-readiness harness (PR #413) |

## SHAs (post PR #413)

| Ref | SHA |
|-----|-----|
| `origin/main` | `51485f5583a4f550cfe6fdb6e529e7339daad9ca` |
| `origin/develop` | `25326b71b72f2d0f2adc47caf81bd4774cf71893` |
| Live `/version` git_sha | `51485f5583a4f550cfe6fdb6e529e7339daad9ca` |

## Actions completed

1. Merged [PR #413](https://github.com/leduytuanvu/avf-vending-api/pull/413) (`feature/market-readiness-final` → `develop`) at `2026-07-04T01:24:27Z`
2. CI green on PR #413 (Go CI Gates, Linux race/contract, Security scans, Postman parity)
3. PR #411 hotfix included in develop via feature branch base on `main`

## Verification commands

```bash
git fetch origin main develop
git log origin/develop..origin/main          # expect empty
git merge-base --is-ancestor origin/main origin/develop   # expect 0
git diff origin/main origin/develop -- db/queries/machine_ops_timeline.sql internal/gen/db/machine_ops_timeline.sql.go  # expect empty
```

## Next step for full tree parity

Promote `develop` → `main` via PR (harness-only; no product runtime change beyond what is already on main).
