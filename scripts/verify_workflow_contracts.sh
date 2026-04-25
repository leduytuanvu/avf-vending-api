#!/usr/bin/env bash
# Delegate to the canonical CI contract script (see .github/workflows/ci.yml).
set -Eeuo pipefail
exec "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/ci/verify_workflow_contracts.sh" "$@"
