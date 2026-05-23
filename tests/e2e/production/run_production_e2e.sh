#!/usr/bin/env bash
# Production release E2E harness — manifest-driven REST, gRPC, MQTT, and Postman parity.
# Usage:
#   bash tests/e2e/production/run_production_e2e.sh --mode contract [--dry-run]
#   bash tests/e2e/production/run_production_e2e.sh --mode live
set -Eeuo pipefail

PROD_E2E_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROD_E2E_REPO_ROOT="$(cd "${PROD_E2E_SCRIPT_DIR}/../../.." && pwd)"
PROD_E2E_PRODUCTION_DIR="${PROD_E2E_SCRIPT_DIR}"
export PROD_E2E_REPO_ROOT PROD_E2E_PRODUCTION_DIR

MODE="contract"
DRY_RUN=0
SKIP_NEWMAN=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --mode) MODE="${2:?}"; shift 2 ;;
    --dry-run) DRY_RUN=1; shift ;;
    --skip-newman) SKIP_NEWMAN=1; shift ;;
    -h|--help)
      sed -n '1,12p' "$0"
      exit 0
      ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done

# shellcheck source=lib/ids.sh
source "${PROD_E2E_SCRIPT_DIR}/lib/ids.sh"
# shellcheck source=lib/redact.sh
source "${PROD_E2E_SCRIPT_DIR}/lib/redact.sh"
# shellcheck source=lib/assertions.sh
source "${PROD_E2E_SCRIPT_DIR}/lib/assertions.sh"
# shellcheck source=lib/evidence.sh
source "${PROD_E2E_SCRIPT_DIR}/lib/evidence.sh"
# shellcheck source=lib/rest.sh
source "${PROD_E2E_SCRIPT_DIR}/lib/rest.sh"
# shellcheck source=lib/grpc.sh
source "${PROD_E2E_SCRIPT_DIR}/lib/grpc.sh"
# shellcheck source=lib/mqtt.sh
source "${PROD_E2E_SCRIPT_DIR}/lib/mqtt.sh"

prod_e2e_python() {
  if [[ -n "${PROD_E2E_PYTHON:-}" ]]; then
    echo "${PROD_E2E_PYTHON}"
    return 0
  fi
  if command -v python3 >/dev/null 2>&1 && python3 -c "import sys" 2>/dev/null; then
    echo python3
    return 0
  fi
  if command -v py >/dev/null 2>&1 && py -3 -c "import sys" 2>/dev/null; then
    echo "py -3"
    return 0
  fi
  echo "python3"
}

prod_e2e_py() {
  # shellcheck disable=SC2046
  $(prod_e2e_python) "$@"
}

prod_e2e_load_env() {
  local env_file="${E2E_ENV_FILE:-${PROD_E2E_SCRIPT_DIR}/.env.production.e2e.local}"
  if [[ -f "$env_file" ]]; then
    set -a
    # shellcheck disable=SC1090
    source "$env_file"
    set +a
  fi
  # GitHub Actions / CI secret mapping
  [[ -n "${E2E_PROD_BASE_URL:-}" ]] && export BASE_URL="${E2E_PROD_BASE_URL}"
  [[ -n "${E2E_PROD_ADMIN_EMAIL:-}" ]] && export ADMIN_EMAIL="${E2E_PROD_ADMIN_EMAIL}"
  [[ -n "${E2E_PROD_ADMIN_PASSWORD:-}" ]] && export ADMIN_PASSWORD="${E2E_PROD_ADMIN_PASSWORD}"
  [[ -n "${E2E_PROD_GRPC_TARGET:-}" ]] && export GRPC_ADDR="${E2E_PROD_GRPC_TARGET}"
  [[ -n "${E2E_PROD_MQTT_HOST:-}" ]] && export MQTT_HOST="${E2E_PROD_MQTT_HOST}"
  [[ -n "${E2E_PROD_MQTT_USERNAME:-}" ]] && export MQTT_USERNAME="${E2E_PROD_MQTT_USERNAME}"
  [[ -n "${E2E_PROD_MQTT_PASSWORD:-}" ]] && export MQTT_PASSWORD="${E2E_PROD_MQTT_PASSWORD}"
  [[ -n "${E2E_PROD_PAYMENT_WEBHOOK_SECRET:-}" ]] && export COMMERCE_PAYMENT_WEBHOOK_SECRET="${E2E_PROD_PAYMENT_WEBHOOK_SECRET}"
  : "${BASE_URL:=https://api.example.invalid}"
  : "${MQTT_TOPIC_PREFIX:=avf/prod}"
  export E2E_TARGET=production
}

prod_e2e_validate_contract() {
  local manifest="${PROD_E2E_SCRIPT_DIR}/e2e-manifest.yaml"
  [[ -f "$manifest" ]] || { echo "missing manifest: $manifest" >&2; return 1; }
  prod_e2e_py -c "
import sys, yaml, json
from pathlib import Path
p = Path(sys.argv[1])
m = yaml.safe_load(p.read_text(encoding='utf-8'))
assert m.get('version') == 1
flows = m.get('flows') or []
ids = [f.get('id') for f in flows]
assert len(ids) == len(set(ids)), 'duplicate flow ids'
for f in flows:
    assert f.get('id') and f.get('evidence_label'), f
print('MANIFEST_OK', len(flows), 'flows')
" "$manifest"

  for f in fixtures/test-product.png fixtures/webhook-payment-success.json fixtures/webhook-payment-failed.json; do
    [[ -f "${PROD_E2E_SCRIPT_DIR}/${f}" ]] || { echo "missing fixture: ${f}" >&2; return 1; }
  done
  for lib in lib/ids.sh lib/rest.sh lib/grpc.sh lib/mqtt.sh lib/assertions.sh lib/redact.sh lib/evidence.sh; do
    [[ -f "${PROD_E2E_SCRIPT_DIR}/${lib}" ]] || { echo "missing lib: ${lib}" >&2; return 1; }
  done
  prod_e2e_py -m py_compile "${PROD_E2E_REPO_ROOT}/postman/production/generate_postman_from_manifest.py"
}

prod_e2e_generate_postman() {
  prod_e2e_py "${PROD_E2E_REPO_ROOT}/postman/production/generate_postman_from_manifest.py"
}

prod_e2e_run_newman() {
  [[ "${SKIP_NEWMAN}" -eq 1 ]] && return 0
  local coll="${PROD_E2E_REPO_ROOT}/postman/production/avf-production-e2e.postman_collection.json"
  local envf="${PROD_E2E_REPO_ROOT}/postman/production/avf-production-e2e.postman_environment.json"
  [[ -f "$coll" && -f "$envf" ]] || { echo "Postman artifacts missing — run generator first" >&2; return 1; }
  if ! command -v newman >/dev/null 2>&1; then
    echo "NEWMAN_SKIP: newman not installed (contract/dry-run OK)"
    return 0
  fi
  if [[ "${DRY_RUN}" -eq 1 ]]; then
    echo "NEWMAN_DRY_RUN: would run newman against ${coll}"
    return 0
  fi
  if [[ "${MODE}" != "live" ]]; then
    echo "NEWMAN_SKIP: not live mode"
    return 0
  fi
  mkdir -p "${PROD_E2E_RUN_DIR}/postman"
  newman run "$coll" -e "$envf" \
    --reporters cli,json,junit \
    --reporter-json-export "${PROD_E2E_RUN_DIR}/postman/newman-report.json" \
    --reporter-junit-export "${PROD_E2E_RUN_DIR}/postman/newman-junit.xml" \
    | tee "${PROD_E2E_RUN_DIR}/postman/newman-cli.log"
}

prod_e2e_run_flows() {
  local manifest="${PROD_E2E_SCRIPT_DIR}/e2e-manifest.yaml"
  local failures=0
  while IFS= read -r flow; do
    [[ -n "$flow" ]] || continue
    local protocol
    protocol="$(echo "$flow" | jq -r '.protocol')"
    case "$protocol" in
      rest) prod_e2e_rest_execute_flow "$flow" || failures=$((failures + 1)) ;;
      grpc) prod_e2e_grpc_execute_flow "$flow" || failures=$((failures + 1)) ;;
      mqtt)
        if echo "$flow" | jq -e '.topic' >/dev/null 2>&1; then
          prod_e2e_mqtt_execute_flow "$flow" || failures=$((failures + 1))
        else
          prod_e2e_rest_execute_flow "$flow" || failures=$((failures + 1))
        fi
        ;;
      *) echo "unknown protocol: ${protocol}" >&2; failures=$((failures + 1)) ;;
    esac
  done < <(python3 -c "
import yaml, json, sys
m = yaml.safe_load(open(sys.argv[1], encoding='utf-8'))
for f in m.get('flows', []):
    print(json.dumps(f, separators=(',', ':')))
" "$manifest")
  return "$failures"
}

main() {
  prod_e2e_load_env
  prod_e2e_ids_init
  [[ "${DRY_RUN}" -eq 1 ]] && export PROD_E2E_DRY_RUN=1

  echo "== production E2E harness mode=${MODE} dry_run=${DRY_RUN} run_id=${PROD_E2E_RUN_ID} =="
  prod_e2e_validate_contract
  prod_e2e_generate_postman

  if [[ "${MODE}" == "contract" ]]; then
    prod_e2e_evidence_init
    prod_e2e_evidence_finalize
    cp "${PROD_E2E_REPO_ROOT}/docs/testing/production-e2e/RESULT_TEMPLATE.md" "${PROD_E2E_RUN_DIR}/RESULT.template.md" 2>/dev/null || true
    prod_e2e_run_newman || true
    echo "CONTRACT_OK run_dir=${PROD_E2E_RUN_DIR}"
    exit 0
  fi

  if [[ "${MODE}" == "live" ]]; then
    if [[ "${E2E_PRODUCTION_WRITE_CONFIRMATION:-}" != "I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION" ]]; then
      echo "FATAL: live mode requires E2E_PRODUCTION_WRITE_CONFIRMATION=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION" >&2
      exit 2
    fi
    [[ -n "${ADMIN_TOKEN:-}" || ( -n "${ADMIN_EMAIL:-}" && -n "${ADMIN_PASSWORD:-}" ) ]] || {
      echo "FATAL: live mode requires ADMIN_TOKEN or ADMIN_EMAIL+ADMIN_PASSWORD (or E2E_PROD_* CI secrets)" >&2
      exit 2
    }
    prod_e2e_evidence_init
    local failures=0
    prod_e2e_run_flows || failures=$?
    prod_e2e_run_newman || failures=$((failures + 1))
    prod_e2e_evidence_finalize
    [[ "$failures" -eq 0 ]] || exit 1
    echo "LIVE_OK run_dir=${PROD_E2E_RUN_DIR}"
    exit 0
  fi

  echo "unknown mode: ${MODE} (use contract or live)" >&2
  exit 2
}

main "$@"
