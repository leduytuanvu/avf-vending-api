#!/usr/bin/env bash
# Independent read-only clean-slate audit (never destructive).
#
# Usage:
#   bash scripts/ops/run-clean-slate-audit.sh --environment production --component all
#   bash scripts/ops/run-clean-slate-audit.sh --environment development --component postgres,redis
set -Eeuo pipefail

case "$(uname -s 2>/dev/null)" in
MINGW* | MSYS*) export MSYS_NO_PATHCONV=1 ;;
esac

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OPS="${ROOT}/scripts/ops"
LIB="${OPS}/lib"
EVIDENCE_DIR="${ROOT}/.db-destroy-evidence"
DEPLOY_ROOT="${AVF_DEPLOY_ROOT:-${ROOT}}"

ENVIRONMENT=""
COMPONENTS="all"
TS="$(date -u +%Y%m%dT%H%M%SZ)"

fail() {
	echo "run-clean-slate-audit: error: $*" >&2
	exit 1
}

note() {
	echo "run-clean-slate-audit: $*"
}

usage() {
	cat <<EOF
usage: bash scripts/ops/run-clean-slate-audit.sh --environment ENV [--component LIST]

  --environment   development | staging | production
  --component     all | postgres | redis | emqx | nats | temporal | media | topology
                  Comma-separated list also accepted.

Read-only. Writes evidence to .db-destroy-evidence/
EOF
}

while [[ $# -gt 0 ]]; do
	case "$1" in
	--environment)
		ENVIRONMENT="${2:-}"
		shift 2
		;;
	--component)
		COMPONENTS="${2:-all}"
		shift 2
		;;
	-h | --help)
		usage
		exit 0
		;;
	*) fail "unknown argument: $1" ;;
	esac
done

[[ -n "${ENVIRONMENT}" ]] || fail "--environment is required"
case "${ENVIRONMENT}" in
development | staging | production) ;;
*) fail "unsupported environment: ${ENVIRONMENT}" ;;
esac

# shellcheck source=lib/redis_resolve.sh
source "${LIB}/redis_resolve.sh"

mkdir -p "${EVIDENCE_DIR}"
EVIDENCE_LOG="${EVIDENCE_DIR}/clean-slate-audit-${ENVIRONMENT}-${TS}.log"
exec > >(tee -a "${EVIDENCE_LOG}") 2>&1

note "start environment=${ENVIRONMENT} components=${COMPONENTS} evidence=${EVIDENCE_LOG}"

want_component() {
	local c="$1"
	[[ "${COMPONENTS}" == "all" ]] && return 0
	[[ ",${COMPONENTS}," == *",${c},"* ]]
}

load_env_for_audit() {
	case "${ENVIRONMENT}" in
	development)
		export APP_ENV=development
		export PAYMENT_ENV="${PAYMENT_ENV:-sandbox}"
		if [[ -z "${DATABASE_URL:-}" ]]; then
			export DATABASE_URL="postgres://postgres:postgres@127.0.0.1:15432/avf_vending?sslmode=disable"
		fi
		export REDIS_URL="${REDIS_URL:-redis://127.0.0.1:6379}"
		;;
	staging)
		export APP_ENV=staging
		ENV_FILE="${DEPLOY_ROOT}/deployments/staging/.env.staging"
		if [[ -f "${ENV_FILE}" ]]; then
			set -a
			# shellcheck disable=SC1090
			source "${ENV_FILE}"
			set +a
		elif [[ -n "${STAGING_DATABASE_URL:-}" ]]; then
			export DATABASE_URL="${STAGING_DATABASE_URL}"
		else
			note "staging env file missing locally — postgres/redis/nats audits may be SKIPPED"
		fi
		;;
	production)
		export APP_ENV=production
		ENV_FILE="${DEPLOY_ROOT}/deployments/prod/app-node/.env.app-node"
		[[ -f "${ENV_FILE}" ]] || fail "missing ${ENV_FILE} for production audit"
		set -a
		# shellcheck disable=SC1090
		source "${ENV_FILE}"
		set +a
		if [[ -z "${DATABASE_URL:-}" ]]; then
			export DATABASE_URL="$(grep -E '^DATABASE_URL=' "${ENV_FILE}" | tail -1 | cut -d= -f2- | tr -d '\r' | sed -E 's/[?&]default_query_exec_mode=[^&]*//g; s/[?&]pgbouncer=[^&]*//g; s/\?&/?/g; s/\?$//')"
		fi
		;;
	esac
}

FINAL_STATUS=0

audit_postgres() {
	note "=== audit postgres ==="
	if [[ -z "${DATABASE_URL:-}" ]]; then
		note "postgres: SKIPPED (no DATABASE_URL)"
		FINAL_STATUS=1
		return
	fi
	bash "${OPS}/run-table-data-audit.sh" --environment "${ENVIRONMENT}" || FINAL_STATUS=1
	bash "${OPS}/verify-goose-migration.sh" || FINAL_STATUS=1
}

audit_redis() {
	note "=== audit redis ==="
	if ! redis_configured; then
		note "redis: not configured — N/A"
		return
	fi
	if [[ "${ENVIRONMENT}" == "development" ]]; then
		export REDIS_CONTAINER="avf-redis"
	fi
	if [[ "${ENVIRONMENT}" == "production" ]]; then
		export REDIS_CONTAINER="$(docker ps --format '{{.Names}}' 2>/dev/null | grep -i redis | head -1 || true)"
	fi
	redis_audit_log
	local dbsize
	dbsize="$(redis_dbsize || echo ERROR)"
	if [[ "${dbsize}" != "0" ]]; then
		note "redis: FAIL DBSIZE=${dbsize}"
		FINAL_STATUS=1
	else
		note "redis: PASS DBSIZE=0"
	fi
}

audit_emqx() {
	note "=== audit emqx ==="
	if [[ "${ENVIRONMENT}" == "production" ]]; then
		note "emqx: run on data-node with AVF_DEPLOY_ROOT=/opt/avf-vending-api"
	fi
	bash "${OPS}/emqx-audit.sh" || FINAL_STATUS=1
}

audit_nats() {
	note "=== audit nats ==="
	if [[ -z "${NATS_URL:-}" ]]; then
		note "nats: SKIPPED (NATS_URL unset)"
		return
	fi
	bash "${OPS}/nats-audit-streams.sh" || FINAL_STATUS=1
}

audit_temporal() {
	note "=== audit temporal ==="
	if [[ "${TEMPORAL_ENABLED:-false}" != "true" && "${TEMPORAL_ENABLED:-false}" != "1" ]]; then
		note "temporal: N/A (TEMPORAL_ENABLED not true)"
		return
	fi
	note "temporal: ENABLED — manual workflow inventory required (tctl/temporal CLI)"
	FINAL_STATUS=1
}

audit_media() {
	note "=== audit media ==="
	local bucket="${OBJECT_STORAGE_BUCKET:-${S3_BUCKET:-}}"
	if [[ -n "${CLOUDINARY_CLOUD_NAME:-}" ]]; then
		note "cloudinary configured folder=${CLOUDINARY_FOLDER:-avf-vending/products} — list not automated in audit"
	fi
	if [[ -n "${bucket}" ]]; then
		if command -v aws >/dev/null 2>&1; then
			local count
			count="$(aws s3 ls "s3://${bucket}/" --recursive 2>/dev/null | wc -l | tr -d ' ')"
			note "s3 bucket=${bucket} object_count=${count}"
			[[ "${count}" -eq 0 ]] || FINAL_STATUS=1
		else
			note "s3 bucket=${bucket} — aws CLI missing, SKIPPED"
			FINAL_STATUS=1
		fi
	else
		note "media: no object storage bucket configured"
	fi
}

audit_topology() {
	note "=== audit topology ==="
	bash "${OPS}/staging-discover.sh" --dns-only || true
	note "registry=$(wc -l < "${OPS}/stateful_system_registry.json" 2>/dev/null || echo 0) lines"
}

load_env_for_audit

if want_component "postgres"; then audit_postgres; fi
if want_component "redis"; then audit_redis; fi
if want_component "emqx"; then audit_emqx; fi
if want_component "nats"; then audit_nats; fi
if want_component "temporal"; then audit_temporal; fi
if want_component "media"; then audit_media; fi
if want_component "topology"; then audit_topology; fi

if [[ "${FINAL_STATUS}" -eq 0 ]]; then
	note "RESULT=PASS"
else
	note "RESULT=FAIL"
fi

note "evidence=${EVIDENCE_LOG}"
exit "${FINAL_STATUS}"
