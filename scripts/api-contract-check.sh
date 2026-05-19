#!/usr/bin/env bash
# Backwards-compatible wrapper — canonical: scripts/openapi/api-contract-check.sh
set -euo pipefail
exec "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/openapi/api-contract-check.sh" "$@"
