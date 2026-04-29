#!/usr/bin/env bash
# Operator entrypoint for P2.4 field-readiness checks. Dry-run by default.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

SCENARIO="${SCENARIO:-100}" # smoke | 100 | 500 | 1000 | reconnect_storm | payment_webhook_burst | telemetry_burst
DRY_RUN="${DRY_RUN:-true}"
EXECUTE_LOAD_TEST="${EXECUTE_LOAD_TEST:-false}"
LOAD_TEST_ENV="${LOAD_TEST_ENV:-staging}"

echo "field_readiness_suite:"
echo "  scenario=${SCENARIO} env=${LOAD_TEST_ENV} dry_run=${DRY_RUN} execute=${EXECUTE_LOAD_TEST}"
echo "  normal go test pipeline does not call this script"

case "${SCENARIO}" in
smoke)
	SCENARIO=smoke DRY_RUN="${DRY_RUN}" EXECUTE_LOAD_TEST="${EXECUTE_LOAD_TEST}" LOAD_TEST_ENV="${LOAD_TEST_ENV}" bash scripts/load/machine_grpc_smoke.sh
	DRY_RUN="${DRY_RUN}" EXECUTE_LOAD_TEST="${EXECUTE_LOAD_TEST}" LOAD_TEST_ENV="${LOAD_TEST_ENV}" bash scripts/load/mqtt_command_ack_smoke.sh
	SCENARIO=100 DRY_RUN="${DRY_RUN}" EXECUTE_LOAD_TEST="${EXECUTE_LOAD_TEST}" LOAD_TEST_ENV="${LOAD_TEST_ENV}" bash scripts/load/telemetry_outbox_burst.sh
	;;
100 | 500 | 1000 | reconnect_storm | payment_webhook_burst | telemetry_burst)
	SCENARIO="${SCENARIO}" DRY_RUN="${DRY_RUN}" EXECUTE_LOAD_TEST="${EXECUTE_LOAD_TEST}" LOAD_TEST_ENV="${LOAD_TEST_ENV}" bash scripts/load/machine_grpc_smoke.sh
	SCENARIO="${SCENARIO}" DRY_RUN="${DRY_RUN}" EXECUTE_LOAD_TEST="${EXECUTE_LOAD_TEST}" LOAD_TEST_ENV="${LOAD_TEST_ENV}" bash scripts/load/telemetry_outbox_burst.sh
	;;
*)
	echo "run_field_readiness_suite: unknown SCENARIO=${SCENARIO}" >&2
	exit 1
	;;
esac

echo "field_readiness_suite: complete"
