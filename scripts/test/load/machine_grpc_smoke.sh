#!/usr/bin/env bash
# Optional machine gRPC readiness/load harness. Defaults to DRY_RUN=true and never requires grpcurl in normal dev.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

DRY_RUN="${DRY_RUN:-true}"
EXECUTE_LOAD_TEST="${EXECUTE_LOAD_TEST:-false}"
LOAD_TEST_ENV="${LOAD_TEST_ENV:-local}" # local | staging | production
GRPC_ADDR="${GRPC_ADDR:-localhost:9090}"
SCENARIO="${SCENARIO:-smoke}" # smoke | 100 | 500 | 1000 | reconnect_storm
MACHINE_JWT="${MACHINE_JWT:-}"
ACTIVATION_CODE="${ACTIVATION_CODE:-}"
REFRESH_TOKEN="${REFRESH_TOKEN:-}"
ORDER_ID="${ORDER_ID:-}"
PRODUCT_ID="${PRODUCT_ID:-}"
SLOT_INDEX="${SLOT_INDEX:-1}"
CONFIRM_PROD_LOAD_TEST="${CONFIRM_PROD_LOAD_TEST:-false}"

truthy() {
	case "$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')" in
	true | 1 | yes) return 0 ;;
	*) return 1 ;;
	esac
}

if [[ "${LOAD_TEST_ENV}" == "production" ]] && ! truthy "${CONFIRM_PROD_LOAD_TEST}"; then
	echo "machine_grpc_smoke: refusing production run without CONFIRM_PROD_LOAD_TEST=true" >&2
	exit 1
fi
if truthy "${EXECUTE_LOAD_TEST}"; then
	DRY_RUN=false
fi

machine_count=1
case "${SCENARIO}" in
smoke) machine_count=1 ;;
100) machine_count=100 ;;
500) machine_count=500 ;;
1000) machine_count=1000 ;;
reconnect_storm) machine_count="${RECONNECT_STORM_CLIENTS:-250}" ;;
*) echo "machine_grpc_smoke: unknown SCENARIO=${SCENARIO}" >&2; exit 1 ;;
esac

echo "machine_grpc_smoke plan:"
echo "  env=${LOAD_TEST_ENV} grpc=${GRPC_ADDR} scenario=${SCENARIO} machine_count=${machine_count} dry_run=${DRY_RUN}"
echo "  flows=activation/bootstrap/catalog, commerce cash sale, QR session status/webhook pairing (webhook sent by REST harness)"
echo "  metrics=p50/p95/p99 grpc latency, error rate, DB pool, Redis, outbox lag"

if truthy "${DRY_RUN}"; then
	exit 0
fi
command -v grpcurl >/dev/null 2>&1 || { echo "machine_grpc_smoke: grpcurl required when EXECUTE_LOAD_TEST=true" >&2; exit 1; }

if [[ -n "${ACTIVATION_CODE}" ]]; then
	grpcurl -plaintext \
		-d "{\"activation_code\":\"${ACTIVATION_CODE}\",\"device_fingerprint\":{\"serial_number\":\"load-${SCENARIO}\",\"android_id\":\"load-${SCENARIO}\"}}" \
		"${GRPC_ADDR}" avf.machine.v1.MachineAuthService/ClaimActivation
fi

if [[ -n "${REFRESH_TOKEN}" ]]; then
	grpcurl -plaintext -d "{\"refresh_token\":\"${REFRESH_TOKEN}\"}" \
		"${GRPC_ADDR}" avf.machine.v1.MachineAuthService/RefreshMachineToken
fi

if [[ -z "${MACHINE_JWT}" ]]; then
	echo "machine_grpc_smoke: MACHINE_JWT required for protected RPCs; activation-only run complete" >&2
	exit 0
fi

grpcurl -plaintext -H "authorization: Bearer ${MACHINE_JWT}" -d "{}" \
	"${GRPC_ADDR}" avf.machine.v1.MachineBootstrapService/GetBootstrap
grpcurl -plaintext -H "authorization: Bearer ${MACHINE_JWT}" -d "{\"include_unavailable\":false}" \
	"${GRPC_ADDR}" avf.machine.v1.MachineCatalogService/GetCatalogSnapshot
grpcurl -plaintext -H "authorization: Bearer ${MACHINE_JWT}" -d "{}" \
	"${GRPC_ADDR}" avf.machine.v1.MachineMediaService/GetMediaManifest

if [[ -n "${PRODUCT_ID}" ]]; then
	client_event="load-$(date +%s)-${RANDOM}"
	grpcurl -plaintext -H "authorization: Bearer ${MACHINE_JWT}" \
		-d "{\"idempotency\":{\"idempotency_key\":\"${client_event}\",\"client_event_id\":\"${client_event}\"},\"items\":[{\"product_id\":\"${PRODUCT_ID}\",\"slot_index\":${SLOT_INDEX},\"quantity\":1}]}" \
		"${GRPC_ADDR}" avf.machine.v1.MachineCommerceService/CreateOrder
fi

if [[ -n "${ORDER_ID}" ]]; then
	grpcurl -plaintext -H "authorization: Bearer ${MACHINE_JWT}" \
		-d "{\"order_id\":\"${ORDER_ID}\"}" \
		"${GRPC_ADDR}" avf.machine.v1.MachineCommerceService/GetOrderStatus
fi
