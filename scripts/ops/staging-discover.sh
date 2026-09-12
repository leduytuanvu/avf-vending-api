#!/usr/bin/env bash
# Discover staging deployment without local .env.staging.
# Usage:
#   STAGING_HOST=... STAGING_SSH_USER=... bash scripts/ops/staging-discover.sh
#   bash scripts/ops/staging-discover.sh --dns-only
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
EVIDENCE_DIR="${ROOT}/.db-destroy-evidence"
TS="$(date -u +%Y%m%dT%H%M%SZ)"
OUT="${EVIDENCE_DIR}/staging-discover-${TS}.log"
DNS_ONLY=0

fail() {
	echo "staging-discover: error: $*" >&2
	exit 1
}

note() {
	echo "staging-discover: $*"
	echo "staging-discover: $*" >>"${OUT}"
}

while [[ $# -gt 0 ]]; do
	case "$1" in
	--dns-only) DNS_ONLY=1; shift ;;
	-h | --help)
		echo "usage: bash scripts/ops/staging-discover.sh [--dns-only]"
		exit 0
		;;
	*) fail "unknown argument: $1" ;;
	esac
done

mkdir -p "${EVIDENCE_DIR}"

STAGING_API_DOMAIN="${STAGING_API_DOMAIN:-staging-api.ldtv.dev}"
STAGING_MQTT_DOMAIN="${STAGING_MQTT_DOMAIN:-staging-mqtt.ldtv.dev}"

note "=== staging discovery ${TS} ==="

if command -v nslookup >/dev/null 2>&1; then
	note "dns ${STAGING_API_DOMAIN}:"
	nslookup "${STAGING_API_DOMAIN}" 2>&1 | tee -a "${OUT}" || true
	note "dns ${STAGING_MQTT_DOMAIN}:"
	nslookup "${STAGING_MQTT_DOMAIN}" 2>&1 | tee -a "${OUT}" || true
elif command -v dig >/dev/null 2>&1; then
	dig +short "${STAGING_API_DOMAIN}" | tee -a "${OUT}" || true
	dig +short "${STAGING_MQTT_DOMAIN}" | tee -a "${OUT}" || true
fi

if [[ "${DNS_ONLY}" -eq 1 ]]; then
	note "dns-only mode complete"
	echo "EVIDENCE=${OUT}"
	exit 0
fi

STAGING_HOST="${STAGING_HOST:-}"
STAGING_SSH_USER="${STAGING_SSH_USER:-root}"
STAGING_SSH_PORT="${STAGING_SSH_PORT:-22}"
STAGING_DEPLOY_ROOT="${STAGING_DEPLOY_ROOT:-/opt/avf-staging}"

[[ -n "${STAGING_HOST}" ]] || {
	note "STAGING_HOST not set — DNS discovery only; set STAGING_HOST for remote inventory"
	echo "EVIDENCE=${OUT}"
	exit 0
}

note "ssh target=${STAGING_SSH_USER}@${STAGING_HOST}:${STAGING_DEPLOY_ROOT}"

remote_cmd() {
	ssh -o StrictHostKeyChecking=accept-new -p "${STAGING_SSH_PORT}" \
		"${STAGING_SSH_USER}@${STAGING_HOST}" "$@"
}

remote_cmd "set -euo pipefail
echo '=== hostname ==='
hostname
echo '=== deploy root ==='
ls -la '${STAGING_DEPLOY_ROOT}' 2>&1 || ls -la /opt/avf-vending-api 2>&1 || true
echo '=== compose ps ==='
for root in '${STAGING_DEPLOY_ROOT}' /opt/avf-staging /opt/avf-vending-api; do
  if [[ -f \"\${root}/deployments/staging/docker-compose.staging.yml\" ]]; then
    cd \"\${root}/deployments/staging\"
    docker compose --env-file .env.staging -f docker-compose.staging.yml ps 2>&1 || true
    break
  fi
done
echo '=== volumes ==='
docker volume ls 2>&1 | grep -i staging || true
echo '=== env keys (redacted) ==='
for f in '${STAGING_DEPLOY_ROOT}/deployments/staging/.env.staging' /opt/avf-staging/deployments/staging/.env.staging /opt/avf-vending-api/deployments/staging/.env.staging; do
  if [[ -f \"\$f\" ]]; then
    echo \"file=\$f\"
    grep -E '^(DATABASE_URL|NATS_URL|MQTT_BROKER_URL|REDIS_|TEMPORAL_|OBJECT_STORAGE_|CLOUDINARY_|API_DOMAIN)=' \"\$f\" | sed 's/=.*/=<redacted>/' || true
    break
  fi
done
" 2>&1 | tee -a "${OUT}"

note "discovery complete"
echo "EVIDENCE=${OUT}"
