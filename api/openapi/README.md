# OpenAPI (canonical generated artifacts)

**Canonical location:** [`../../docs/swagger/`](../../docs/swagger/)

OpenAPI 3.0 JSON and the Go embed package live under `docs/swagger/` because `internal/httpserver` imports `github.com/avf/avf-vending-api/docs/swagger` with `//go:embed swagger.json`. Moving embed paths would require Go import changes.

| File | Role |
|------|------|
| `docs/swagger/swagger.json` | Generated OpenAPI 3.0 spec (CI-checked) |
| `docs/swagger/docs.go` | Go embed + swag registration |

Regenerate: `make swagger` or `python tools/build_openapi.py`  
Validate: `make swagger-check` or `python tools/openapi_verify_release.py`

Contract gate: `make api-contract-check` / `scripts/openapi/api-contract-check.sh`
