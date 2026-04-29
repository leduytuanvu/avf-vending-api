# Swagger / OpenAPI — enterprise audit appendix

This file is for maintainers when **Agent mode** (or unrestricted edits) is available. **Plan mode** cannot modify `tools/build_openapi.py` or regenerate `docs/swagger/*`.

**Full enterprise Swagger UI upgrade (error schemas, public `security`, named examples, tags, `/version`, tests):** see [openapi-enterprise-upgrade-handoff.md](../runbooks/openapi-enterprise-upgrade-handoff.md).

## Already satisfied (verify on branch)

- **Production server first:** `tools/build_openapi.py` sets `servers[0].url` to `https://api.ldtv.dev` and asserts it in `main()`.
- **Bearer on protected routes:** `build_operation_oas3` sets `security: [{ bearerAuth: [] }]` when `@Security BearerAuth` is present; public webhook omits it.
- **Examples:** Generator uses documentation UUIDs (`_U`, `_U2`, …) — not production secrets.

## Recommended enhancements (optional patch)

1. **`@Deprecated` support** — In `build_operation_oas3`, after parsing `Tags`, if `@Deprecated true` appears in a `DocOp*` block, set `op["deprecated"] = True`. Use sparingly (only for routes scheduled for removal).

2. **Fallback classification (extensions)** — After building `paths`, annotate:
   - `POST /v1/device/machines/{machineId}/commands/poll` with `x-avf-client-classification: device_http_fallback` and `x-avf-primary-transport: mqtt`.
   - Optionally append to `description` that HTTP poll is **not** a high-volume substitute for MQTT.

3. **Tag copy** — Broaden the **Device** tag description in `main()`’s `tags` list to state HTTP poll is **fallback**, primary commands via MQTT.

4. **`cmd/api/main.go`** — Add an `@description` line pointing integrators to `docs/api/api-surface-audit.md` and `docs/api/kiosk-app-flow.md`.

## Regenerate and check

```bash
make swagger
make swagger-check
make postman-generate
make postman-check
make proto-generate
make proto-check
make api-contract-check
```

On Windows PowerShell, if `python3` is missing: `PY=python make swagger` (see root `Makefile`).

`make api-contract-check` is the aggregate local gate for API artifacts. It fails when OpenAPI has unresolved local `$ref` values, a required registered REST operation is missing from the Swagger operation docs policy, Postman artifacts are stale, protobuf files have breaking changes or generated drift, or `docs/api/machine-grpc.md` does not mention every `avf.machine.v1` service.

**Admin vs internal transports:** **`/v1/admin/*`** documents operator HTTPS + JSON. **`avf.machine.v1` gRPC** and loopback **`avf.internal.v1`** are **not** part of the Bearer-authenticated Admin Web contract; `openapi_spec_test.go` rejects OpenAPI paths exposing `/v1/internal…` or gratuitous `grpc` URL segments.

CI runs the same aggregate gate through `make api-contract-check` after sqlc drift checks, so OpenAPI, Postman, proto generation, and machine gRPC documentation drift fail in one place.

## Local testing

Git Bash:

```bash
curl -fsS http://localhost:8080/swagger/doc.json >/tmp/avf-openapi.json
python tools/build_openapi.py
python tools/openapi_verify_release.py
```

PowerShell:

```powershell
Invoke-WebRequest -UseBasicParsing http://localhost:8080/swagger/doc.json -OutFile .\avf-openapi.json
python tools/build_openapi.py
python tools/openapi_verify_release.py
```

For full local field smoke, see [field-smoke-tests.md](../runbooks/field-smoke-tests.md).

## Tests

```bash
make api-contract-test
go test ./internal/httpserver ./internal/config
go test ./...
```
