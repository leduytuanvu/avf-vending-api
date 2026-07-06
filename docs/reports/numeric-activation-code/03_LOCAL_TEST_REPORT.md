# Numeric Activation Code — Local Test Report

**Date:** 2026-07-06  
**Environment:** Windows, Go toolchain, `-short` mode (no TEST_DATABASE_URL integration)

## Commands and results

| Command | Exit code | Result |
|---------|-----------|--------|
| `go test ./internal/app/activation -count=1 -short` | 0 | PASS (2.114s) |
| `go test ./internal/httpserver -count=1 -short` | 0 | PASS (2.066s) |
| `go test ./internal/grpcserver -count=1 -short` | 0 | PASS (2.477s) |
| `go test ./... -count=1 -short` | 0 | PASS (all packages) |
| `python tools/build_openapi.py` | 0 | swagger.json + docs.go regenerated |
| `python tools/build_postman_collection.py` | 1 | FAIL — missing postman/scripts in workspace |

## Notes

- Integration tests requiring `TEST_DATABASE_URL` are skipped in `-short` mode per project convention.
- Unit tests in `code_format_test.go` cover all specified valid/invalid cases.
- Build fix applied: added `context` import to `activation_http.go` and `activation_claim_http_test.go`.

## Pre-commit status

Ready for branch `fix/numeric-six-digit-activation-code` and PR to `develop`.
