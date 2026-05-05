#!/usr/bin/env bash
# shellcheck shell=bash
# Web admin flow scenarios (optional).

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/e2e_common.sh
source "${SCRIPT_DIR}/lib/e2e_common.sh"
e2e_strict_mode

e2e_print_help() {
  cat <<EOF
Usage: ./tests/e2e/run-web-admin-flows.sh [options]

  Automates Web Admin REST setup (flow WA-SETUP-01): auth, site, machine,
  activation code, catalog (category / optional brand+tag), product, then
  best-effort planogram draft/publish and stock adjustment when the org has a planogram
  and operator login succeeds.

  Common options:
    --fresh-data              Clear test-data.json in this run; create timestamped resources
    --reuse-data PATH         Copy PATH -> run test-data.json (reuse IDs; fewer creates)
    -h, --help                This message

  Environment (see tests/e2e/.env.example):
    E2E_ALLOW_WRITES=true     Required for mutations (local default in load_env is true)
    BASE_URL                  API base (default http://127.0.0.1:8080)
    ADMIN_TOKEN               Optional: skip login
    ADMIN_EMAIL / ADMIN_PASSWORD / E2E_ORGANIZATION_ID
                              Login via POST /v1/auth/login when token not set
    E2E_SEED_FILE             Optional path to seed JSON (default: tests/e2e/data/seed.local.example.json)
    E2E_TARGET=production     Only with E2E_PRODUCTION_WRITE_CONFIRMATION=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION

  Artifacts: .e2e-runs/run-*/test-data.json, test-events.jsonl, rest/*.request.json|*.response.json|*.meta.json
EOF
}

e2e_capture_inherited_data_flags
e2e_parse_common_args "$@"
load_env
e2e_restore_inherited_data_flags_if_needed

export BASE_URL GRPC_ADDR E2E_TARGET E2E_ALLOW_WRITES E2E_PRODUCTION_WRITE_CONFIRMATION
export E2E_REUSE_DATA E2E_DATA_FILE E2E_CLI_FRESH_DATA
export ADMIN_TOKEN ADMIN_EMAIL ADMIN_PASSWORD E2E_ORGANIZATION_ID
export E2E_SEED_FILE E2E_WEB_ADMIN_SKIP_BRAND E2E_WEB_ADMIN_SKIP_TAG

require_cmd jq curl python3

new_run_dir
e2e_write_run_meta "run-web-admin-flows"

# shellcheck source=lib/e2e_data.sh
source "${SCRIPT_DIR}/lib/e2e_data.sh"
if [[ "${E2E_IN_PARENT:-0}" != "1" ]]; then
  e2e_data_initialize
fi

[[ "${E2E_IN_PARENT:-0}" == "1" ]] || cleanup_trap_register

start_step "web-admin-flows"
SCENARIO="${SCRIPT_DIR}/scenarios/01_web_admin_setup.sh"
ec=0
if [[ -f "$SCENARIO" ]]; then
  set +e
  bash "$SCENARIO"
  ec=$?
  set -e
  if [[ "$ec" -eq 0 ]]; then
    end_step passed "web admin flows completed"
  else
    end_step failed "web admin flows exit ${ec}"
  fi
else
  e2e_skip "web admin scenarios not implemented (${SCENARIO} missing)"
  end_step skipped "web admin placeholder"
fi

if [[ "${E2E_IN_PARENT:-0}" != "1" ]]; then
  # shellcheck source=lib/e2e_report.sh
  source "${SCRIPT_DIR}/lib/e2e_report.sh"
  e2e_finalize_reports "${ec}"
  sm="${E2E_RUN_DIR}/reports/summary.md"
  if [[ -f "$sm" ]]; then
    {
      echo ""
      echo "## Web admin setup steps (WA-SETUP-01)"
      echo ""
      echo "Ordered REST steps automated by \`scenarios/01_web_admin_setup.sh\` (see \`test-events.jsonl\` for pass/fail/skip per call):"
      echo ""
      echo "1. **Auth** — \`ADMIN_TOKEN\` or \`POST /v1/auth/login\`"
      echo "2. **Site** — \`POST /v1/admin/organizations/{organizationId}/sites\` (or reuse \`siteId\`)"
      echo "3. **Machine** — \`POST /v1/admin/organizations/{organizationId}/machines\` (or reuse \`machineId\`)"
      echo "4. **Activation code** — \`POST /v1/admin/organizations/{organizationId}/activation-codes\`"
      echo "5. **Category** — \`POST /v1/admin/categories\` (+ optional brand/tag)"
      echo "6. **Product** — \`POST /v1/admin/products\`"
      echo "7. **Planogram list** — \`GET /v1/admin/planograms\`"
      echo "8. **Operator session** — \`POST /v1/machines/{machineId}/operator-sessions/login\`"
      echo "9. **Planogram draft / publish** — PUT draft, POST publish"
      echo "10. **Inventory** — \`POST .../stock-adjustments\` (after publish)"
      echo ""
    } >>"$sm"
  fi
fi

[[ "${E2E_IN_PARENT:-0}" == "1" ]] && exit "${ec}"
exit "${ec}"
