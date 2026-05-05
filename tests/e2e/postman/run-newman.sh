#!/usr/bin/env bash
# shellcheck shell=bash
# Run Newman against POSTMAN_COLLECTION + POSTMAN_ENV; honor write and production guards.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
E2E_SCRIPT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
# shellcheck source=../lib/e2e_common.sh
source "${E2E_SCRIPT_DIR}/lib/e2e_common.sh"
e2e_strict_mode

load_env

: "${POSTMAN_COLLECTION:=docs/postman/avf-vending-api-function-path.postman_collection.json}"
: "${POSTMAN_ENV:=docs/postman/avf-local.postman_environment.json}"
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
fi

set +e
{
  echo "### $(now_utc) newman run"
  echo "### collection ${COLL}"
  echo "### environment ${ENVF}"
  newman run "$COLL" -e "$ENVF" \
    "${extra_args[@]}" \
    --reporters cli,json,junit \
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
