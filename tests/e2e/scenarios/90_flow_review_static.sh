#!/usr/bin/env bash
# shellcheck shell=bash
# E2E-FR-90: Static flow-review — repo/docs/Postman/proto/scenario coverage (no HTTP mutations).

set +e
set -u

E2E_SCENARIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/e2e_common.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_common.sh"
# shellcheck source=../lib/e2e_flow_review.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_flow_review.sh"
# shellcheck source=../lib/e2e_grpc.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_grpc.sh"

FLOW_ID="E2E-FR-90"
SCEN="$(basename "${BASH_SOURCE[0]}")"
IMP="$(e2e_improvement_findings_path)"
IMP_BEFORE="$(wc -l <"${IMP}" 2>/dev/null | tr -d ' ')"
[[ -z "${IMP_BEFORE}" ]] && IMP_BEFORE=0

start_step "flow-review-static"

MATRIX="${E2E_REPO_ROOT}/docs/testing/e2e-flow-coverage.md"
MQTT_DOC="${E2E_REPO_ROOT}/docs/api/mqtt-contract.md"
: "${POSTMAN_COLLECTION:=docs/postman/avf-vending-api-function-path.postman_collection.json}"
POSTMAN_ABS="${POSTMAN_COLLECTION}"
[[ "${POSTMAN_ABS}" != /* ]] && POSTMAN_ABS="${E2E_REPO_ROOT}/${POSTMAN_ABS}"

STATIC_OUT="${E2E_RUN_DIR}/reports/flow-review-static.json"
mkdir -p "${E2E_RUN_DIR}/reports"

# Matrix rows with **scripted** automation should tie to a scenario file (FLOW_ID text or backtick *.sh in the row).
missing_ref=0
matrix_scripted_checked=0
if [[ -f "$MATRIX" ]]; then
while IFS= read -r line; do
  [[ "$line" =~ ^\|[[:space:]]+(WA-|VM-|GRPC-|MQTT-|E2E-|PF-|SP-) ]] || continue
  mid="$(echo "$line" | awk -F'|' '{
    gsub(/^[ \t]+|[ \t]+$/,"",$2)
    print $2
  }')"
  auto="$(echo "$line" | awk -F'|' '{
    gsub(/^[ \t]+|[ \t]+$/,"",$10)
    print $10
  }')"
  [[ -z "$mid" || "$mid" == "flow_id" ]] && continue
  [[ "$auto" =~ \*\*scripted\*\* ]] || continue
  matrix_scripted_checked=$((matrix_scripted_checked + 1))
  hit=""
  for sf in "${E2E_SCENARIO_DIR}"/*.sh; do
    [[ -f "$sf" ]] || continue
    if grep -qF "$mid" "$sf" 2>/dev/null; then
      hit="1"
      break
    fi
  done
  if [[ -z "$hit" ]]; then
    while IFS= read -r bt; do
      [[ -z "$bt" ]] && continue
      local_bn="${bt##*/}"
      if [[ -f "${E2E_SCENARIO_DIR}/${local_bn}" ]]; then
        hit="1"
        break
      fi
    done < <(grep -oE '`[0-9][a-zA-Z0-9_./-]*\.sh`' <<<"$line" | tr -d '`' || true)
  fi
  if [[ -z "$hit" ]]; then
    log_flow_design_issue "P3" "$FLOW_ID" "$SCEN" "matrix-${mid}" "docs" "$mid" \
      "Flow id ${mid} is **scripted** in e2e-flow-coverage.md but not referenced by scenario FLOW_ID text and no backtick .sh in that row resolves under tests/e2e/scenarios/" \
      "Automation ownership is unclear" \
      "Reference the flow id from a scenario or link the exact .sh name in the matrix row" \
      "$MATRIX"
    missing_ref=$((missing_ref + 1))
  fi
done <"${MATRIX}"
fi

# --- Postman collection ---
postman_paths=0
postman_has_health=""
if [[ -f "$POSTMAN_ABS" ]]; then
  postman_paths="$(
    jq '[.. | objects | select(has("request")) | .request.url |
        if type == "string" then . else (.raw // "") end] | map(select(. != "")) | length' \
      "$POSTMAN_ABS" 2>/dev/null || echo 0
  )"
  if jq -e '.. | strings | test("health/live")' "$POSTMAN_ABS" >/dev/null 2>&1; then
    postman_has_health="true"
  fi
else
  log_postman_issue "P2" "$FLOW_ID" "$SCEN" "postman-missing" "docs" "${POSTMAN_COLLECTION}" \
    "Postman collection not found at ${POSTMAN_ABS}" \
    "Cannot verify REST request coverage vs matrix" \
    "Add export or fix POSTMAN_COLLECTION path" \
    "${STATIC_OUT}"
fi

if [[ -f "$POSTMAN_ABS" ]] && [[ "${postman_paths:-0}" -eq 0 ]]; then
  log_postman_issue "P2" "$FLOW_ID" "$SCEN" "postman-empty" "Postman" "${POSTMAN_COLLECTION}" \
    "Collection parses but exposes no request URLs for coverage comparison" \
    "Phase 9 / Newman mapping ineffective" \
    "Validate Postman export structure" \
    "$POSTMAN_ABS"
fi

if [[ -f "$POSTMAN_ABS" ]] && [[ "${postman_has_health}" != "true" ]]; then
  log_postman_issue "P3" "$FLOW_ID" "$SCEN" "postman-health" "Postman" "GET /health/live" \
    "Postman collection does not mention health/live (WA-001 baseline)" \
    "Ops smoke not represented in collection" \
    "Add health/version requests or document exclusion" \
    "$POSTMAN_ABS"
fi

# --- Docs matrix presence ---
if [[ ! -f "$MATRIX" ]]; then
  log_docs_issue "P1" "$FLOW_ID" "$SCEN" "coverage-matrix" "docs" "e2e-flow-coverage.md" \
    "docs/testing/e2e-flow-coverage.md missing from repo checkout" \
    "Cannot align scenarios to business flows" \
    "Restore coverage matrix" \
    "${STATIC_OUT}"
fi

# --- MQTT topic documentation ---
if [[ ! -f "$MQTT_DOC" ]]; then
  log_docs_issue "P2" "$FLOW_ID" "$SCEN" "mqtt-doc" "docs" "mqtt-contract.md" \
    "docs/api/mqtt-contract.md not found" \
    "MQTT scenarios cannot derive topics from documentation" \
    "Restore MQTT contract doc" \
    "${STATIC_OUT}"
else
  if ! grep -qi "telemetry" "$MQTT_DOC" || ! grep -qiE 'command|ack|receipt|topic' "$MQTT_DOC"; then
    log_docs_issue "P3" "$FLOW_ID" "$SCEN" "mqtt-topic-matrix" "docs" "mqtt-contract.md" \
      "MQTT contract doc missing expected telemetry / command topic terminology" \
      "Topic layout harder to validate in review mode" \
      "Expand mqtt-contract.md with PF-* topic templates" \
      "$MQTT_DOC"
  fi
fi

# --- Proto RPC spot-checks ---
while IFS= read -r line; do
  [[ -z "$line" || "$line" =~ ^# ]] && continue
  svc="${line%%,*}"
  rpc="${line#*,}"
  if ! e2e_grpc_rpc_declared_in_repo "$svc" "$rpc"; then
    log_missing_endpoint_issue "P2" "$FLOW_ID" "$SCEN" "proto-${svc}-${rpc}" "gRPC" "${svc}/${rpc}" \
      "Expected vending proto rpc ${svc}.${rpc} not found under proto/avf/machine/v1" \
      "Matrix VM-* / GRPC-* rows cannot be contract-tested from repo protos" \
      "Sync protos; update matrix if RPC renamed" \
      "${E2E_REPO_ROOT}/proto"
  fi
done <<'EOF'
MachineAuthService,ClaimActivation
MachineBootstrapService,GetBootstrap
MachineCatalogService,GetSaleCatalog
MachineCommerceService,CreateOrder
MachineOfflineSyncService,PushOfflineEvents
EOF

proto_rpc_count="0"
if [[ -d "${E2E_REPO_ROOT}/proto/avf/machine/v1" ]]; then
  proto_rpc_count="$(
    grep -hE '^\s*rpc\s+[A-Za-z0-9_]+\s*\(' "${E2E_REPO_ROOT}"/proto/avf/machine/v1/*.proto 2>/dev/null \
      | wc -l | tr -d ' '
  )"
fi

# --- Destructive REST helpers without allow-writes / guard mentions ---
while IFS= read -r f; do
  [[ -f "$f" ]] || continue
  base="$(basename "$f")"
  [[ "$base" == "90_flow_review_static.sh" || "$base" == "91_flow_review_existing_data.sh" ]] && continue
  if grep -qE 'e2e_http_(post|put|patch|delete)_json|e2e_http_post_json|e2e_http_put_json|e2e_http_patch_json' "$f" 2>/dev/null \
    || grep -qE 'curl(\([^)]*\))*[[:space:]]+-X[[:space:]]*(POST|PUT|PATCH|DELETE)' "$f" 2>/dev/null; then
    if ! grep -qE 'E2E_ALLOW_WRITES|grpc_contract_try|production.*guard|E2E_PRODUCTION|prod-safe|log_production_safety' "$f"; then
      log_production_safety_issue "P2" "$FLOW_ID" "$SCEN" "allow-writes-guard" "harness" "$base" \
        "Scenario appears to perform HTTP mutations but lacks E2E_ALLOW_WRITES / production guard references — review for accidental prod writes" \
        "Unsafe if a runner enables writes against production" \
        "Gate mutations on E2E_ALLOW_WRITES; document production guards" \
        "$f"
    fi
  fi
done < <(find "${E2E_SCENARIO_DIR}" -maxdepth 1 -name '*.sh' | LC_ALL=C sort)

jq -nc \
  --arg gen "$(now_utc)" \
  --arg mode "static" \
  --argjson matrix_scripted_rows_considered "$matrix_scripted_checked" \
  --argjson missing_matrix_ref "$missing_ref" \
  --argjson postman_requests "${postman_paths:-0}" \
  --argjson postman_health "$([[ "${postman_has_health}" == true ]] && echo true || echo false)" \
  --argjson proto_rpc_count "${proto_rpc_count:-0}" \
  --argjson mqtt_doc_present "$([[ -f "$MQTT_DOC" ]] && echo true || echo false)" \
  '{
    generatedAt:$gen,
    mode:$mode,
    matrixScriptedRowsConsidered:$matrix_scripted_rows_considered,
    matrixScriptedWithoutScenarioReference:$missing_matrix_ref,
    postmanRequestLikePaths:$postman_requests,
    postmanMentionsHealthLive:$postman_health,
    protoMachineV1RpcLines:$proto_rpc_count,
    mqttContractDocPresent:$mqtt_doc_present
  }' >"${STATIC_OUT}"

append_event_jsonl "flow-review-static" "passed" "matrix_scripted_rows=${matrix_scripted_checked} missing_refs=${missing_ref} postman_paths=${postman_paths:-0}"

end_step passed "flow-review static analysis complete"

IMP_AFTER="$(wc -l <"${IMP}" 2>/dev/null | tr -d ' ')"
[[ -z "${IMP_AFTER}" ]] && IMP_AFTER=0
if [[ "${IMP_AFTER}" -eq "${IMP_BEFORE}" ]]; then
  log_no_improvement_findings "$FLOW_ID" "$SCEN" "static-review-clean"
fi

e2e_flow_review_scenario_complete "$FLOW_ID" "$SCEN" "flow-review-static" "static_review_pass"

exit 0
