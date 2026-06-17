# Moved

Canonical Postman collections live under:

- [`../../postman/collections/`](../../postman/collections/)

Regenerate from OpenAPI: `make postman-generate` (after `make swagger`).

Environment JSON files are generated locally or imported from your CI/runtime secrets workflow — they are not checked in under `postman/environments/` in this branch. Typical names: `postman/environments/avf-local.postman_environment.json`, `avf-staging.postman_environment.json`, `avf-production.postman_environment.json`.

Operator guide: [`../runbooks/postman.md`](../runbooks/postman.md).
