#!/usr/bin/env bash
# shellcheck shell=bash
# Post-run cleanup placeholder: records a P2 finding until dedicated delete/archive scenarios exist.
# Only E2E-prefixed resources should ever be targeted by future cleanup APIs.
#
# Requires the same production write + destructive confirmations as other destructive runners when
# E2E_TARGET=production and E2E_ALLOW_WRITES=true.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/e2e_common.sh
source "${SCRIPT_DIR}/lib/e2e_common.sh"
e2e_strict_mode

e2e_print_help() {
  cat <<EOF
Usage: ./tests/e2e/run-cleanup-production-e2e.sh [--reuse-data PATH]

Loads E2E_ENV_FILE (or tests/e2e/.env), creates a run dir, seeds test-data when --reuse-data is set,
logs a cleanup harness gap (P2) to improvement-findings.jsonl, and finalizes reports.

Future: map delete/archive/disable endpoints for resources tagged in test-data.json (E2E prefix only).
EOF
}

e2e_capture_inherited_data_flags
e2e_parse_common_args "$@"
load_env
e2e_restore_inherited_data_flags_if_needed

if [[ "${E2E_ALLOW_DESTRUCTIVE_CLEANUP:-false}" != "true" ]]; then
  echo "FATAL: E2E_ALLOW_DESTRUCTIVE_CLEANUP=true required for this runner" >&2
  exit 2
fi

require_cmd jq curl
e2e_require_python

new_run_dir
e2e_write_run_meta "run-cleanup-production-e2e"

# shellcheck source=lib/e2e_data.sh
source "${SCRIPT_DIR}/lib/e2e_data.sh"
e2e_data_initialize

cleanup_trap_register

# shellcheck source=lib/e2e_report.sh
source "${SCRIPT_DIR}/lib/e2e_report.sh"

log_info "cleanup runner: no automated DELETE/archive flow implemented — logging P2 finding; operator may DB-reset."

log_cleanup_issue \
  "P2" \
  "PROD-CLEANUP" \
  "run-cleanup-production-e2e.sh" \
  "cleanup-harness" \
  "REST" \
  "—" \
  "No dedicated harness to delete/archive only E2E-prefixed org/site/machine/catalog entities after destructive prod runs" \
  "Manual triage or full DB reset required post-test" \
  "Add documented teardown API matrix; wire scenarios that call delete/disable for IDs in test-data.json" \
  "${E2E_RUN_DIR}/test-data.json"

e2e_finalize_reports 0
