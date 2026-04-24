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
```

On Windows PowerShell, if `python3` is missing: `PY=python make swagger` (see root `Makefile`).

## Tests

```bash
go test ./internal/httpserver ./internal/config
go test ./...
```
