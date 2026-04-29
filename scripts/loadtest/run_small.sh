#!/usr/bin/env bash
# Minimal local check: build + dry-run (no credentials, no network).
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"
make loadtest-small
