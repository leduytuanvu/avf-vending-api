#!/usr/bin/env bash
# Emit mosquitto client availability + TCP probe to stdout (for reports/test/mqtt-evidence).
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=../../tests/e2e/lib/e2e_common.sh
source "${ROOT}/tests/e2e/lib/e2e_common.sh"
e2e_prepend_windows_tool_paths || true

echo "=== mosquitto clients ==="
if command -v mosquitto_pub >/dev/null 2>&1; then
  echo "mosquitto_pub: $(command -v mosquitto_pub)"
else
  echo "mosquitto_pub: NOT_ON_PATH"
fi
if command -v mosquitto_sub >/dev/null 2>&1; then
  echo "mosquitto_sub: $(command -v mosquitto_sub)"
else
  echo "mosquitto_sub: NOT_ON_PATH"
fi

echo ""
echo "=== TCP probe (MQTT_PORT default 1883) ==="
: "${MQTT_HOST:=127.0.0.1}"
: "${MQTT_PORT:=1883}"
host="${MQTT_HOST%%:*}"
port="${MQTT_PORT:-1883}"
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
