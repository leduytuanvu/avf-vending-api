#!/usr/bin/env bash
# Fleet-scale production gate: validate telemetry-storm-result.json (see deploy-prod workflow).
# Environment: see validate_production_scale_storm_evidence.py docstring.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec python3 "${SCRIPT_DIR}/validate_production_scale_storm_evidence.py"
