# Project Cleanup After Numeric Activation — Audit

**Date:** 2026-07-06

## Scan commands

```powershell
powershell -File scripts/local/clean-local-artifacts.ps1
git status --ignored --short | Select-Object -First 30
```

## Candidates found

| Path | Type | Safe to delete? | Reason |
|------|------|-----------------|--------|
| `docs/reports/numeric-activation-code/evidence/full_suite_run.log` | Local test log | Yes | Generated during production suite run; not referenced by CI |
| `repomix-output-avf-vending-api.xml` | Local repomix export | Yes | Large scratch export; gitignored pattern |

## Not deleted (protected)

- migrations, db schema, sqlc, proto, swagger, postman, CI workflows
- `docs/reports/numeric-activation-code/*.md` official evidence
- `reports/production-full-api-grpc-mqtt/` (production evidence; tracked or regenerated)

## Confusion check

No `activationCode` vs `machineCode` conflation in cleanup scope.
