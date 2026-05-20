#!/usr/bin/env bash
# UUID v7 static audit entrypoint (Phase 2 full-system verification).
# Delegates to scripts/checks/check-uuid-v7.sh — single source of truth for CI and local audits.
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

echo "verify-uuid-v7: running static UUID v7 audit..."
if bash "${ROOT}/scripts/checks/check-uuid-v7.sh"; then
	echo "verify-uuid-v7: PASS"
	exit 0
fi

echo "verify-uuid-v7: FAIL — see docs/architecture/UUID_V7_POLICY.md" >&2
exit 1
