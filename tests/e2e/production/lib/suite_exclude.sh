#!/usr/bin/env bash
# shellcheck shell=bash
# Suite profile exclusions (e.g. all-no-online-payment).

prod_e2e_suite_profile_init() {
  PROD_E2E_SUITE_PROFILE="${PROD_E2E_SUITE_PROFILE:-}"
  case "${SUITE}" in
    all-no-online-payment)
      export PROD_E2E_SUITE_PROFILE=all-no-online-payment
      export PROD_E2E_EXCLUDE_ONLINE_PAYMENT=1
      export SKIP_GRPC_QR_WEBHOOK=1
      ;;
  esac
}

prod_e2e_suite_effective() {
  case "${SUITE}" in
    all-no-online-payment) echo all ;;
    *) echo "${SUITE}" ;;
  esac
}

prod_e2e_flow_skip_reason() {
  local fid="$1"
  local fpath="${2:-}"
  [[ "${PROD_E2E_EXCLUDE_ONLINE_PAYMENT:-}" == "1" ]] || return 1
  local profile="${PROD_E2E_PRODUCTION_DIR}/suite-profiles.yaml"
  [[ -f "$profile" ]] || return 1
  prod_e2e_py -c "
import sys
from pathlib import Path
try:
    import yaml
except ImportError:
    sys.exit(1)
prof = yaml.safe_load(Path(sys.argv[1]).read_text(encoding='utf-8'))
cfg = (prof.get('profiles') or {}).get('all-no-online-payment') or {}
fid, path = sys.argv[2], sys.argv[3]
if fid in (cfg.get('exclude_flow_ids') or []):
    print(cfg.get('skip_reason', 'excluded'))
    raise SystemExit(0)
for sub in cfg.get('exclude_coverage_path_substrings') or []:
    if sub and sub in path:
        print(cfg.get('skip_reason', 'excluded'))
        raise SystemExit(0)
sys.exit(1)
" "$profile" "$fid" "$fpath" 2>/dev/null || return 1
}

prod_e2e_record_skipped_flow() {
  local flow_json="$1"
  local reason="$2"
  local id label protocol evidence_label
  id="$(echo "$flow_json" | jq -r '.id // ""' | tr -d '\r')"
  label="$(echo "$flow_json" | jq -r '.label // ""' | tr -d '\r')"
  protocol="$(echo "$flow_json" | jq -r '.protocol // ""' | tr -d '\r')"
  evidence_label="$(echo "$flow_json" | jq -r '.evidence_label // ""' | tr -d '\r')"
  printf '%s\n' "$reason" >"${PROD_E2E_RAW_DIR}/${evidence_label}.skipped.txt" 2>/dev/null || true
  prod_e2e_evidence_append_row "$id" "$label" "$protocol" "skipped" "$evidence_label"
  {
    echo ""
    echo "### ${id} — SKIPPED"
    echo ""
    echo "- reason: \`${reason}\`"
  } >>"${PROD_E2E_RESULT_MD}"
  printf '%s\n' "${id}|skipped|${reason}" >>"${PROD_E2E_RUN_DIR}/skipped.flows.txt"
}
