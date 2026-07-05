# Moved

Canonical Postman assets:

## JSON v2.1 (Newman / CI)

- [`../../postman/collections/`](../../postman/collections/)
- [`../../postman/environments/`](../../postman/environments/)

Regenerate: `make postman-generate-json` (after `make swagger`).

## YAML v3 (Postman Local Mode)

- [`../../postman/v3/`](../../postman/v3/)

Regenerate: `make postman-generate-v3` (requires Postman CLI).

Validate both: `make postman-check`

Production full suite (JSON): [`../../postman/suites/production-full/`](../../postman/suites/production-full/)  
Production full suite (v3): [`../../postman/v3/suites/production-full/`](../../postman/v3/suites/production-full/)

Import guide: [`POSTMAN_V3_LOCAL_MODE_IMPORT_GUIDE.md`](POSTMAN_V3_LOCAL_MODE_IMPORT_GUIDE.md)  
Operator runbook: [`../runbooks/postman.md`](../runbooks/postman.md)
