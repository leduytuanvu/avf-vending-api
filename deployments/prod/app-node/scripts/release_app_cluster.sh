#!/usr/bin/env bash
set -Eeuo pipefail

NODE_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SHARED_ROOT="$(cd "${NODE_ROOT}/../shared" && pwd)"

exec bash "${SHARED_ROOT}/scripts/release_app_cluster.sh" "${1-}" "${2-}"
