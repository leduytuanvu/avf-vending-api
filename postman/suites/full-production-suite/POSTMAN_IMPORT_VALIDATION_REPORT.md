# Postman import validation report — AVF FULL 100

## Files

- `AVF_FULL_100.postman_collection.json` — Postman v2.1, **329** HTTP items.
- `AVF_FULL_100.postman_environment.json` — expanded variables (placeholders).

## Commands

```text
python -m json.tool postman/suites/full-production-suite/AVF_FULL_100.postman_collection.json
python -m json.tool postman/suites/full-production-suite/AVF_FULL_100.postman_environment.json
python postman/suites/full-production-suite/validate_generated_assets.py
```

## Forbidden literals scan (generator self-check)

- PASS (no matches)
