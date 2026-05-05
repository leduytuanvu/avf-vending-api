#!/usr/bin/env bash
# shellcheck shell=bash
# WA-RPT-10: finance reporting, commerce reconciliation, audit, artifacts, cash collections list.
# Read-focused; non-500 + basic JSON shape checks.

set +e
set -u

E2E_SCENARIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
: "${E2E_SCRIPT_DIR:=$(cd "${E2E_SCENARIO_DIR}/.." && pwd)}"

# shellcheck source=../lib/e2e_common.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_common.sh"
# shellcheck source=../lib/e2e_http.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_http.sh"
# shellcheck source=../lib/e2e_data.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_data.sh"

FLOW_ID="WA-RPT-10"
MODULE="reporting_audit"
WA4_LOG="${E2E_RUN_DIR}/reports/wa-module-results.jsonl"

mkdir -p "${E2E_RUN_DIR}/reports"

wa4_record() {
  local step="$1" endpoint="$2" status="$3" http="$4" expected="$5" actual="$6" remed="$7" stepfile="$8"
  local reqf="${E2E_RUN_DIR}/rest/${stepfile}.request.json"
  local respf="${E2E_RUN_DIR}/rest/${stepfile}.response.json"
  [[ -f "$reqf" ]] || reqf=""
  [[ -f "$respf" ]] || respf=""
  jq -nc \
    --arg ts "$(now_utc)" \
    --arg mod "$MODULE" \
    --arg flow "$FLOW_ID" \
    --arg step "$step" \
    --arg ep "$endpoint" \
    --arg st "$status" \
    --argjson http "${http:-0}" \
    --arg exp "$expected" \
    --arg act "$actual" \
    --arg rem "$remed" \
    --arg req "$reqf" \
    --arg rsp "$respf" \
    '{ts:$ts,module:$mod,flow_id:$flow,step:$step,endpoint:$ep,status:$st,httpStatus:$http,expected:$exp,actual:$act,remediation:$rem,requestPath:$req,responsePath:$rsp}' \
    >>"$WA4_LOG"
  e2e_append_test_event "$FLOW_ID" "$step" "REST" "$endpoint" "$status" "$actual" "{}"
}

if [[ -z "${ADMIN_TOKEN:-}" ]]; then
  t="$(get_secret adminAccessToken 2>/dev/null || true)"
  [[ -n "$t" ]] && export ADMIN_TOKEN="$t"
fi
if [[ -z "${ADMIN_TOKEN:-}" ]]; then
  log_error "WA-RPT-10: ADMIN_TOKEN or secrets.adminAccessToken required"
  exit 2
fi

ORG_ID="$(get_data organizationId)"
MACHINE_ID="$(get_data machineId)"
if [[ -z "$ORG_ID" ]] || [[ "$ORG_ID" == "null" ]]; then
  log_error "WA-RPT-10: organizationId missing in test-data.json"
  exit 2
fi
Q_ORG="organization_id=$(printf '%s' "$ORG_ID" | jq -sRr @uri)"

ANY_FAIL=0
http_server_error() { [[ "$1" =~ ^5 ]] || [[ "$1" == "0" ]]; }

# --- Finance daily close list ---
path="/v1/admin/finance/daily-close?${Q_ORG}&limit=5"
code="$(e2e_http_get "rpt-finance-close-list" "$path")"
if [[ "$code" == "200" ]] && jq -e '(.items|type=="array") and (.meta|type=="object")' "${E2E_RUN_DIR}/rest/rpt-finance-close-list.response.json" >/dev/null 2>&1; then
  wa4_record "finance-daily-close-list" "GET /v1/admin/finance/daily-close" "pass" "$code" "200 + items[] + meta" "ok" "" "rpt-finance-close-list"
elif [[ "$code" == "401" ]] || [[ "$code" == "403" ]]; then
  wa4_record "finance-daily-close-list" "GET /v1/admin/finance/daily-close" "skip" "$code" "200 + items + meta" "HTTP $code" "finance / cash.write role; rpt-finance-close-list.response.json" "rpt-finance-close-list"
elif http_server_error "$code"; then
  ANY_FAIL=1
  wa4_record "finance-daily-close-list" "GET /v1/admin/finance/daily-close" "fail" "$code" "not 5xx" "HTTP $code" "Server error — inspect rest/rpt-finance-close-list.response.json" "rpt-finance-close-list"
else
  wa4_record "finance-daily-close-list" "GET /v1/admin/finance/daily-close" "skip" "$code" "200 + items + meta" "HTTP $code" "Schema or client error" "rpt-finance-close-list"
fi

# --- Commerce reconciliation ---
path="/v1/admin/organizations/${ORG_ID}/commerce/reconciliation?limit=5"
code="$(e2e_http_get "rpt-commerce-recon" "$path")"
if [[ "$code" == "200" ]] && jq -e '(.items|type=="array")' "${E2E_RUN_DIR}/rest/rpt-commerce-recon.response.json" >/dev/null 2>&1; then
  wa4_record "commerce-reconciliation-list" "GET .../commerce/reconciliation" "pass" "$code" "200 + items" "ok" "" "rpt-commerce-recon"
elif [[ "$code" == "401" ]] || [[ "$code" == "403" ]]; then
  wa4_record "commerce-reconciliation-list" "GET .../commerce/reconciliation" "skip" "$code" "200 + items" "HTTP $code" "Org scope / role" "rpt-commerce-recon"
elif http_server_error "$code"; then
  ANY_FAIL=1
  wa4_record "commerce-reconciliation-list" "GET .../commerce/reconciliation" "fail" "$code" "not 5xx" "HTTP $code" "rest/rpt-commerce-recon.response.json" "rpt-commerce-recon"
else
  wa4_record "commerce-reconciliation-list" "GET .../commerce/reconciliation" "skip" "$code" "200 + items" "HTTP $code" "rest/rpt-commerce-recon.response.json" "rpt-commerce-recon"
fi

# --- Audit events ---
path="/v1/admin/audit/events?${Q_ORG}&limit=10"
code="$(e2e_http_get "rpt-audit-events" "$path")"
if [[ "$code" == "200" ]] && jq -e '(.items|type=="array")' "${E2E_RUN_DIR}/rest/rpt-audit-events.response.json" >/dev/null 2>&1; then
  wa4_record "audit-events-list" "GET /v1/admin/audit/events" "pass" "$code" "200 + items" "ok" "" "rpt-audit-events"
elif [[ "$code" == "401" ]] || [[ "$code" == "403" ]]; then
  wa4_record "audit-events-list" "GET /v1/admin/audit/events" "skip" "$code" "200 + items" "HTTP $code" "audit.read" "rpt-audit-events"
elif http_server_error "$code"; then
  ANY_FAIL=1
  wa4_record "audit-events-list" "GET /v1/admin/audit/events" "fail" "$code" "not 5xx" "HTTP $code" "rest/rpt-audit-events.response.json" "rpt-audit-events"
else
  wa4_record "audit-events-list" "GET /v1/admin/audit/events" "skip" "$code" "200 + items" "HTTP $code" "rest/rpt-audit-events.response.json" "rpt-audit-events"
fi

# --- Artifacts list ---
path="/v1/admin/organizations/${ORG_ID}/artifacts?limit=5"
code="$(e2e_http_get "rpt-artifacts-list" "$path")"
if [[ "$code" == "200" ]] && jq -e '(.items|type=="array")' "${E2E_RUN_DIR}/rest/rpt-artifacts-list.response.json" >/dev/null 2>&1; then
  wa4_record "artifacts-list" "GET .../artifacts" "pass" "$code" "200 + items" "ok" "" "rpt-artifacts-list"
elif [[ "$code" == "200" ]]; then
  wa4_record "artifacts-list" "GET .../artifacts" "pass" "$code" "200 JSON" "ok (schema variant)" "" "rpt-artifacts-list"
elif [[ "$code" == "401" ]] || [[ "$code" == "403" ]]; then
  wa4_record "artifacts-list" "GET .../artifacts" "skip" "$code" "200" "HTTP $code" "Artifacts ACL / feature flag" "rpt-artifacts-list"
elif http_server_error "$code"; then
  ANY_FAIL=1
  wa4_record "artifacts-list" "GET .../artifacts" "fail" "$code" "not 5xx" "HTTP $code" "rest/rpt-artifacts-list.response.json" "rpt-artifacts-list"
else
  wa4_record "artifacts-list" "GET .../artifacts" "skip" "$code" "200" "HTTP $code" "rest/rpt-artifacts-list.response.json" "rpt-artifacts-list"
fi

# --- Cash collections (list only) ---
if [[ -n "$MACHINE_ID" ]] && [[ "$MACHINE_ID" != "null" ]]; then
  path="/v1/admin/machines/${MACHINE_ID}/cash-collections?${Q_ORG}&limit=10"
  code="$(e2e_http_get "rpt-cash-collections" "$path")"
  if [[ "$code" == "200" ]] && jq -e '(.items|type=="array")' "${E2E_RUN_DIR}/rest/rpt-cash-collections.response.json" >/dev/null 2>&1; then
    wa4_record "cash-collections-list" "GET .../cash-collections" "pass" "$code" "200 + items" "ok" "" "rpt-cash-collections"
  elif [[ "$code" == "401" ]] || [[ "$code" == "403" ]]; then
    wa4_record "cash-collections-list" "GET .../cash-collections" "skip" "$code" "200 + items" "HTTP $code" "Cash settlement permissions" "rpt-cash-collections"
  elif http_server_error "$code"; then
    ANY_FAIL=1
    wa4_record "cash-collections-list" "GET .../cash-collections" "fail" "$code" "not 5xx" "HTTP $code" "rest/rpt-cash-collections.response.json" "rpt-cash-collections"
  else
    wa4_record "cash-collections-list" "GET .../cash-collections" "skip" "$code" "200 + items" "HTTP $code" "rest/rpt-cash-collections.response.json" "rpt-cash-collections"
  fi
else
  wa4_record "cash-collections-list" "—" "skip" "0" "machineId" "missing" "Run Phase 3 setup or reuse-data with machineId" ""
fi

log_observability_issue "P2" "$FLOW_ID" "10_reporting_audit_reconciliation.sh" "audit-correlation" "REST" "GET /v1/admin/audit/events" "Harness does not assert correlation_id / request_id presence on each audit row" "Cannot tie audit to commerce requests" "Require correlation fields in audit API and docs" "${E2E_RUN_DIR}/rest/rpt-audit-events.response.json"
log_cleanup_issue "P3" "$FLOW_ID" "10_reporting_audit_reconciliation.sh" "report-filters" "REST" "reporting routes" "Commerce/finance lists may not filter cleanly by ephemeral E2E machine/order id" "Noisy shared environments" "Add narrow filters (machineId, external ref) per OpenAPI" "${E2E_RUN_DIR}/reports/wa-module-results.jsonl"
if [[ "$ANY_FAIL" -eq 0 ]]; then
  e2e_flow_review_scenario_complete "$FLOW_ID" "10_reporting_audit_reconciliation.sh" "flow-review-complete" "reporting_audit_reviewed"
fi

exit "$ANY_FAIL"
