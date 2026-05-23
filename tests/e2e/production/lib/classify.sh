#!/usr/bin/env bash
# shellcheck shell=bash
# Failure classification and contract drift hints.

prod_e2e_fail_classify() {
  local kind="$1"
  local flow_id="$2"
  local msg="$3"
  local label
  case "$kind" in
    a) label="test harness bug" ;;
    b) label="documentation/MD bug" ;;
    c) label="backend contract bug" ;;
    d) label="production env/config bug" ;;
    e) label="data setup bug" ;;
    *) label="unknown" ;;
  esac
  echo "FAIL_CLASSIFICATION flow=${flow_id} kind=${kind} (${label}): ${msg}" >&2
  echo "${flow_id}|${kind}|${msg}" >>"${PROD_E2E_RUN_DIR}/failures.classification.txt"
}
