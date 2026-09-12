#!/usr/bin/env bash
# Destructive EMQX purge: machine users, retained messages, stale sessions.
# Usage:
#   bash scripts/ops/emqx-purge.sh
#   bash scripts/ops/emqx-purge.sh --dry-run
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
LEGACY="${ROOT}/scripts/ops/emqx-audit-and-purge.sh"

fail() {
	echo "emqx-purge: error: $*" >&2
	exit 1
}

[[ -f "${LEGACY}" ]] || fail "missing ${LEGACY}"

args=(--purge)
while [[ $# -gt 0 ]]; do
	case "$1" in
	--dry-run) args+=(--dry-run); shift ;;
	-h | --help)
		echo "usage: bash scripts/ops/emqx-purge.sh [--dry-run]"
		exit 0
		;;
	*) fail "unknown argument: $1" ;;
	esac
done

exec bash "${LEGACY}" "${args[@]}"
