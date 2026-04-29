#!/usr/bin/env bash
# Optional telemetry + payment webhook + outbox load entrypoint. Defaults to DRY_RUN=true.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

DRY_RUN="${DRY_RUN:-true}"
EXECUTE_LOAD_TEST="${EXECUTE_LOAD_TEST:-false}"
LOAD_TEST_ENV="${LOAD_TEST_ENV:-staging}"
SCENARIO="${SCENARIO:-100}" # 100 | 500 | 1000 | telemetry_burst | payment_webhook_burst | reconnect_storm
CONFIRM_PROD_LOAD_TEST="${CONFIRM_PROD_LOAD_TEST:-false}"
API_BASE_URL="${API_BASE_URL:-http://localhost:8080}"
WEBHOOK_PATH="${WEBHOOK_PATH:-/v1/commerce/webhooks/mock}"
WEBHOOK_BURST_COUNT="${WEBHOOK_BURST_COUNT:-100}"

truthy() {
	case "$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')" in
	true | 1 | yes) return 0 ;;
	*) return 1 ;;
	esac
}

if [[ "${LOAD_TEST_ENV}" == "production" ]] && ! truthy "${CONFIRM_PROD_LOAD_TEST}"; then
	echo "telemetry_outbox_burst: refusing production run without CONFIRM_PROD_LOAD_TEST=true" >&2
	exit 1
fi
if truthy "${EXECUTE_LOAD_TEST}"; then
	DRY_RUN=false
fi

case "${SCENARIO}" in
100) preset=100x100 ;;
500) preset=500x200 ;;
1000 | telemetry_burst | reconnect_storm) preset=1000x500 ;;
payment_webhook_burst) preset="" ;;
*) echo "telemetry_outbox_burst: unknown SCENARIO=${SCENARIO}" >&2; exit 1 ;;
esac

echo "telemetry_outbox_burst plan:"
echo "  env=${LOAD_TEST_ENV} scenario=${SCENARIO} dry_run=${DRY_RUN}"
echo "  telemetry_preset=${preset:-none} webhook_burst_count=${WEBHOOK_BURST_COUNT}"
echo "  metrics=telemetry ingest latency/error, DB pool, Redis, outbox pending/oldest age, payment webhook duration"

if [[ -n "${preset}" ]]; then
	DRY_RUN="${DRY_RUN}" EXECUTE_LOAD_TEST="${EXECUTE_LOAD_TEST}" LOAD_TEST_ENV="${LOAD_TEST_ENV}" SCENARIO_PRESET="${preset}" \
		bash deployments/prod/scripts/telemetry_storm_load_test.sh
fi

if [[ "${SCENARIO}" != "payment_webhook_burst" ]]; then
	exit 0
fi

if truthy "${DRY_RUN}"; then
	exit 0
fi
command -v curl >/dev/null 2>&1 || { echo "telemetry_outbox_burst: curl required" >&2; exit 1; }

for i in $(seq 1 "${WEBHOOK_BURST_COUNT}"); do
	body="{\"id\":\"load-webhook-${i}\",\"type\":\"payment.captured\",\"amount_minor\":100,\"currency\":\"THB\",\"created_at\":\"$(date -u +%FT%TZ)\"}"
	curl -fsS -X POST "${API_BASE_URL%/}${WEBHOOK_PATH}" \
		-H "content-type: application/json" \
		-d "${body}" >/dev/null &
	if (( i % 50 == 0 )); then
		wait
	fi
done
wait
echo "telemetry_outbox_burst: webhook burst submitted"
