# Postman v3 YAML (Local Mode / Native Git)

These assets are **generated** from the canonical JSON v2.1 Postman files. Do not edit by hand.

## Layout

| Path | Purpose |
|------|--------|
| `collections/` | Primary, function-path, and production E2E collections (v3 folder format) |
| `environments/` | Local, staging, production, production-full, production-e2e environments |
| `suites/production-full/` | Full OpenAPI + gRPC/MQTT documentation suite |
| `manifest.json` | Generation metadata and parity counts |

## Regenerate

```bash
make postman-generate-v3    # v3 only (from current JSON)
make postman-generate       # JSON + production suites + v3
```

Requires **Postman CLI** on PATH (`npm install -g postman-cli`). On Windows, set `TEMP`/`TMP` to a directory on the same drive as the repo if migrate fails with `EXDEV`.

## Validate

```bash
make postman-check-v3
python tools/check_postman_v3_artifacts.py
python scripts/postman/validate_v3_yaml.py
```

Optional local lint (not required in CI):

```bash
postman collection lint postman/v3/collections/avf-vending-api --fail-severity warning
```

## Newman / CI

CI and Newman continue to use **JSON** under `postman/collections/` and `postman/environments/`.
Newman does not run v3 YAML collections; use Postman CLI `postman collection run <v3-folder>` instead.

Import guide: [`docs/postman/POSTMAN_V3_LOCAL_MODE_IMPORT_GUIDE.md`](../../docs/postman/POSTMAN_V3_LOCAL_MODE_IMPORT_GUIDE.md)
