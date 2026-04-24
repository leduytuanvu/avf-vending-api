#!/usr/bin/env bash
# Enterprise static release verification: tests, shell syntax, YAML structure (optional),
# Docker Compose config for production examples, and obvious secret leakage in example files.
#
# Does not connect to production or require real secrets. Example env files use placeholders.
#
# Environment (optional):
#   VERIFY_ENTERPRISE_SKIP_DOCKER=1  Skip docker compose config phases (local dev without Docker).
#   VERIFY_ENTERPRISE_SKIP_YAML=1    Skip YAML parse phase.
#   VERIFY_ENTERPRISE_SKIP_GO=1      Skip go test (not recommended).

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

section() {
  echo ""
  echo "=== $* ==="
}

die() {
  echo "ERROR: $*" >&2
  exit 1
}

count_sh_files() {
  find deployments scripts -type f -name '*.sh' 2>/dev/null | wc -l | tr -d ' '
}

phase_shell_syntax() {
  section "Phase 1: Bash syntax (bash -n) for deployments/**/*.sh and scripts/**/*.sh"
  local n
  n="$(count_sh_files)"
  echo "Found ${n} shell scripts."
  local files=()
  mapfile -t files < <(find deployments scripts -type f -name '*.sh' | LC_ALL=C sort)
  local f
  for f in "${files[@]}"; do
    echo "  bash -n: ${f}"
    bash -n "$f"
  done
  echo "OK: shell syntax"
}

line_looks_placeholder_ok() {
  # Allow obvious template / doc lines.
  [[ "$1" =~ (CHANGE_ME|PLACEHOLDER|REDACTED|YOUR_|INSERT_|REPLACE_ME|example\.com|myorg|ldtv\.dev|127\.0\.0\.1|localhost) ]]
}

phase_example_secret_scan() {
  section "Phase 2: Example / template files — block obvious live secrets"
  local f
  local hits=0
  local ex_files=()
  mapfile -t ex_files < <(find deployments -type f \( -name '*.example' -o -name '.env.*.example' \) | LC_ALL=C sort)
  for f in "${ex_files[@]}"; do
    [[ -f "$f" ]] || continue
    while IFS= read -r line; do
      [[ "$line" =~ ^[[:space:]]*# ]] && continue
      [[ -z "${line// }" ]] && continue
      if line_looks_placeholder_ok "$line"; then
        continue
      fi
      if echo "$line" | grep -Eq 'sk_live_[a-zA-Z0-9]+|pk_live_[a-zA-Z0-9]+'; then
        echo "  BLOCKED (live payment key pattern): $f"
        echo "    $line"
        hits=$((hits + 1))
        continue
      fi
      if echo "$line" | grep -Eq 'AKIA[0-9A-Z]{16}|ASIA[0-9A-Z]{16}'; then
        echo "  BLOCKED (AWS key id pattern): $f"
        echo "    $line"
        hits=$((hits + 1))
        continue
      fi
      if echo "$line" | grep -Eq 'xox[baprs]-[A-Za-z0-9-]+'; then
        echo "  BLOCKED (Slack token pattern): $f"
        echo "    $line"
        hits=$((hits + 1))
        continue
      fi
      if echo "$line" | grep -Eq 'ghp_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]+'; then
        echo "  BLOCKED (GitHub token pattern): $f"
        echo "    $line"
        hits=$((hits + 1))
        continue
      fi
      if echo "$line" | grep -Fq 'PRIVATE KEY-----'; then
        echo "  BLOCKED (PEM private key material): $f"
        echo "    $line"
        hits=$((hits + 1))
        continue
      fi
      # JWT / opaque bearer-like blobs (three dot-separated base64url chunks).
      if echo "$line" | grep -Eq 'eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+'; then
        echo "  BLOCKED (JWT-shaped token): $f"
        echo "    $line"
        hits=$((hits + 1))
        continue
      fi
    done <"$f"
  done

  if [[ "$hits" -gt 0 ]]; then
    die "example secret scan failed (${hits} finding(s)); use CHANGE_ME_* placeholders only"
  fi
  echo "OK: example files passed live-secret heuristics"

  section "Phase 2b: Example env — no empty required assignments"
  local empty_hits=0
  for f in "${ex_files[@]}"; do
    [[ -f "$f" ]] || continue
    while IFS= read -r line; do
      [[ "$line" =~ ^[[:space:]]*# ]] && continue
      [[ -z "${line// }" ]] && continue
      if echo "$line" | grep -Eq '^[A-Za-z_][A-Za-z0-9_]*=[[:space:]]*$'; then
        echo "  empty value: $f -> $line"
        empty_hits=$((empty_hits + 1))
      fi
    done <"$f"
  done

  if [[ "$empty_hits" -gt 0 ]]; then
    die "example env files contain empty assignments (${empty_hits}); use explicit CHANGE_ME placeholders"
  fi

  echo "OK: example / template secret posture"
}

phase_yaml_optional() {
  section "Phase 3: YAML structure (optional — PyYAML)"
  if [[ "${VERIFY_ENTERPRISE_SKIP_YAML:-}" == "1" ]]; then
    echo "SKIP: VERIFY_ENTERPRISE_SKIP_YAML=1"
    return 0
  fi
  if ! command -v python3 >/dev/null 2>&1; then
    echo "SKIP: python3 not on PATH"
    return 0
  fi
  if ! python3 -c 'import yaml' 2>/dev/null; then
    echo "SKIP: PyYAML not installed (e.g. apt install python3-yaml or pip install pyyaml)"
    return 0
  fi
  local yaml_files=()
  mapfile -t yaml_files < <(find deployments \( -name '*.yml' -o -name '*.yaml' \) -type f | LC_ALL=C sort)
  local f
  for f in "${yaml_files[@]}"; do
    echo "  yaml parse: $f"
    python3 -c 'import sys, yaml; yaml.safe_load(open(sys.argv[1], "r", encoding="utf-8"))' "$f"
  done
  echo "OK: YAML parse"
}

phase_docker_compose() {
  section "Phase 4: Docker Compose config (example env — no cluster contact)"
  if [[ "${VERIFY_ENTERPRISE_SKIP_DOCKER:-}" == "1" ]]; then
    echo "SKIP: VERIFY_ENTERPRISE_SKIP_DOCKER=1"
    return 0
  fi
  if ! command -v docker >/dev/null 2>&1; then
    die "docker not on PATH (install Docker or set VERIFY_ENTERPRISE_SKIP_DOCKER=1 for a partial run)"
  fi
  local dc=(docker compose)

  echo "  app-node (default)"
  "${dc[@]}" --env-file deployments/prod/app-node/.env.app-node.example \
    -f deployments/prod/app-node/docker-compose.app-node.yml config >/dev/null

  echo "  app-node (profile: temporal)"
  "${dc[@]}" --env-file deployments/prod/app-node/.env.app-node.example \
    -f deployments/prod/app-node/docker-compose.app-node.yml --profile temporal config >/dev/null

  echo "  app-node (profile: migration)"
  "${dc[@]}" --env-file deployments/prod/app-node/.env.app-node.example \
    -f deployments/prod/app-node/docker-compose.app-node.yml --profile migration config >/dev/null

  echo "  data-node"
  "${dc[@]}" --env-file deployments/prod/data-node/.env.data-node.example \
    -f deployments/prod/data-node/docker-compose.data-node.yml config >/dev/null

  if [[ -f deployments/prod/.env.production.example ]] && [[ -f deployments/prod/docker-compose.prod.yml ]]; then
    echo "  legacy single-host prod (rollback path example)"
    (cd deployments/prod && "${dc[@]}" --env-file .env.production.example -f docker-compose.prod.yml config >/dev/null)
  fi

  echo "OK: compose config"
}

phase_go_test() {
  section "Phase 5: Go tests (go test ./...)"
  if [[ "${VERIFY_ENTERPRISE_SKIP_GO:-}" == "1" ]]; then
    echo "SKIP: VERIFY_ENTERPRISE_SKIP_GO=1"
    return 0
  fi
  go test ./...
  echo "OK: Go tests"
}

main() {
  echo "Enterprise release verification (repo root: $ROOT)"
  phase_shell_syntax
  phase_example_secret_scan
  phase_yaml_optional
  phase_docker_compose
  phase_go_test
  echo ""
  echo "=== All phases passed ==="
}

main "$@"
