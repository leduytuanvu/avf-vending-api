#!/usr/bin/env bash
# Nightly image candidate resolver entrypoint (Nightly Security Rescan).
# Resolves digest-pinned app/goose refs from recent successful "Build and Push Images" on main
# using promotion-manifest (semantic source_event, artifact-first — CI workflow_run chain OK).
# Writes: ${OUT_DIR}/nightly-image-candidate.json
# Exit: 0 = ok, 2 = no candidate, 1 = unexpected Python failure.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OUT_DIR="${1:-nightly-reports}"
mkdir -p "${OUT_DIR}"
exec python3 "${ROOT}/scripts/security/resolve_nightly_main_image_candidate.py" --out-dir "${OUT_DIR}"
