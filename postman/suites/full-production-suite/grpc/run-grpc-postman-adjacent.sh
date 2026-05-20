#!/usr/bin/env bash
# Adjacent gRPC runner — delegates to repo E2E harness (grpcurl).
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$ROOT"
exec bash tests/e2e/run-grpc-local.sh "$@"
