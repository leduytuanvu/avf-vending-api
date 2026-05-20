#!/usr/bin/env bash
# Adjacent MQTT runner — delegates to repo E2E harness (mosquitto).
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$ROOT"
exec bash tests/e2e/run-mqtt-local.sh "$@"
