#!/usr/bin/env bash
# Fail if documentation contradicts shipped P0 HTTP surfaces (mounted + OpenAPI).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

die() {
  echo "check_stale_p0_docs: $*" >&2
  exit 1
}

hits=0
while IFS= read -r -d '' f; do
  case "$f" in
    */docs/api/roadmap.md | */docs/runbooks/enterprise-api-backend-audit-report.md)
      continue
      ;;
  esac
  if grep -q -e 'Not applied in-repo' -e 'not applied in-repo' -e 'Until those routes are mounted' "$f" 2>/dev/null; then
    grep -n -e 'Not applied in-repo' -e 'not applied in-repo' -e 'Until those routes are mounted' "$f" 2>/dev/null | sed "s|^|  $f:|" || true
    hits=$((hits + 1))
  fi
  # Phrase "are not mounted" is only stale when it implies Chi (not MQTT transport).
  if grep -q 'are not mounted.*Chi' "$f" 2>/dev/null; then
    grep -n 'are not mounted.*Chi' "$f" 2>/dev/null | sed "s|^|  $f:|" || true
    hits=$((hits + 1))
  fi
done < <(find docs -name '*.md' -print0)

if [[ "$hits" -gt 0 ]]; then
  die "found stale P0 doc contradiction(s) — fix lines above or move content to docs/api/roadmap.md"
fi

echo "OK: no stale P0 mounting / handoff contradictions in docs/ (excluding roadmap + historical audit snapshot)"
