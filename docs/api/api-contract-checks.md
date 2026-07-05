# API contract checks (P1.4)

These gates prevent broken OpenAPI references, duplicate **operationId** values, stale generated protobuf Go code, stale **sqlc** output, stale Postman/OpenAPI artifacts, and drift between machine `.proto` services and `docs/api/machine-grpc.md`.

## Makefile targets

| Target | Purpose |
|--------|---------|
| `make swagger` | Regenerate `docs/swagger/swagger.json` and `docs/swagger/docs.go` from `cmd/api/main.go` + `internal/httpserver/swagger_operations.go`. Each route doc emits a stable **`operationId`** equal to the `DocOp*` function name. Legacy machine HTTP bridge routes get **`deprecated: true`** (see `LEGACY_MACHINE_REST_DEPRECATED` in `tools/build_openapi.py`). |
| `make swagger-check` | Runs generation, then `tools/openapi_verify_release.py` (local `$ref`, external `$ref`, **`components.securitySchemes.bearerAuth`**, duplicate operationIds, legacy-route deprecation, route-doc registry, servers, Bearer rules, examples), then `git diff` on `docs/swagger/` — fails if committed Swagger is stale. |
| `make postman-generate-json` | Regenerates Postman v2.1 collection/env under `postman/collections/` and `postman/environments/` (depends on `swagger`). |
| `make postman-generate-v3` | Regenerates Postman v3 YAML under `postman/v3/` from JSON (requires Postman CLI). |
| `make postman-generate` | Regenerates JSON, production suites, and v3 YAML. |
| `make postman-check-json` | Regenerates JSON artifacts and runs `tools/check_postman_artifacts.py`, then `git diff` on JSON paths. |
| `make postman-check-v3` | Runs v3 validators and `git diff` on `postman/v3/`. |
| `make postman-check` | **`postman-check-json`** + **`postman-check-v3`**. |
| `make proto-generate` | Runs `buf generate` for machine/public protos and internal query protos (`proto/buf.gen.yaml`, `proto/buf.gen.avfinternal.yaml`). |
| `make proto-check` | Lint + breaking-change check vs baseline + `git diff` on generated paths — fails if committed `.pb.go` / stubs drift. |
| `make sqlc` | Regenerate `internal/gen/db` from `db/queries` + schema (pinned sqlc via `SQLC_VERSION`). |
| `make sqlc-check` | Runs `sqlc generate` then `git diff` on `internal/gen/db/` — fails if sqlc output drifts from committed files. |
| `make machine-grpc-docs-check` | Generates [`android-proto-sync.md`](android-proto-sync.md), ensures `docs/api/machine-grpc.md` and [`machine-grpc-production-contract.md`](machine-grpc-production-contract.md) cover all services/flows. |
| `make api-contract-check` | **`sqlc-check`** + **`swagger-check`** + **`postman-check`** + **`proto-check`** + **`machine-grpc-docs-check`**. |

Wrapper script (same as `make api-contract-check`): `bash scripts/api-contract-check.sh`

### Windows / Git Bash

Use Git Bash or another POSIX shell for `bash scripts/...`. Python is invoked as **`PY`** (default `python3`); override if needed, e.g. `PY=python make swagger-check`.

## Python verification modules

- **`tools/openapi_refs.py`** — Local JSON Pointer resolution for `$ref`; duplicate **operationId** detection; forbidden non-local refs.
- **`tools/openapi_verify_release.py`** — Validates committed `docs/swagger/swagger.json` after generation in CI (`make swagger-check`).

## Tests

- **`make api-contract-test`** — `python -m unittest discover -s tests -p '*_test.py'` (includes `tests/api_contract_tools_test.py`).
- Go tests in **`internal/httpserver/openapi_spec_test.go`** mirror OpenAPI policies (embedded JSON validity, unresolved refs, Bearer examples).

## CI

GitHub Actions **Go CI Gates** runs `make api-contract-check` (with `PROTO_BREAKING_AGAINST` set from the PR base / previous commit). Failures surface as non-zero exit codes from generation drift (`git diff`) or verifier scripts.

## Fixing failures

| Symptom | What to do |
|--------|------------|
| Unresolved `$ref` / external `$ref` | Fix schema wiring in `tools/build_openapi.py` or stop hand-editing `swagger.json`; run `make swagger`, commit `docs/swagger/`. |
| Duplicate **operationId** | Ensure each `DocOp*` function name in `swagger_operations.go` is unique (they become OpenAPI **operationId**). |
| Missing **deprecated** on legacy machine REST | Align `LEGACY_MACHINE_REST_DEPRECATED` in `tools/build_openapi.py` with routes behind `machineLegacyRESTGuard`. |
| Route-doc registry | Add/update `DocOp*` blocks with `@Router` in `internal/httpserver/swagger_operations.go`; align `REQUIRED_OPERATIONS` when adding endpoints. |
| Proto drift | Run `make proto-generate`, commit outputs under `proto/...` and `internal/gen/avfinternalv1/`. |
| sqlc drift | Run `make sqlc`, commit `internal/gen/db/`. |
| Machine gRPC docs | Update `docs/api/machine-grpc.md` when adding/removing `service` definitions under `proto/avf/machine/v1/`. |
