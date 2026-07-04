# Git Merge / Deploy Report — Market Readiness Final

**UTC:** 20260704T002300Z

## Branch state

| Ref | SHA | Notes |
|-----|-----|-------|
| `origin/main` | `51485f5583a4f550cfe6fdb6e529e7339daad9ca` | Live production; PR #411 timeline hotfix |
| `origin/develop` | `8991f526d883fd6cfbc996ba5a4affbd558ae02d` | Missing timeline SQL fix |
| Local branch | `docs/runtime-fleet-prod-verify` | Harness + market-readiness work in progress |

## develop..main diff

```
db/queries/machine_ops_timeline.sql
internal/gen/db/machine_ops_timeline.sql.go
```

**Parity:** NOT empty — merge hotfix from `main` → `develop` before claiming gate 13.

## PRs

| PR | Status | Purpose |
|----|--------|---------|
| #411 | Merged to main | Timeline unified 500 fix |
| #412 | Open → develop | Runtime-fleet reports + prior harness |

## Deploy

| Item | Value |
|------|-------|
| Last runtime deploy | Run `28686916171` @ `277a3ad4`, migrations 00017/00018 |
| Timeline hotfix deploy | Run `28688099702` @ `51485f55` |
| Market readiness deploy | **Not required** — harness/docs only in this session |
| Live `/version` | Matches `origin/main` |

## Recommended merge sequence

1. Branch `feature/market-readiness-final` from updated `develop` (after hotfix back-merge)
2. PR → `develop` (harness + docs)
3. PR `develop` → `main`
4. Verify empty `git diff origin/develop..origin/main`
5. Deploy only if product code changes ship (none in this harness-only session)

## Post-deploy smoke (when product changes)

Minimal subset: `/health/live`, `/health/ready`, `/version`, one market gap matrix pass.
