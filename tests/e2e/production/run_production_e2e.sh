#!/usr/bin/env bash
# Production release E2E harness — manifest-driven REST, gRPC, MQTT, and Postman parity.
# Usage:
#   bash tests/e2e/production/run_production_e2e.sh --mode contract [--dry-run]
#   bash tests/e2e/production/run_production_e2e.sh --mode route-matrix [--fetch-swagger]
#   bash tests/e2e/production/run_production_e2e.sh --mode live [--suite all|all-no-online-payment|planogram-no-online-payment|rest|grpc|mqtt]
set -Eeuo pipefail

PROD_E2E_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROD_E2E_REPO_ROOT="$(cd "${PROD_E2E_SCRIPT_DIR}/../../.." && pwd)"
PROD_E2E_PRODUCTION_DIR="${PROD_E2E_SCRIPT_DIR}"
export PROD_E2E_REPO_ROOT PROD_E2E_PRODUCTION_DIR

MODE="contract"
SUITE="all"
DRY_RUN=0
SKIP_NEWMAN=0

FETCH_SWAGGER=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --mode) MODE="${2:?}"; shift 2 ;;
    --suite) SUITE="${2:?}"; shift 2 ;;
    --dry-run) DRY_RUN=1; shift ;;
    --skip-newman) SKIP_NEWMAN=1; shift ;;
    --fetch-swagger) FETCH_SWAGGER=1; shift ;;
    -h|--help)
      sed -n '1,16p' "$0"
      exit 0
      ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done

# shellcheck source=lib/ids.sh
source "${PROD_E2E_SCRIPT_DIR}/lib/ids.sh"
# shellcheck source=lib/state.sh
source "${PROD_E2E_SCRIPT_DIR}/lib/state.sh"
# shellcheck source=lib/classify.sh
source "${PROD_E2E_SCRIPT_DIR}/lib/classify.sh"
# shellcheck source=lib/redact.sh
source "${PROD_E2E_SCRIPT_DIR}/lib/redact.sh"
# shellcheck source=lib/assertions.sh
source "${PROD_E2E_SCRIPT_DIR}/lib/assertions.sh"
# shellcheck source=lib/evidence.sh
source "${PROD_E2E_SCRIPT_DIR}/lib/evidence.sh"
# shellcheck source=lib/rest_handlers.sh
source "${PROD_E2E_SCRIPT_DIR}/lib/rest_handlers.sh"
# shellcheck source=lib/rest.sh
source "${PROD_E2E_SCRIPT_DIR}/lib/rest.sh"
# shellcheck source=lib/postman_env.sh
source "${PROD_E2E_SCRIPT_DIR}/lib/postman_env.sh"
# shellcheck source=lib/grpc.sh
source "${PROD_E2E_SCRIPT_DIR}/lib/grpc.sh"
# shellcheck source=lib/grpc_handlers.sh
source "${PROD_E2E_SCRIPT_DIR}/lib/grpc_handlers.sh"
# shellcheck source=lib/mqtt.sh
source "${PROD_E2E_SCRIPT_DIR}/lib/mqtt.sh"
# shellcheck source=lib/mqtt_handlers.sh
source "${PROD_E2E_SCRIPT_DIR}/lib/mqtt_handlers.sh"
# shellcheck source=lib/suite_exclude.sh
source "${PROD_E2E_SCRIPT_DIR}/lib/suite_exclude.sh"

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
  : "${BASE_URL:=https://api.ldtv.dev}"
  export BASE_URL
  : "${MQTT_TOPIC_PREFIX:=avf/prod}"
  : "${MQTT_TOPIC_LAYOUT:=enterprise}"
  : "${MQTT_USE_TLS:=true}"
  : "${MQTT_PORT:=8883}"
  : "${GRPC_USE_PLAINTEXT:=false}"
  : "${GRPC_USE_REFLECTION:=false}"
  : "${GRPC_PROTO_ROOT:=${PROD_E2E_REPO_ROOT}/proto}"
  export PROD_E2E_USE_MEDIA_PIPE=1
  export PROD_E2E_USE_CLOUDINARY_MEDIA=1
  export E2E_TARGET=production
  export GRPC_PROTO_ROOT
  if [[ "${MODE}" == "live" ]]; then
    export PROD_E2E_SKIP_LEGACY_MACHINE_HTTP=1
  fi
  if [[ -z "${COMMERCE_PAYMENT_WEBHOOK_SECRET:-}" && "${PROD_E2E_EXCLUDE_ONLINE_PAYMENT:-}" != "1" ]]; then
    export SKIP_GRPC_QR_WEBHOOK=1
  fi
}

prod_e2e_validate_manifest_file() {
  local manifest="$1"
  [[ -f "$manifest" ]] || { echo "missing manifest: $manifest" >&2; return 1; }
  prod_e2e_py -c "
import sys, yaml
from pathlib import Path
p = Path(sys.argv[1])
m = yaml.safe_load(p.read_text(encoding='utf-8'))
assert m.get('version') == 1
flows = m.get('flows') or []
ids = [f.get('id') for f in flows]
assert len(ids) == len(set(ids)), f'duplicate flow ids in {p.name}'
for f in flows:
    assert f.get('id'), f
    assert f.get('evidence_label') or f.get('handler'), f
print('MANIFEST_OK', p.name, len(flows), 'flows')
" "$manifest"
}

prod_e2e_validate_contract() {
  local eff="${EFFECTIVE_SUITE:-${SUITE}}"
  prod_e2e_validate_manifest_file "${PROD_E2E_SCRIPT_DIR}/e2e-manifest.yaml"
  if [[ "${eff}" == "grpc" || "${eff}" == "all" ]]; then
    prod_e2e_validate_manifest_file "${PROD_E2E_SCRIPT_DIR}/e2e-manifest-grpc.yaml"
  fi
  if [[ "${eff}" == "mqtt" || "${eff}" == "all" ]]; then
    prod_e2e_validate_manifest_file "${PROD_E2E_SCRIPT_DIR}/e2e-manifest-mqtt.yaml"
  fi
  for f in fixtures/test-product.png fixtures/webhook-payment-success.json fixtures/webhook-payment-failed.json; do
    [[ -f "${PROD_E2E_SCRIPT_DIR}/${f}" ]] || { echo "missing fixture: ${f}" >&2; return 1; }
  done
  for lib in lib/ids.sh lib/state.sh lib/rest.sh lib/rest_handlers.sh lib/classify.sh lib/postman_env.sh lib/grpc_common.sh lib/grpc.sh lib/grpc_handlers.sh lib/mqtt_common.sh lib/mqtt.sh lib/mqtt_handlers.sh lib/assertions.sh lib/redact.sh lib/evidence.sh lib/suite_exclude.sh; do
    [[ -f "${PROD_E2E_SCRIPT_DIR}/${lib}" ]] || { echo "missing lib: ${lib}" >&2; return 1; }
  done
  [[ -f "${PROD_E2E_SCRIPT_DIR}/suite-profiles.yaml" ]] || { echo "missing suite-profiles.yaml" >&2; return 1; }
  if [[ "${eff}" == "rest" || "${eff}" == "all" ]]; then
    [[ -f "${PROD_E2E_SCRIPT_DIR}/e2e-manifest-rest-coverage.yaml" ]] && \
      prod_e2e_validate_manifest_file "${PROD_E2E_SCRIPT_DIR}/e2e-manifest-rest-coverage.yaml"
  fi
  prod_e2e_py -m py_compile "${PROD_E2E_REPO_ROOT}/postman/production/manifest_postman_lib.py"
  prod_e2e_py -m py_compile "${PROD_E2E_REPO_ROOT}/postman/production/generate_postman_from_manifest.py"
  prod_e2e_py -m py_compile "${PROD_E2E_SCRIPT_DIR}/scripts/validate_postman_shell_parity.py"
  prod_e2e_py -m py_compile "${PROD_E2E_SCRIPT_DIR}/scripts/generate_rest_route_matrix.py"
  if [[ "${eff}" == "rest" || "${eff}" == "all" ]]; then
    prod_e2e_route_matrix_pipeline || return 1
  fi
}

prod_e2e_generate_route_matrix() {
  local -a args=()
  [[ "${FETCH_SWAGGER}" -eq 1 ]] && args+=(--fetch-swagger)
  args+=(--skip-postman-check)
  # Route matrix + Postman artifacts must reflect the full manifest; online payment exclusion is runtime-only.
  local saved_exclude="${PROD_E2E_EXCLUDE_ONLINE_PAYMENT:-}"
  unset PROD_E2E_EXCLUDE_ONLINE_PAYMENT
  prod_e2e_py "${PROD_E2E_SCRIPT_DIR}/scripts/generate_rest_route_matrix.py" "${args[@]}"
  local rc=$?
  [[ -n "${saved_exclude}" ]] && export PROD_E2E_EXCLUDE_ONLINE_PAYMENT="${saved_exclude}"
  return "${rc}"
}

prod_e2e_validate_route_matrix() {
  local saved_exclude="${PROD_E2E_EXCLUDE_ONLINE_PAYMENT:-}"
  unset PROD_E2E_EXCLUDE_ONLINE_PAYMENT
  prod_e2e_py "${PROD_E2E_SCRIPT_DIR}/scripts/generate_rest_route_matrix.py" --validate-only --no-write-manifest
  local rc=$?
  [[ -n "${saved_exclude}" ]] && export PROD_E2E_EXCLUDE_ONLINE_PAYMENT="${saved_exclude}"
  return "${rc}"
}

prod_e2e_route_matrix_pipeline() {
  prod_e2e_generate_route_matrix || return 1
  prod_e2e_generate_postman || return 1
  prod_e2e_validate_route_matrix || return 1
}

prod_e2e_generate_postman() {
  # Committed Postman must always reflect the full manifest; online payment exclusion is runtime-only.
  local saved_exclude="${PROD_E2E_EXCLUDE_ONLINE_PAYMENT:-}"
  unset PROD_E2E_EXCLUDE_ONLINE_PAYMENT
  prod_e2e_py "${PROD_E2E_REPO_ROOT}/postman/production/generate_postman_from_manifest.py"
  local rc=$?
  [[ -n "${saved_exclude}" ]] && export PROD_E2E_EXCLUDE_ONLINE_PAYMENT="${saved_exclude}"
  return "${rc}"
}

prod_e2e_validate_postman_parity() {
  prod_e2e_py "${PROD_E2E_SCRIPT_DIR}/scripts/validate_postman_shell_parity.py" || {
    prod_e2e_fail_classify "c" "POSTMAN-PARITY-000" "shell REST and Postman collection diverged — regenerate from e2e-manifest.yaml"
    return 1
  }
}

prod_e2e_lock_postman_parity() {
  case "${SUITE}" in
    grpc|mqtt) return 0 ;;
  esac
  echo "== Postman parity lock (e2e-manifest.yaml) =="
  prod_e2e_generate_postman || return 1
  prod_e2e_validate_postman_parity || return 1
}

prod_e2e_write_postman_checksums() {
  case "${SUITE}" in
    grpc|mqtt) return 0 ;;
  esac
  mkdir -p "${PROD_E2E_RUN_DIR}/postman"
  prod_e2e_py -c "
import hashlib, json, os
from pathlib import Path
root = Path(os.environ['PROD_E2E_REPO_ROOT'])
run = Path(os.environ['PROD_E2E_RUN_DIR'])
def sha(p):
    h = hashlib.sha256()
    with open(p, 'rb') as f:
        for c in iter(lambda: f.read(65536), b''):
            h.update(c)
    return h.hexdigest()
artifacts = {
    'manifest_sha256': sha(root / 'tests/e2e/production/e2e-manifest.yaml'),
    'collection_sha256': sha(root / 'postman/production/avf-production-e2e.postman_collection.json'),
    'environment_sha256': sha(root / 'postman/production/avf-production-e2e.postman_environment.json'),
}
nr = run / 'postman/newman-report.json'
if nr.is_file():
    artifacts['newman_report_sha256'] = sha(nr)
(run / 'postman/parity-checksums.json').write_text(json.dumps(artifacts, indent=2) + '\n', encoding='utf-8')
print('POSTMAN_CHECKSUMS', json.dumps(artifacts))
"
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
  prod_e2e_sync_postman_env || return 1
  local envf="${PROD_E2E_RUN_DIR}/postman/runtime.postman_environment.json"
  mkdir -p "${PROD_E2E_RUN_DIR}/postman"
  newman run "$coll" -e "$envf" \
    --reporters cli,json,junit \
    --reporter-json-export "${PROD_E2E_RUN_DIR}/postman/newman-report.json" \
    --reporter-junit-export "${PROD_E2E_RUN_DIR}/postman/newman-junit.xml" \
    | tee "${PROD_E2E_RUN_DIR}/postman/newman-cli.log"
}

prod_e2e_flow_matches_suite() {
  local protocol="$1"
  local phase="$2"
  local eff="${EFFECTIVE_SUITE:-${SUITE}}"
  case "${eff}" in
    all) return 0 ;;
    rest)
      [[ "$protocol" == "rest" ]] && return 0
      return 1
      ;;
    grpc)
      if [[ "$protocol" == "grpc" ]]; then
        return 0
      fi
      if [[ "$protocol" == "rest" ]]; then
        case "$phase" in
          preflight|auth|catalog|media|provisioning|planogram) return 0 ;;
          *) return 1 ;;
        esac
      fi
      return 1
      ;;
    mqtt)
      if [[ "$protocol" == "mqtt" ]]; then
        return 0
      fi
      if [[ "$protocol" == "rest" ]]; then
        case "$phase" in
          preflight|auth|catalog|media|provisioning) return 0 ;;
          *) return 1 ;;
        esac
      fi
      return 1
      ;;
    planogram-no-online-payment)
      if [[ "$protocol" == "rest" ]]; then
        case "$phase" in
          preflight|auth|catalog|media|provisioning|planogram|operator|reports) return 0 ;;
          *) return 1 ;;
        esac
      fi
      return 1
      ;;
    *) return 0 ;;
  esac
}

prod_e2e_run_flows_from_manifest() {
  local manifest="$1"
  local failures=0
  local last_phase=""
  while IFS= read -r flow; do
    [[ -n "$flow" ]] || continue
    local protocol phase fid fpath skip_reason
    protocol="$(echo "$flow" | jq -r '.protocol' | tr -d '\r')"
    phase="$(echo "$flow" | jq -r '.phase // ""' | tr -d '\r')"
    fid="$(echo "$flow" | jq -r '.id // ""' | tr -d '\r')"
    fpath="$(echo "$flow" | jq -r '.path // .label // ""' | tr -d '\r')"
    if skip_reason="$(prod_e2e_flow_skip_reason "$fid" "$fpath")"; then
      prod_e2e_record_skipped_flow "$flow" "$skip_reason"
      continue
    fi
    if [[ "${PROD_E2E_PREFLIGHT_ONLY:-}" == "1" ]]; then
      case "$phase" in
        preflight) ;;
        auth)
          case "$fid" in
            REST-AUTH-003|REST-AUTH-004) ;;
            *) continue ;;
          esac
          ;;
        rest-coverage) continue ;;
        *) continue ;;
      esac
    fi
    prod_e2e_flow_matches_suite "$protocol" "$phase" || continue
    if [[ "$phase" == "planogram" && "$last_phase" != "planogram" ]]; then
      prod_e2e_refresh_admin_token || true
    fi
    last_phase="$phase"
    case "$protocol" in
      rest) prod_e2e_rest_run_flow "$flow" || failures=$((failures + 1)) ;;
      grpc) prod_e2e_grpc_run_flow "$flow" || failures=$((failures + 1)) ;;
      mqtt)
        if echo "$flow" | jq -e '.topic' >/dev/null 2>&1; then
          prod_e2e_mqtt_execute_flow "$flow" || failures=$((failures + 1))
        else
          prod_e2e_mqtt_run_flow "$flow" || failures=$((failures + 1))
        fi
        ;;
      *) echo "unknown protocol: ${protocol}" >&2; failures=$((failures + 1)) ;;
    esac
  done < <(prod_e2e_py -c "
import yaml, json, sys
m = yaml.safe_load(open(sys.argv[1], encoding='utf-8'))
for f in m.get('flows', []):
    print(json.dumps(f, separators=(',', ':')))
" "$manifest")
  return "$failures"
}

prod_e2e_run_flows() {
  local failures=0
  local eff="${EFFECTIVE_SUITE:-${SUITE}}"
  prod_e2e_run_flows_from_manifest "${PROD_E2E_SCRIPT_DIR}/e2e-manifest.yaml" || failures=$?
  if [[ "${eff}" == "rest" || "${eff}" == "all" ]]; then
    if [[ -f "${PROD_E2E_SCRIPT_DIR}/e2e-manifest-rest-coverage.yaml" && "${PROD_E2E_PREFLIGHT_ONLY:-}" != "1" ]]; then
      prod_e2e_refresh_admin_token || true
      local rf=0
      prod_e2e_run_flows_from_manifest "${PROD_E2E_SCRIPT_DIR}/e2e-manifest-rest-coverage.yaml" || rf=$?
      failures=$((failures + rf))
    fi
  fi
  if [[ "${eff}" == "grpc" || "${eff}" == "all" ]]; then
    local gf=0
    prod_e2e_run_flows_from_manifest "${PROD_E2E_SCRIPT_DIR}/e2e-manifest-grpc.yaml" || gf=$?
    failures=$((failures + gf))
  fi
  if [[ "${eff}" == "mqtt" || "${eff}" == "all" ]]; then
    local mf=0
    prod_e2e_run_flows_from_manifest "${PROD_E2E_SCRIPT_DIR}/e2e-manifest-mqtt.yaml" || mf=$?
    failures=$((failures + mf))
  fi
  return "$failures"
}

prod_e2e_write_results_report() {
  [[ "${MODE}" == "live" ]] || return 0
  prod_e2e_py "${PROD_E2E_SCRIPT_DIR}/scripts/generate_e2e_results_report.py" "${PROD_E2E_RUN_ID}" || true
}

prod_e2e_print_final_verdict() {
  [[ "${MODE}" != "live" ]] && return 0
  local failures_file="${PROD_E2E_RUN_DIR}/failures.classification.txt"
  local fail_count=0
  [[ -f "$failures_file" ]] && fail_count="$(wc -l <"$failures_file" | tr -d ' \r')"
  local skipped_file="${PROD_E2E_RUN_DIR}/skipped.flows.txt"
  local skipped_count=0
  [[ -f "$skipped_file" ]] && skipped_count="$(wc -l <"$skipped_file" | tr -d ' \r')"
  local verdict="PRODUCTION_E2E_NO_ONLINE_PAYMENT_FAILED"
  if [[ "${fail_count:-0}" -eq 0 ]]; then
    verdict="PRODUCTION_E2E_NO_ONLINE_PAYMENT_100_PERCENT_PASS"
  fi
  if [[ "${SUITE}" == "all-no-online-payment" || "${SUITE}" == "planogram-no-online-payment" ]]; then
    echo ""
    echo "== FINAL VERDICT =="
    echo "RUN_ID=${PROD_E2E_RUN_ID}"
    echo "RESULTS=docs/testing/production-e2e/RESULTS_${PROD_E2E_RUN_ID}.md"
    echo "RAW=${PROD_E2E_RUN_DIR}/raw"
    echo "NEWMAN=${PROD_E2E_RUN_DIR}/postman/newman-report.json"
    echo "online_payment_excluded=YES"
    echo "skipped_flows=${skipped_count}"
    echo "final_verdict=${verdict}"
  fi
}

main() {
  prod_e2e_load_env
  prod_e2e_suite_profile_init
  EFFECTIVE_SUITE="$(prod_e2e_suite_effective)"
  export EFFECTIVE_SUITE
  prod_e2e_ids_init
  [[ "${DRY_RUN}" -eq 1 ]] && export PROD_E2E_DRY_RUN=1

  echo "== production E2E harness mode=${MODE} suite=${SUITE} effective=${EFFECTIVE_SUITE} dry_run=${DRY_RUN} run_id=${PROD_E2E_RUN_ID} =="
  prod_e2e_validate_contract

  if [[ "${MODE}" == "route-matrix" ]]; then
    echo "ROUTE_MATRIX_OK"
    exit 0
  fi

  if [[ "${EFFECTIVE_SUITE}" != "rest" && "${EFFECTIVE_SUITE}" != "all" ]]; then
    prod_e2e_generate_postman
  fi

  if [[ "${MODE}" == "contract" ]]; then
    prod_e2e_evidence_init
    prod_e2e_generate_postman || exit 1
    prod_e2e_validate_postman_parity || exit 1
    prod_e2e_evidence_finalize
    cp "${PROD_E2E_REPO_ROOT}/docs/testing/production-e2e/RESULT_TEMPLATE.md" "${PROD_E2E_RUN_DIR}/RESULT.template.md" 2>/dev/null || true
    prod_e2e_run_newman || true
    echo "CONTRACT_OK run_dir=${PROD_E2E_RUN_DIR}"
    exit 0
  fi

  if [[ "${MODE}" == "preflight" ]]; then
    export PROD_E2E_PREFLIGHT_ONLY=1
    prod_e2e_evidence_init
    local failures=0
    prod_e2e_run_flows || failures=$?
    prod_e2e_evidence_finalize
    prod_e2e_state_sync_json
    [[ "$failures" -eq 0 ]] || exit 1
    echo "PREFLIGHT_OK run_dir=${PROD_E2E_RUN_DIR}"
    exit 0
  fi

  if [[ "${MODE}" == "live" ]]; then
    if [[ "${E2E_PRODUCTION_WRITE_CONFIRMATION:-}" != "I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION" ]]; then
      echo "FATAL: live mode requires E2E_PRODUCTION_WRITE_CONFIRMATION=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION" >&2
      exit 2
    fi
    [[ -n "${ADMIN_TOKEN:-}" || ( -n "${ADMIN_EMAIL:-}" && -n "${ADMIN_PASSWORD:-}" ) ]] || {
      prod_e2e_fail_classify "d" "REST-AUTH-000" "live mode requires ADMIN_TOKEN or ADMIN_EMAIL+ADMIN_PASSWORD (or E2E_PROD_* CI secrets)"
      echo "FATAL: live mode requires ADMIN_TOKEN or ADMIN_EMAIL+ADMIN_PASSWORD (or E2E_PROD_* CI secrets)" >&2
      exit 2
    }
    if [[ "${EFFECTIVE_SUITE}" == "mqtt" || "${EFFECTIVE_SUITE}" == "all" ]]; then
      [[ -n "${MQTT_HOST:-}" && -n "${MQTT_USERNAME:-}" && -n "${MQTT_PASSWORD:-}" ]] || {
        prod_e2e_fail_classify "d" "MQTT-CONN-000" "live MQTT suite requires E2E_PROD_MQTT_HOST/USERNAME/PASSWORD (or .env.production.e2e.local)"
        echo "FATAL: live MQTT suite requires MQTT_HOST, MQTT_USERNAME, MQTT_PASSWORD (map from E2E_PROD_MQTT_*)" >&2
        exit 2
      }
      [[ -n "${GRPC_ADDR:-}" || -n "${E2E_PROD_GRPC_TARGET:-}" ]] || {
        echo "FATAL: live suite requires E2E_PROD_GRPC_TARGET (or GRPC_ADDR)" >&2
        exit 2
      }
      prod_e2e_mqtt_ensure_clients || {
        prod_e2e_fail_classify "a" "MQTT-CONN-000" "mosquitto_pub/mosquitto_sub not installed"
        echo "FATAL: install mosquitto clients (e.g. winget install EclipseFoundation.Mosquitto)" >&2
        exit 2
      }
    fi
    if [[ "${EFFECTIVE_SUITE}" == "grpc" || "${EFFECTIVE_SUITE}" == "all" ]]; then
      [[ -n "${GRPC_ADDR:-}" || -n "${E2E_PROD_GRPC_TARGET:-}" ]] || {
        echo "FATAL: live gRPC suite requires E2E_PROD_GRPC_TARGET (or GRPC_ADDR)" >&2
        exit 2
      }
    fi
    prod_e2e_evidence_init
    printf '%s\n' "${MODE}" >"${PROD_E2E_RUN_DIR}/harness.mode.txt"
    printf '%s\n' "${SUITE}" >"${PROD_E2E_RUN_DIR}/suite.profile.txt"
    local failures=0
    prod_e2e_run_flows || failures=$?
    if [[ "$failures" -eq 0 && "${EFFECTIVE_SUITE}" != "grpc" && "${EFFECTIVE_SUITE}" != "mqtt" && "${EFFECTIVE_SUITE}" != "planogram-no-online-payment" ]]; then
      prod_e2e_lock_postman_parity || failures=$((failures + 1))
    fi
    if [[ "${EFFECTIVE_SUITE}" != "grpc" && "${EFFECTIVE_SUITE}" != "mqtt" && "${EFFECTIVE_SUITE}" != "planogram-no-online-payment" ]]; then
      prod_e2e_run_newman || failures=$((failures + 1))
    fi
    prod_e2e_write_postman_checksums || true
    prod_e2e_evidence_finalize
    prod_e2e_state_sync_json
    prod_e2e_write_results_report
    prod_e2e_print_final_verdict
    [[ "$failures" -eq 0 ]] || exit 1
    case "${SUITE}" in
      grpc) echo "GRPC_LIVE_OK run_dir=${PROD_E2E_RUN_DIR}" ;;
      mqtt) echo "MQTT_LIVE_OK run_dir=${PROD_E2E_RUN_DIR}" ;;
      *) echo "LIVE_OK run_dir=${PROD_E2E_RUN_DIR}" ;;
    esac
    exit 0
  fi

  echo "unknown mode: ${MODE} (use contract, preflight, live, or route-matrix)" >&2
  exit 2
}

main "$@"
