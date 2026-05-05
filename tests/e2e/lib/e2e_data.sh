#!/usr/bin/env bash
# shellcheck shell=bash
# Test-data.json + secrets.private.json helpers. Requires e2e_common.sh (jq, E2E_RUN_DIR).

: "${E2E_TEST_DATA_FILE:=${E2E_RUN_DIR}/test-data.json}"
: "${E2E_SECRETS_FILE:=${E2E_RUN_DIR}/secrets.private.json}"

e2e_data_paths() {
  E2E_TEST_DATA_FILE="${E2E_RUN_DIR}/test-data.json"
  E2E_SECRETS_FILE="${E2E_RUN_DIR}/secrets.private.json"
}

e2e_data_initialize() {
  e2e_data_paths
  mkdir -p "$(dirname "$E2E_TEST_DATA_FILE")"

  local reuse="false"
  if [[ "${E2E_REUSE_DATA:-false}" == "true" ]] && [[ -n "${E2E_DATA_FILE:-}" ]]; then
    reuse="true"
  fi

  if [[ "$reuse" == "true" ]]; then
    if [[ ! -f "${E2E_DATA_FILE}" ]]; then
      echo "FATAL: E2E reuse data file not found: ${E2E_DATA_FILE}" >&2
      exit 2
    fi
    cp "${E2E_DATA_FILE}" "${E2E_TEST_DATA_FILE}"
    if [[ ! -f "${E2E_SECRETS_FILE}" ]]; then
      echo '{}' >"${E2E_SECRETS_FILE}"
    fi
  else
    echo '{}' >"${E2E_TEST_DATA_FILE}"
    echo '{}' >"${E2E_SECRETS_FILE}"
  fi
}

e2e_set_data() {
  local key="$1"
  local val="$2"
  e2e_data_paths
  local tmp
  tmp="$(mktemp)"
  jq --arg k "$key" --arg v "$val" '. + {($k): $v}' "${E2E_TEST_DATA_FILE}" >"${tmp}" && mv "${tmp}" "${E2E_TEST_DATA_FILE}"
}

e2e_set_data_json() {
  local key="$1"
  local json="$2"
  e2e_data_paths
  local tmp
  tmp="$(mktemp)"
  jq --arg k "$key" --argjson v "$json" '. + {($k): $v}' "${E2E_TEST_DATA_FILE}" >"${tmp}" && mv "${tmp}" "${E2E_TEST_DATA_FILE}"
}

e2e_get_data() {
  local key="$1"
  e2e_data_paths
  jq -r --arg k "$key" '.[$k] // empty' "${E2E_TEST_DATA_FILE}"
}

e2e_require_data() {
  local key="$1"
  local v
  v="$(e2e_get_data "$key")"
  if [[ -z "$v" ]] || [[ "$v" == "null" ]]; then
    echo "FATAL: required test data key missing in ${E2E_TEST_DATA_FILE}: $key" >&2
    exit 2
  fi
}

# Full value in secrets.private.json; masked placeholder in test-data.json (for summaries / sharing)
e2e_save_token() {
  local key="$1"
  local val="$2"
  e2e_data_paths
  local tmp
  tmp="$(mktemp)"
  jq --arg k "$key" --arg v "$val" '. + {($k): $v}' "${E2E_SECRETS_FILE}" >"${tmp}" && mv "${tmp}" "${E2E_SECRETS_FILE}"
  local masked
  masked="$(mask_secret "$val")"
  e2e_set_data "$key" "$masked"
}

e2e_get_secret() {
  local key="$1"
  e2e_data_paths
  jq -r --arg k "$key" '.[$k] // empty' "${E2E_SECRETS_FILE}"
}

# Aliases for scenario scripts (spec names)
initialize_test_data() { e2e_data_initialize; }
set_data() { e2e_set_data "$@"; }
set_data_json() { e2e_set_data_json "$@"; }
get_data() { e2e_get_data "$@"; }
require_data() { e2e_require_data "$@"; }
save_token() { e2e_save_token "$@"; }
get_secret() { e2e_get_secret "$@"; }
