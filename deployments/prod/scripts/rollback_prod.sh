#!/usr/bin/env bash
# Thin wrapper: delegates to release.sh rollback using the last known good image refs
# (image-only; no DB schema undo).
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RELEASE_SH="${ROOT}/scripts/release.sh"

[[ -f "${ROOT}/.env.production" ]] || {
	echo "rollback_prod: error: missing ${ROOT}/.env.production" >&2
	exit 1
}
[[ -f "${RELEASE_SH}" ]] || {
	echo "rollback_prod: error: missing ${RELEASE_SH}" >&2
	exit 1
}

exec bash "${RELEASE_SH}" rollback "$@"
