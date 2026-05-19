#!/usr/bin/env bash
# Thin wrapper: tools/generate_release_manifest.py (release manifest + summary).
set -Eeuo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
exec python3 "${ROOT}/tools/generate_release_manifest.py" "$@"
