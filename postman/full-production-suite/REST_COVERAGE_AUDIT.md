# REST coverage audit — AVF FULL 100

| Check | Expected | Actual | Status |
|-------|----------|--------|--------|
| OpenAPI operations | 327 | 327 | PASS |
| Postman REST requests | 327 | 327 | PASS |
| gRPC template rows | 86 | 86 | PASS |
| MQTT flow rows | 28 | 28 | PASS |
| Forbidden-term scan (FULL100 json) | 0 hits | 0 | PASS |

## Empty URL / body audits

- Run `python postman/full-production-suite/validate_generated_assets.py` after generation.

## Destructive gate

- Writes outside `AUTH_PUBLIC_WRITE` require env gate flags (collection pre-request).
