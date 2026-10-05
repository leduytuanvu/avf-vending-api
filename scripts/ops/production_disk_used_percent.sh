#!/usr/bin/env bash
# Print root filesystem used percent (integer 0-100) for automation thresholds.
set -Eeuo pipefail
mount="${1:-/}"
df -P "${mount}" | awk 'NR==2 { gsub(/%/, "", $5); print $5 }'
