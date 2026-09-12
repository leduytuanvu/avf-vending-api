#!/usr/bin/env bash
# Append-only JSONL evidence ledger for ops wipe/audit runs.
set -Eeuo pipefail

OPS_EVIDENCE_FILE="${OPS_EVIDENCE_FILE:-}"
OPS_EVIDENCE_OPERATOR="${OPS_EVIDENCE_OPERATOR:-${USER:-unknown}}"
OPS_EVIDENCE_SHA="${OPS_EVIDENCE_SHA:-$(git -C "${OPS_EVIDENCE_REPO_ROOT:-.}" rev-parse HEAD 2>/dev/null || echo unknown)}"

ops_evidence_init() {
	local path="$1"
	OPS_EVIDENCE_FILE="${path}"
	mkdir -p "$(dirname "${OPS_EVIDENCE_FILE}")"
}

ops_evidence_append() {
	local event="$1"
	local detail="${2:-}"
	[[ -n "${OPS_EVIDENCE_FILE}" ]] || return 0
	# shellcheck source=redact.sh
	local lib_dir
	lib_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
	# shellcheck source=redact.sh
	source "${lib_dir}/redact.sh"
	local safe_detail
	safe_detail="$(printf '%s' "${detail}" | sed -E 's#(://)[^/@]+@#\1***@#g')"
	printf '{"ts":"%s","operator":"%s","sha":"%s","event":"%s","detail":"%s"}\n' \
		"$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
		"${OPS_EVIDENCE_OPERATOR}" \
		"${OPS_EVIDENCE_SHA}" \
		"${event}" \
		"${safe_detail}" >>"${OPS_EVIDENCE_FILE}" 2>/dev/null || true
}
