# Postman scripts

## JSON generation

- `generate_collection.sh` — regenerate `postman/collections/` and `postman/environments/` (`tools/build_postman_collection.py`)
- `check_artifacts.sh` — validate JSON (`tools/check_postman_artifacts.py`)

## v3 YAML generation

- `generate_v3_yaml.py` — wrapper for `tools/build_postman_v3_yaml.py`
- `validate_v3_yaml.py` — deep JSON↔YAML parity

## Shared library

- `postman_openapi_lib.py` — OpenAPI/proto/MQTT Postman builder (tracked; replaces gitignored `postman/generated/generate_full_postman.py`)
- `generate_production_full_suite.py` — writes `postman/suites/production-full/*.json`

## Make targets

```bash
make postman-generate-json   # JSON CI collections + envs
make postman-generate-v3     # YAML v3 from current JSON (Postman CLI)
make postman-generate        # JSON + production suites + v3
make postman-check           # JSON + v3 drift gates
```

Wrappers at repo root: `scripts/generate_postman_collection.sh`, `scripts/check_postman_artifacts.sh`.
