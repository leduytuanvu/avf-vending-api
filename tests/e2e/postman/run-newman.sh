#!/usr/bin/env bash
# shellcheck shell=bash
# Run Newman against POSTMAN_COLLECTION + POSTMAN_ENV; honor write and production guards.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
E2E_SCRIPT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
# shellcheck source=../lib/e2e_common.sh
source "${E2E_SCRIPT_DIR}/lib/e2e_common.sh"
e2e_strict_mode

load_env

: "${POSTMAN_COLLECTION:=postman/collections/avf-vending-api-function-path.postman_collection.json}"
: "${POSTMAN_ENV:=postman/environments/avf-local.postman_environment.json}"
: "${E2E_ALLOW_WRITES:=true}"
: "${E2E_TARGET:=local}"

COLL="${POSTMAN_COLLECTION}"
[[ "${COLL}" != /* ]] && COLL="${E2E_REPO_ROOT}/${COLL}"
ENVF="${POSTMAN_ENV}"
[[ "${ENVF}" != /* ]] && ENVF="${E2E_REPO_ROOT}/${ENVF}"

RUN_DIR="${E2E_RUN_DIR:-}"
if [[ -z "$RUN_DIR" ]]; then
  RUN_DIR="${E2E_REPO_ROOT}/.e2e-runs/newman-standalone-$$"
fi
mkdir -p "${RUN_DIR}/rest"

LOG="${RUN_DIR}/rest/newman-cli.log"
JSON_REPORT="${RUN_DIR}/rest/newman-report.json"
JUNIT_REPORT="${RUN_DIR}/rest/newman-junit.xml"

NEWMAN_REMEDIATION=(
  "Install Newman globally: npm install -g newman"
  "Or use npx: npx newman run ..."
  "See tests/e2e/README.md (Postman / Newman)"
)

log_newman_skip() {
  {
    echo "$(now_utc) newman not installed — skipping Postman CLI run"
    for line in "${NEWMAN_REMEDIATION[@]}"; do
      echo "  remediation: ${line}"
    done
  } | tee -a "$LOG"
}

if [[ ! -f "$COLL" ]]; then
  log_error "Postman collection not found: ${COLL} (set POSTMAN_COLLECTION)"
  exit 2
fi
if [[ ! -f "$ENVF" ]]; then
  log_error "Postman environment not found: ${ENVF} (set POSTMAN_ENV)"
  exit 2
fi

: >"$LOG"

if ! command -v newman >/dev/null 2>&1; then
  log_newman_skip
  if [[ -n "${E2E_EVENTS_FILE:-}" ]] || [[ -n "${E2E_RUN_DIR:-}" ]]; then
    append_event_jsonl "newman-cli" "skipped" "newman not on PATH — see ${LOG}"
  fi
  exit 0
fi

# Production: require same confirmation as other E2E writers; Newman collection prerequest also gates mutations.
if [[ "${E2E_TARGET}" == "production" ]] && [[ "${E2E_ALLOW_WRITES}" == "true" ]]; then
  if [[ "${E2E_PRODUCTION_WRITE_CONFIRMATION:-}" != "I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION" ]]; then
    log_error "Newman blocked: E2E_TARGET=production with E2E_ALLOW_WRITES=true requires E2E_PRODUCTION_WRITE_CONFIRMATION=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION"
    exit 2
  fi
fi

extra_args=()
if [[ "${E2E_ALLOW_WRITES}" != "true" ]]; then
  if jq -e '.item[] | select(.name == "Public")' "$COLL" >/dev/null 2>&1; then
    extra_args+=(--folder "Public")
    echo "$(now_utc) E2E_ALLOW_WRITES!=true — restricting Newman to folder Public only" >>"$LOG"
  else
    echo "$(now_utc) WARN: no Public folder in collection — running full collection (read-only mode not folder-scoped)" >>"$LOG"
  fi
else
  # Align Postman {{base_url}} with harness BASE_URL (avoid stale localhost:8080 in checked-in env JSON).
  if [[ -n "${BASE_URL:-}" ]]; then
    extra_args+=(--env-var "base_url=${BASE_URL}")
    extra_args+=(--env-var "baseUrl=${BASE_URL}")
  fi
  # Canary/admin-write folders gate POST /v1/admin/sites unless explicitly enabled.
  if [[ "${E2E_TARGET:-}" == "local" ]]; then
    extra_args+=(--env-var "allow_destructive=true")
  fi
  if [[ -n "${ADMIN_EMAIL:-}" ]]; then
    extra_args+=(--env-var "adminEmail=${ADMIN_EMAIL}")
    extra_args+=(--env-var "admin_email=${ADMIN_EMAIL}")
  fi
  if [[ -n "${ADMIN_PASSWORD:-}" ]]; then
    extra_args+=(--env-var "adminPassword=${ADMIN_PASSWORD}")
    extra_args+=(--env-var "admin_password=${ADMIN_PASSWORD}")
  fi
  extra_args+=(--env-var "auth_type=admin")
  extra_args+=(--env-var "allowGatedWrites=true")
  extra_args+=(--env-var "allow_destructive=true")
fi

# production-full: Newman cannot execute gRPC/MQTT folders 30–35; skip 00 runbook + 99 utilities.
if jq -e '.item[] | select(.name | startswith("01 System"))' "$COLL" >/dev/null 2>&1; then
  rest_folders=(
    "01 System - Health, Version, Metrics, Swagger"
    "02 Auth - Admin / Technician / Session"
    "03 Admin - Users, Roles, Sessions"
    "04 Fleet - Sites"
    "05 Fleet - Machines"
    "06 Fleet - Technicians & Assignments"
    "07 Machine Setup - Activation, Bootstrap, Config"
    "08 Machine Setup - Topology, Slots, Port Binding"
    "09 Catalog - Brands"
    "10 Catalog - Categories"
    "11 Catalog - Tags"
    "12 Catalog - Products"
    "13 Catalog - Media"
    "15 Inventory & Stock"
    "16 Commerce - Orders"
    "17 Commerce - Payments"
    "18 Commerce - Cash & Vend"
    "19 Telemetry - Machine Runtime"
    "20 Device Commands"
    "21 Reports & Exports"
    "22 Finance & Settlement"
    "23 Audit, Operations & Incidents"
    "24 Outbox, Retention & Reconciliation"
    "25 OTA, Rollouts & Feature Flags"
    "26 Artifacts & Diagnostics"
    "27 Webhooks & Public APIs"
    "29 Admin Utilities / Negative Tests"
  )
  for _f in "${rest_folders[@]}"; do
    extra_args+=(--folder "$_f")
  done
  echo "$(now_utc) production-full collection — Newman folders 01–29 only (skip 00/30–35/99)" >>"$LOG"
fi

set +e
{
  echo "### $(now_utc) newman run"
  echo "### collection ${COLL}"
  echo "### environment ${ENVF}"
  newman run "$COLL" -e "$ENVF" \
    "${extra_args[@]}" \
    --reporters cli json junit \
    --reporter-json-export "$JSON_REPORT" \
    --reporter-junit-export "$JUNIT_REPORT" \
    --color off
} >>"$LOG" 2>&1
ec=$?
set -e

if [[ "$ec" -ne 0 ]]; then
  log_error "Newman exited ${ec} — transcript: ${LOG} report: ${JSON_REPORT}"
fi
exit "$ec"
