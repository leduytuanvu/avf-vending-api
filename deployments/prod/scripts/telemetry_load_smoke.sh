#!/usr/bin/env bash
# Historical name: this entrypoint now delegates to the automated storm load test.
# Default is dry-run (no MQTT, no credentials). See telemetry_storm_load_test.sh for full options.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec bash "${SCRIPT_DIR}/telemetry_storm_load_test.sh" "$@"
