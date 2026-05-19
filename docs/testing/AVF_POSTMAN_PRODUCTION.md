# Postman production suite (AVF vending API)

**Canonical FULL100 pack:** [`postman/full-production-suite/`](../../postman/full-production-suite/).

- **REST (import Postman):** [`AVF_FULL_100.postman_collection.json`](../../postman/full-production-suite/AVF_FULL_100.postman_collection.json)
- **Environment:** [`AVF_FULL_100.postman_environment.json`](../../postman/full-production-suite/AVF_FULL_100.postman_environment.json)
- **Legacy matrix build:** `AVF_REST_365_FULL.postman_collection.json` + `AVF_PRODUCTION.postman_environment.json` (same OpenAPI parity count)
- Generator: `python postman/full-production-suite/generate_full_postman_suite.py`
- Validator: `python postman/full-production-suite/validate_generated_assets.py`
- Zip pack: [`avf_full_100_postman_suite.zip`](../../postman/full-production-suite/avf_full_100_postman_suite.zip)

Execution order: [05_PRODUCTION_TEST_EXECUTION_ORDER.md](05_PRODUCTION_TEST_EXECUTION_ORDER.md)
