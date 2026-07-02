# Repository Discovery — avf-vending-api

**Timestamp:** 20260702T184910Z  
**Repository:** `https://github.com/leduytuanvu/avf-vending-api.git`

## Summary

| Field | Value |
|-------|-------|
| Current branch | `develop` |
| Working tree | **dirty** |
| develop SHA | `f191db94cdcbabe71eb956419fac3ac3a76a1f10` (= origin/develop) |
| main SHA | `75e86628bb1b5ac283c8a9373f2ac3f058bfaee8` (= origin/main) |
| Release-worthy | **Yes** — enterprise flow backend implementation |
| Secret risk | **Low** — Postman env uses placeholders; no `.env` in diff |

## Modified files (24)

Production/schema/docs/swagger/postman/tooling changes for enterprise machine lifecycle, activation/reattach, runtime sessions, OpenAPI parity, migration 00016.

## Untracked files (~50+)

New Go/SQL/migration/tests, `tools/enterprise_flow/`, `reports/enterprise-flow-verification/20260703T013119Z/`.

## Recent commits

```
f191db94 Merge pull request #391 from leduytuanvu/main
75e86628 docs(cleanup): add main/develop sync report for 2026-07-02 (#390)
```

## Verdict

**Proceed with release** on `develop`, then merge to `main`.
