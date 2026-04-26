#!/usr/bin/env bash
# Legacy single-host rollback wrapper: delegates to release.sh rollback
# using the last known good image refs (image-only; no DB schema undo).
# For enterprise incident evidence + digest preflight in GitHub Actions, see
# .github/workflows/rollback-prod.yml and docs/runbooks/production-rollback.md
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RELEASE_SH="${ROOT}/scripts/release.sh"

fail() {
	echo "rollback_prod: error: $*" >&2
	exit 1
}

legacy_banner() {
	cat >&2 <<'EOF'
================================================================
LEGACY SINGLE-HOST PRODUCTION PATH
NOT THE PRIMARY 2-VPS RELEASE PATH
This wrapper exists only for legacy/rollback/reference use.
Set ALLOW_LEGACY_SINGLE_HOST=1 to proceed intentionally.
================================================================
EOF
}

require_legacy_ack() {
	legacy_banner
	[[ "${ALLOW_LEGACY_SINGLE_HOST:-0}" == "1" ]] || fail "refusing to run legacy single-host rollback path without ALLOW_LEGACY_SINGLE_HOST=1"
}

require_legacy_ack
[[ -f "${ROOT}/.env.production" ]] || {
	fail "missing ${ROOT}/.env.production"
}
[[ -f "${RELEASE_SH}" ]] || {
	fail "missing ${RELEASE_SH}"
}

exec bash "${RELEASE_SH}" rollback "$@"
