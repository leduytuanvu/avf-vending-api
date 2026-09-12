#!/usr/bin/env bash
# Destructive NATS JetStream purge for AVF-owned streams.
# Usage:
#   NATS_URL=nats://... bash scripts/ops/nats-purge-streams.sh
#   NATS_URL=nats://... bash scripts/ops/nats-purge-streams.sh --dry-run
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
LEGACY="${ROOT}/scripts/ops/nats-purge-avf-streams.sh"

fail() {
	echo "nats-purge-streams: error: $*" >&2
	exit 1
}

[[ -f "${LEGACY}" ]] || fail "missing ${LEGACY}"

args=(--purge)
while [[ $# -gt 0 ]]; do
	case "$1" in
	--dry-run) args+=(--dry-run); shift ;;
	-h | --help)
		echo "usage: NATS_URL=... bash scripts/ops/nats-purge-streams.sh [--dry-run]"
		exit 0
		;;
	*) fail "unknown argument: $1" ;;
	esac
done

exec bash "${LEGACY}" "${args[@]}"
