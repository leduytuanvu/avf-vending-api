#!/usr/bin/env bash
# Local validation for collect_deploy_slo_evidence.sh critical retry behavior.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
exec python3 "${ROOT}/tools/test_collect_deploy_slo_evidence_retry.py" "$@"
