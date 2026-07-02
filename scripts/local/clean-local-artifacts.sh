#!/usr/bin/env bash
# Remove gitignored local temp/test/e2e artifacts. Dry-run by default; pass --apply to delete.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

APPLY=0
if [[ "${1:-}" == "--apply" ]]; then
  APPLY=1
elif [[ -n "${1:-}" ]]; then
  echo "usage: $0 [--apply]" >&2
  exit 2
fi

# Top-level directories (relative to repo root)
DIR_PATTERNS=(
  .tmp
  .tmp-*
  .go-tmp
  tmp
  temp
  .cache
  .test-runs
  .e2e-runs
  tests/e2e/.e2e-runs
  .production-smoke-runs
  .production-latency-runs
  ci-reports
  security-reports
  coverage
  dist
  bin
)

# Individual files / globs at repo root or anywhere
FILE_GLOBS=(
  repomix-output*.xml
  '**/newman-report.json'
  '**/newman-junit.xml'
)

is_tracked() {
  local p="$1"
  [[ -n "$(git -C "$ROOT" ls-files -- "$p" 2>/dev/null)" ]]
}

collect_dirs() {
  local d pat
  for pat in "${DIR_PATTERNS[@]}"; do
    for d in $pat; do
      [[ -e "$d" ]] || continue
      if [[ -d "$d" ]]; then
        printf '%s\n' "$d"
      fi
    done
  done
  # migration-evidence JSON only (keep directory for CI artifact path)
  [[ -f migration-evidence/migration-safety-report.json ]] && printf '%s\n' migration-evidence/migration-safety-report.json
}

collect_files() {
  local g
  for g in "${FILE_GLOBS[@]}"; do
    find . -path "./.git" -prune -o -name "${g#**/}" -type f -print 2>/dev/null || true
  done | sort -u
  find . -path "./.git" -prune -o \( -name '*.log' -o -name '*.bak' -o -name '*.old' -o -name '*.orig' \) -type f -print 2>/dev/null | sort -u || true
}

CANDIDATES=()
while IFS= read -r line; do
  [[ -n "$line" ]] && CANDIDATES+=("$line")
done < <(collect_dirs | sort -u)

while IFS= read -r line; do
  [[ -n "$line" ]] && CANDIDATES+=("${line#./}")
done < <(collect_files | sort -u)

if [[ ${#CANDIDATES[@]} -eq 0 ]]; then
  echo "No local artifact candidates found."
  exit 0
fi

echo "Local artifact candidates (${#CANDIDATES[@]}):"
SKIPPED=0
TO_DELETE=()
for p in "${CANDIDATES[@]}"; do
  if is_tracked "$p"; then
    echo "  SKIP (tracked): $p"
    SKIPPED=$((SKIPPED + 1))
  else
    echo "  $p"
    TO_DELETE+=("$p")
  fi
done

if [[ $SKIPPED -gt 0 ]]; then
  echo "Refused to delete $SKIPPED tracked path(s)." >&2
fi

if [[ $APPLY -eq 0 ]]; then
  echo ""
  echo "Dry-run only. Re-run with --apply to delete ${#TO_DELETE[@]} path(s)."
  exit 0
fi

for p in "${TO_DELETE[@]}"; do
  rm -rf "$p"
done

echo "Deleted ${#TO_DELETE[@]} path(s)."
git -C "$ROOT" status --short
