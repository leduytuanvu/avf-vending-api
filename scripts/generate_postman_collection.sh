#!/usr/bin/env bash
# Backwards-compatible wrapper — canonical: scripts/postman/generate_collection.sh
set -euo pipefail
exec "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/postman/generate_collection.sh" "$@"
