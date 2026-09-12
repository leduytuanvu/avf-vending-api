#!/usr/bin/env bash
# Record staging/production table-audit status when remote VPS access is unavailable locally.
# Usage:
#   bash scripts/ops/record-remote-audit-status.sh --environment staging|production --status blocked|pass|fail [--detail TEXT]
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
EVIDENCE_DIR="${ROOT}/.db-destroy-evidence"

ENVIRONMENT=""
STATUS=""
DETAIL=""

while [[ $# -gt 0 ]]; do
	case "$1" in
	--environment)
		ENVIRONMENT="${2:-}"
		shift 2
		;;
	--status)
		STATUS="${2:-}"
		shift 2
		;;
	--detail)
		DETAIL="${2:-}"
		shift 2
		;;
	*)
		echo "record-remote-audit-status: unknown argument: $1" >&2
		exit 1
		;;
	esac
done

[[ -n "${ENVIRONMENT}" && -n "${STATUS}" ]] || {
	echo "usage: record-remote-audit-status.sh --environment ENV --status blocked|pass|fail [--detail TEXT]" >&2
	exit 1
}

mkdir -p "${EVIDENCE_DIR}"
OUT="${EVIDENCE_DIR}/table-audit-${ENVIRONMENT}-$(date -u +%Y%m%dT%H%M%SZ).jsonl"

{
	printf '{"ts":"%s","environment":"%s","status":"%s","detail":"%s","runbook":"bash scripts/ops/run-table-data-audit.sh --environment %s"}\n' \
		"$(date -u +%Y-%m-%dT%H:%M:%SZ)" "${ENVIRONMENT}" "${STATUS}" "${DETAIL}" "${ENVIRONMENT}"
	case "${ENVIRONMENT}" in
	staging)
		printf '{"ts":"%s","note":"staging_vps","command":"cd /opt/avf-vending-api && bash scripts/ops/run-table-data-audit.sh --environment staging"}\n' \
			"$(date -u +%Y-%m-%dT%H:%M:%SZ)"
		printf '{"ts":"%s","note":"staging_wipe_if_needed","command":"CONFIRM_STAGING_DATA_WIPE=AVF-WIPE-STAGING-DATA BACKUP_PATH=/var/backups/... make staging-data-wipe"}\n' \
			"$(date -u +%Y-%m-%dT%H:%M:%SZ)"
		;;
	production)
		printf '{"ts":"%s","note":"production_appnode","command":"cd /opt/avf-vending-api && bash scripts/ops/run-table-data-audit.sh --environment production"}\n' \
			"$(date -u +%Y-%m-%dT%H:%M:%SZ)"
		printf '{"ts":"%s","note":"production_emqx","command":"cd /opt/avf-vending-api && bash scripts/ops/run-environment-data-wipe.sh --environment production --phase emqx"}\n' \
			"$(date -u +%Y-%m-%dT%H:%M:%SZ)"
		printf '{"ts":"%s","note":"production_wipe_if_needed","command":"CONFIRM_PRODUCTION_DATA_WIPE=I_UNDERSTAND_THIS_WIPES_PRODUCTION BACKUP_PATH=/var/backups/... make prod-data-wipe"}\n' \
			"$(date -u +%Y-%m-%dT%H:%M:%SZ)"
		;;
	esac
} >"${OUT}"

echo "record-remote-audit-status: wrote ${OUT}"
