#!/usr/bin/env bash
# Audit and purge AVF-owned JetStream streams (read-only audit with --audit-only).
# Usage:
#   NATS_URL=nats://... bash scripts/ops/nats-purge-avf-streams.sh --audit-only
#   NATS_URL=nats://... bash scripts/ops/nats-purge-avf-streams.sh --purge
set -Eeuo pipefail

MODE="audit-only"
DRY_RUN=0

AVF_STREAMS=(
	AVF_INTERNAL_OUTBOX
	AVF_INTERNAL_DLQ
	AVF_TELEMETRY_HEARTBEAT
	AVF_TELEMETRY_STATE
	AVF_TELEMETRY_METRICS
	AVF_TELEMETRY_INCIDENTS
	AVF_TELEMETRY_COMMAND_RECEIPTS
	AVF_TELEMETRY_DIAGNOSTIC_READY
)

fail() {
	echo "nats-purge-avf-streams: error: $*" >&2
	exit 1
}

note() {
	echo "nats-purge-avf-streams: $*"
}

usage() {
	cat <<EOF
usage: bash scripts/ops/nats-purge-avf-streams.sh [--audit-only|--purge] [--dry-run]

Requires NATS_URL in environment.
EOF
}

while [[ $# -gt 0 ]]; do
	case "$1" in
	--audit-only) MODE="audit-only"; shift ;;
	--purge) MODE="purge"; shift ;;
	--dry-run) DRY_RUN=1; shift ;;
	-h | --help) usage; exit 0 ;;
	*) fail "unknown argument: $1" ;;
	esac
done

[[ -n "${NATS_URL:-}" ]] || fail "NATS_URL is required"

nats_cmd() {
	local server="${NATS_URL}"
	local ctn=""
	if docker ps --format '{{.Names}}' 2>/dev/null | grep -qi nats; then
		ctn="$(docker ps --format '{{.Names}}' | grep -i nats | head -1)"
		server="nats://127.0.0.1:4222"
	fi
	if [[ -n "${ctn}" ]]; then
		docker run --rm --network "container:${ctn}" natsio/nats-box:latest \
			nats --server "${server}" "$@"
	elif command -v nats >/dev/null 2>&1; then
		nats --server "${server}" "$@"
	else
		docker run --rm --network host natsio/nats-box:latest nats --server "${server}" "$@"
	fi
}

note "mode=${MODE} dry_run=${DRY_RUN} server=${NATS_URL%%@*}@<redacted>"

total_messages=0
for stream in "${AVF_STREAMS[@]}"; do
	if ! info="$(nats_cmd stream info "${stream}" 2>/dev/null)"; then
		note "stream=${stream} status=ABSENT"
		continue
	fi
	msgs="$(echo "${info}" | awk '/^State:/{f=1} f && /^[[:space:]]+Messages:/{print $2; exit}' | tr -d ',' || echo "?")"
	bytes="$(echo "${info}" | awk '/^State:/{f=1} f && /^[[:space:]]+Bytes:/{print $2; exit}' | tr -d ',' || echo "?")"
	note "stream=${stream} messages=${msgs} bytes=${bytes}"
	if [[ "${msgs}" =~ ^[0-9]+$ ]]; then
		total_messages=$((total_messages + msgs))
	fi

	if [[ "${MODE}" == "purge" ]]; then
		if [[ "${DRY_RUN}" -eq 1 ]]; then
			note "[dry-run] would purge stream ${stream}"
		else
			nats_cmd stream purge "${stream}" --force || fail "purge failed for ${stream}"
			note "purged stream=${stream}"
		fi
	fi
done

note "total_avf_stream_messages=${total_messages}"

if [[ "${MODE}" == "audit-only" && "${total_messages}" -gt 0 ]]; then
	note "audit: FAIL — stale messages remain"
	exit 1
fi

if [[ "${MODE}" == "purge" && "${DRY_RUN}" -eq 0 ]]; then
	for stream in "${AVF_STREAMS[@]}"; do
		if nats_cmd stream info "${stream}" >/dev/null 2>&1; then
			remaining="$(nats_cmd stream info "${stream}" 2>/dev/null | awk '/^State:/{f=1} f && /^[[:space:]]+Messages:/{gsub(/,/,"",$2); print $2; exit}')"
			remaining="${remaining:-0}"
			if ! [[ "${remaining}" =~ ^[0-9]+$ ]] || [[ "${remaining}" -ne 0 ]]; then
				fail "post-purge messages not zero for ${stream}: ${remaining}"
			fi
		fi
	done
	note "purge: PASS — all present AVF streams at 0 messages"
fi
