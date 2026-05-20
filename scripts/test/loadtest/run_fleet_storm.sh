#!/usr/bin/env bash
# Fleet-scale storm: executes avf-loadtest suite/storm with -machines N and -execute.
# Requires LOADTEST_MACHINE_MANIFEST (and staging-style creds). Never sets LOAD_TEST_ENV=production.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

N="${1:-100}"
if [[ $# -ge 1 ]]; then
  shift
fi

export LOAD_TEST_ENV="${LOAD_TEST_ENV:-staging}"

if [[ -z "${LOADTEST_MACHINE_MANIFEST:-}" ]]; then
  echo "run_fleet_storm: set LOADTEST_MACHINE_MANIFEST to a TSV/JSON fleet manifest" >&2
  exit 2
fi

if [[ "${N}" == "1000" ]] && [[ "${LOAD_TEST_ENV:-}" == "local" ]]; then
  echo "run_fleet_storm: 1000-machine runs are usually run against staging; LOAD_TEST_ENV=local set (override if intentional)." >&2
fi

HTTP_BASE="${API_BASE_URL:-${AVF_LOADTEST_HTTP_BASE:-http://localhost:8080}}"
GRPC_EP="${GRPC_ADDR:-${AVF_LOADTEST_GRPC_ADDR:-localhost:9090}}"

exec go run ./tools/loadtest/cmd/avf-loadtest \
  -scenario storm \
  -http-base "${HTTP_BASE}" \
  -grpc-addr "${GRPC_EP}" \
  -metrics-url "${AVF_LOADTEST_METRICS_URL:-}" \
  -machines "${N}" \
  -execute \
  "$@"
