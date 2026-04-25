#!/usr/bin/env bash
# Validate docs/postman (thin wrapper; implementation in tools for Windows-friendly make postman-check).
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
PY="${PY:-python3}"
exec "${PY}" tools/check_postman_artifacts.py
