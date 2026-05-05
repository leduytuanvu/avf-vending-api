#!/usr/bin/env bash
# shellcheck shell=bash
# Read-only REST smoke: public GETs only; optional routes skip on 404.

E2E_SCENARIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/e2e_common.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_common.sh"
# shellcheck source=../lib/e2e_http.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_http.sh"

for path in "/health/live" "/health/ready" "/version"; do
  step="ro-$(echo "${path#/}" | tr '/' '-')"
  if ! e2e_http_get_capture "$step" "$path" "required" "false"; then
    fail_step "read-only smoke required GET failed: ${path}"
    exit 1
  fi
done

for tuple in "/swagger/doc.json|ro-swagger-doc-json" "/metrics|ro-metrics"; do
  IFS='|' read -r path step <<<"$tuple"
  if ! e2e_http_get_capture "$step" "$path" "optional404" "false"; then
    fail_step "optional read-only GET failed (non-404): ${path}"
    exit 1
  fi
done

append_event_jsonl "rest-readonly" "passed" "required + optional GETs completed"

e2e_flow_review_scenario_complete "WA-001-public" "00_rest_readonly_smoke.sh" "flow-review-complete" "readonly_smoke_ok_no_scenario_findings"

exit 0
