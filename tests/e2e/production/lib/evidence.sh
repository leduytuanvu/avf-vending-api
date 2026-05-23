#!/usr/bin/env bash
# shellcheck shell=bash
# Evidence markdown assembly (secrets redacted).

prod_e2e_evidence_init() {
  PROD_E2E_RESULT_MD="${PROD_E2E_RUN_DIR}/RESULT.md"
  export PROD_E2E_RESULT_MD
  cat >"${PROD_E2E_RESULT_MD}" <<EOF
# Production E2E Result

- run_id: \`${PROD_E2E_RUN_ID}\`
- prefix: \`${PROD_E2E_PREFIX}\`
- base_url: \`${BASE_URL:-}\`
- started: \`$(date -u +%Y-%m-%dT%H:%M:%SZ)\`

## Flow results

| id | label | protocol | status | evidence |
|----|-------|----------|--------|----------|
EOF
}

prod_e2e_evidence_append_row() {
  local id="$1"
  local label="$2"
  local protocol="$3"
  local status="$4"
  local evidence_label="$5"
  printf '| %s | %s | %s | %s | `%s` |\n' "$id" "$label" "$protocol" "$status" "${evidence_label}" >>"${PROD_E2E_RESULT_MD}"
}

prod_e2e_evidence_append_section() {
  local flow_id="$1"
  local evidence_label="$2"
  local method_path="$3"
  local http_code="$4"
  local req_file="$5"
  local resp_file="$6"
  local redacted_req="${PROD_E2E_RAW_DIR}/${evidence_label}.request.redacted.json"
  local redacted_resp="${PROD_E2E_RAW_DIR}/${evidence_label}.response.redacted.json"
  [[ -f "$req_file" ]] && prod_e2e_redact_file "$req_file" "$redacted_req"
  [[ -f "$resp_file" ]] && prod_e2e_redact_file "$resp_file" "$redacted_resp"
  {
    echo ""
    echo "### ${flow_id} — ${evidence_label}"
    echo ""
    echo "- method/path: \`${method_path}\`"
    echo "- status: \`${http_code}\`"
    if [[ -f "$redacted_req" ]]; then
      echo "- request (redacted):"
      echo '```json'
      cat "$redacted_req"
      echo '```'
    fi
    if [[ -f "$redacted_resp" ]]; then
      echo "- response (redacted):"
      echo '```json'
      cat "$redacted_resp"
      echo '```'
    fi
  } >>"${PROD_E2E_RESULT_MD}"
}

prod_e2e_evidence_append_grpc_section() {
  local flow_id="$1"
  local evidence_label="$2"
  local rpc="$3"
  local code="$4"
  {
    echo ""
    echo "### ${flow_id} — ${evidence_label} (gRPC)"
    echo ""
    echo "- rpc: \`${rpc}\`"
    echo "- code: \`${code}\`"
    echo "- raw: \`${PROD_E2E_RAW_DIR}/${evidence_label}.*\`"
  } >>"${PROD_E2E_RESULT_MD}"
}

prod_e2e_evidence_append_mqtt_section() {
  local flow_id="$1"
  local evidence_label="$2"
  local topic="$3"
  {
    echo ""
    echo "### ${flow_id} — ${evidence_label} (MQTT)"
    echo ""
    echo "- topic: \`${topic}\`"
    echo "- raw: \`${PROD_E2E_RAW_DIR}/${evidence_label}.*\`"
  } >>"${PROD_E2E_RESULT_MD}"
}

prod_e2e_evidence_finalize() {
  {
    echo ""
    echo "## Postman parity"
    echo ""
    echo "Collection generated from \`tests/e2e/production/e2e-manifest.yaml\`."
    echo "gRPC and MQTT flows are documented above — not duplicated as fake Postman requests."
    echo ""
    echo "## Cleanup policy"
    echo ""
    echo "Only resources with prefix \`${PROD_E2E_PREFIX}\` may be deleted. Non-E2E resources are never removed by this harness."
  } >>"${PROD_E2E_RESULT_MD}"
}
