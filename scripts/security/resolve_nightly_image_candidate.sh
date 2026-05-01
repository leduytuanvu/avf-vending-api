#!/usr/bin/env bash
# Alias entrypoint for nightly image candidate resolution (same as resolve_latest_main_release_images.sh).
set -Eeuo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OUT_DIR="${1:-nightly-reports}"
mkdir -p "${OUT_DIR}"
exec python3 "${ROOT}/scripts/security/resolve_nightly_main_image_candidate.py" --out-dir "${OUT_DIR}"
