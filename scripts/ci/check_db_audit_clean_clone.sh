#!/usr/bin/env bash
# Optional clean-clone acceptance gate for PR1 audit toolchain.
# Requires git worktree support and network for fetch (when not run in CI checkout).
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

fail() {
	echo "check_db_audit_clean_clone: $*" >&2
	exit 1
}

note() {
	echo "check_db_audit_clean_clone: $*"
}

BRANCH="${DB_AUDIT_CLEAN_CLONE_BRANCH:-$(git branch --show-current)}"
WORKTREE="${DB_AUDIT_CLEAN_CLONE_DIR:-/tmp/avf-audit-clean-clone}"

[[ -n "${BRANCH}" ]] || fail "could not determine branch"

note "branch=${BRANCH} worktree=${WORKTREE}"

if [[ -d "${WORKTREE}" ]]; then
	note "removing existing worktree ${WORKTREE}"
	git worktree remove --force "${WORKTREE}" 2>/dev/null || rm -rf "${WORKTREE}"
fi

git worktree add "${WORKTREE}" "${BRANCH}"
trap 'git worktree remove --force "${WORKTREE}" 2>/dev/null || true' EXIT

cd "${WORKTREE}"
bash scripts/ci/check_db_audit_readonly.sh
bash scripts/ops/tests/python3_shim.test.sh

note "OK: clean-clone audit gate passed"
