#!/usr/bin/env bash
# Ensure run_remote_script keeps GHCR credentials out of remote SSH argv.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../../.." && pwd)"
LIB="${ROOT}/deployments/prod/shared/scripts/lib_release.sh"
CAPTURE_DIR="$(mktemp -d)"
trap 'rm -rf "${CAPTURE_DIR}"' EXIT

SENTINEL="TEST_SECRET_DO_NOT_EXPOSE_123"
CAPTURED_ARGV="${CAPTURE_DIR}/argv.txt"
CAPTURED_STDIN="${CAPTURE_DIR}/stdin.txt"

ssh() {
	printf '%s' "$*" >"${CAPTURED_ARGV}"
	cat >"${CAPTURED_STDIN}"
	return 0
}

# shellcheck source=/dev/null
source "${LIB}"

GHCR_PULL_USERNAME="ci-user"
GHCR_PULL_TOKEN="${SENTINEL}"
SSH_OPTS=""

run_remote_script "example-host" "/opt/app" "scripts/release_app_node.sh" "app@sha" "goose@sha" "1" "0" ""

argv="$(cat "${CAPTURED_ARGV}")"
stdin="$(cat "${CAPTURED_STDIN}")"

if [[ "${argv}" == *"${SENTINEL}"* ]]; then
	echo "FAIL: sentinel token appeared in ssh argv: ${argv}" >&2
	exit 1
fi

if [[ "${argv}" != *"bash -s -- /opt/app scripts/release_app_node.sh app@sha goose@sha"* ]]; then
	echo "FAIL: unexpected ssh argv: ${argv}" >&2
	exit 1
fi

if [[ "${stdin}" != *"${SENTINEL}"* ]]; then
	echo "FAIL: sentinel token missing from ssh stdin payload" >&2
	exit 1
fi

echo "PASS  run_remote_script keeps GHCR token out of ssh argv"
echo "PASS  run_remote_script delivers GHCR token via stdin only"
