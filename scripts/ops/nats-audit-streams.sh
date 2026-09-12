#!/usr/bin/env bash
# Read-only audit of AVF-owned JetStream streams.
# Usage: NATS_URL=nats://... bash scripts/ops/nats-audit-streams.sh
set -Eeuo pipefail

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
	echo "nats-audit-streams: error: $*" >&2
	exit 1
}

note() {
	echo "nats-audit-streams: $*"
}

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

note "server=${NATS_URL%%@*}@<redacted>"

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
done

note "total_avf_stream_messages=${total_messages}"

if [[ "${total_messages}" -gt 0 ]]; then
	note "audit: FAIL — stale messages remain"
	exit 1
fi

note "audit: PASS"
