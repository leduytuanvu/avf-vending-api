#!/usr/bin/env bash
set -Eeuo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"
eval "$(./scripts/e2e/mint-production-machine-token.sh "${1:-019e702c-11c6-7ab0-89c7-5eb32f0b12cb}")"
exec ./scripts/e2e/production-canary-vend-evidence-grpc.sh
