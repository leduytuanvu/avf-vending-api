#!/usr/bin/env bash
# Backwards-compatible wrapper — canonical: scripts/postman/check_artifacts.sh
set -euo pipefail
exec "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/postman/check_artifacts.sh" "$@"
