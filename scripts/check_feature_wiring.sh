#!/usr/bin/env bash
# Repository-level wiring invariants: enabled features must keep their startup validation hooks.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT"

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

require_substring() {
  local file="$1"
  local needle="$2"
  if [[ ! -f "$file" ]]; then
    fail "missing file: $file"
  fi
  if ! grep -qF "$needle" "$file"; then
    fail "$file must contain: $needle"
  fi
}

echo "== feature wiring string checks =="

require_substring "cmd/reconciler/main.go" "ValidateReconciler"
require_substring "cmd/reconciler/main.go" "BuildReconcilerDeps"

require_substring "internal/bootstrap/api.go" "ValidateRuntimeWiring"

# Reconciler bootstrap must construct real adapters when actions are enabled (not a stub return path only).
require_substring "internal/bootstrap/reconciler.go" "NewHTTPStatusGateway"
require_substring "internal/bootstrap/reconciler.go" "ConnectJetStream"
require_substring "internal/bootstrap/reconciler.go" "NewNATSCoreRefundReviewSink"

echo "OK: feature wiring hooks present."
