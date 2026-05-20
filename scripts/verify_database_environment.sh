#!/usr/bin/env bash
# Backwards-compatible wrapper — canonical: scripts/db/verify_database_environment.sh
set -euo pipefail
exec "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/db/verify_database_environment.sh" "$@"
