#!/usr/bin/env bash
# Fail CI if obvious fake-success / skeleton / nil-adapter patterns appear on production paths.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT"

if ! command -v rg >/dev/null 2>&1; then
  echo "ERROR: ripgrep (rg) is required for scripts/check_production_placeholders.sh" >&2
  exit 1
fi

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

echo "== production placeholder scan (rg) =="

# Nil payment / refund adapters in process entrypoints or bootstrap (live wiring regression).
if rg -n --glob '*.go' --glob '!*_test.go' 'Gateway:\s*nil|RefundSink:\s*nil' cmd/ internal/bootstrap/; then
  fail "nil Gateway/RefundSink found under cmd/ or internal/bootstrap/"
fi

# Legacy skeleton list handlers that returned HTTP-200-shaped empty collections without capability errors.
if rg -n --glob '*.go' --glob '!*_test.go' 'Items:\s*\[\]map\[string\]any\{\s*\}' internal/app/api/; then
  fail "fake empty ListView pattern in internal/app/api (use explicit not-implemented errors, not empty success)"
fi

# Skeleton adapter structs must not ship in production API package (use real adapters or not_implemented types).
if rg -n --glob '*.go' --glob '!*_test.go' 'type\s+skeleton[A-Za-z0-9_]*\s+struct' internal/app/api/; then
  fail "skeleton* adapter types reintroduced under internal/app/api/"
fi

# httptest belongs in *_test.go only (no stub mounted routes in production packages).
if rg -l --glob '*.go' --glob '!*_test.go' 'net/http/httptest' cmd/ internal/bootstrap/ internal/httpserver/ internal/grpcserver/ internal/app/ internal/platform/; then
  fail "net/http/httptest imported from non-test .go (stub HTTP server in production path)"
fi

echo "OK: no blocked placeholder patterns matched."
