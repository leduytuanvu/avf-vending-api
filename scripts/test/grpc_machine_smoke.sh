#!/usr/bin/env bash
# Smoke-check machine gRPC health after deployment or locally (plaintext).
# Requires grpcurl on PATH. Override GRPC_SMOKE_TARGET (default localhost:9090).
set -euo pipefail

TARGET="${GRPC_SMOKE_TARGET:-localhost:9090}"

if ! command -v grpcurl >/dev/null 2>&1; then
	echo "grpcurl not found on PATH; install https://github.com/fullstorydev/grpcurl" >&2
	exit 127
fi

grpcurl -plaintext "${TARGET}" grpc.health.v1.Health/Check
echo "grpc_machine_smoke: OK (${TARGET})"
