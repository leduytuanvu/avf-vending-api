#!/usr/bin/env bash
# P1.4 — API contract gate (Git Bash on Windows, Linux, macOS).
# Mirrors `make api-contract-check`: sqlc drift, Swagger/OpenAPI validation + drift, Postman drift,
# buf lint + proto breaking + generated Go drift, machine gRPC docs vs protos.
#
# Usage (from repo root): bash scripts/api-contract-check.sh
# Extra args are forwarded to make (e.g. PROTO_BREAKING_AGAINST=...).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

echo "api-contract-check: repo root=${ROOT}"
echo "api-contract-check: invoking make api-contract-check ..."
exec make api-contract-check "$@"
