#!/usr/bin/env bash
# gRPC client preflight: grpcurl on PATH + TCP probe for GRPC_ADDR (default 127.0.0.1:9090).
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=../../tests/e2e/lib/e2e_common.sh
source "${ROOT}/tests/e2e/lib/e2e_common.sh"

: "${GRPC_ADDR:=127.0.0.1:9090}"
host="${GRPC_ADDR%%:*}"
port="${GRPC_ADDR##*:}"

echo "=== grpcurl ==="
if command -v grpcurl >/dev/null 2>&1; then
  echo "grpcurl: $(command -v grpcurl)"
  grpcurl --version 2>&1 || true
else
  echo "grpcurl: NOT_ON_PATH"
fi

echo ""
echo "=== TCP ${GRPC_ADDR} ==="
if command -v timeout >/dev/null 2>&1; then
  if timeout 2 bash -c "echo >/dev/tcp/${host}/${port}" >/dev/null 2>&1; then
    echo "tcp://${host}:${port} OPEN"
  else
    echo "tcp://${host}:${port} CLOSED_OR_TIMEOUT"
  fi
else
  if bash -c "echo >/dev/tcp/${host}/${port}" >/dev/null 2>&1; then
    echo "tcp://${host}:${port} OPEN"
  else
    echo "tcp://${host}:${port} CLOSED_OR_TIMEOUT"
  fi
fi
