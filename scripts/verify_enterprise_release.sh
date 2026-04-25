#!/usr/bin/env bash
# Enterprise release verification: tests, Swagger regen + drift check, shell syntax,
# Docker Compose config (example env), OpenAPI release checks (incl. P0 path matrix),
# stale P0 doc contradiction scan, template + docs secret heuristics, optional YAML parse.
#
# Does not connect to production or require real secrets.
#
# Phase order matches docs/runbooks/production-release-readiness.md (includes P0 OpenAPI + doc contradiction gates).
#
# Environment (optional):
#   VERIFY_ENTERPRISE_SKIP_DOCKER=1   Skip docker compose config phases.
#   VERIFY_ENTERPRISE_SKIP_YAML=1     Skip YAML parse phase.
#   VERIFY_ENTERPRISE_SKIP_GO=1       Skip go test (debug only).
#   MAKE=make                         Make program (default: make).

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

PY="${PY:-python3}"

section() {
  echo ""
  echo "=== $* ==="
}

die() {
  echo "ERROR: $*" >&2
  exit 1
}

phase_go_test() {
  section "Phase 1: Go tests (go test ./...)"
  if [[ "${VERIFY_ENTERPRISE_SKIP_GO:-}" == "1" ]]; then
    echo "SKIP: VERIFY_ENTERPRISE_SKIP_GO=1"
    return 0
  fi
  go test ./...
  echo "OK: Go tests"
}

phase_swagger() {
  section "Phase 2: Regenerate OpenAPI (make swagger)"
  if command -v make >/dev/null 2>&1; then
    make swagger
  else
    echo "WARN: make not on PATH; running ${PY} tools/build_openapi.py"
    "${PY}" tools/build_openapi.py
  fi
  echo "OK: Swagger generation"
}

phase_swagger_check() {
  section "Phase 3: Swagger drift check (make swagger-check)"
  if command -v make >/dev/null 2>&1; then
    make swagger-check
  else
    "${PY}" tools/build_openapi.py
    git diff --exit-code -- docs/swagger/
  fi
  echo "OK: swagger-check (committed docs/swagger matches generator)"
}

phase_postman_check() {
  section "Phase 3b: Postman artifacts (same as make postman-check; offline, no network)"
  if ! command -v "${PY}" >/dev/null 2>&1; then
    die "${PY} not on PATH (required for postman checks)"
  fi
  "${PY}" tools/check_postman_artifacts.py
  echo "OK: postman-check"
}

phase_shell_syntax() {
  section "Phase 4: Bash syntax (bash -n) — scripts/**/*.sh and deployments/**/*.sh"
  local n files=()
  mapfile -t files < <(find scripts deployments -type f -name '*.sh' 2>/dev/null | LC_ALL=C sort)
  n="${#files[@]}"
  echo "Found ${n} shell scripts."
  [[ "$n" -gt 0 ]] || die "no .sh files found under scripts/ or deployments/"
  local f
  for f in "${files[@]}"; do
    echo "  bash -n: ${f}"
    bash -n "$f"
  done
  echo "OK: shell syntax"
}

phase_docker_compose() {
  section "Phase 5: Docker Compose config (example env — no cluster contact)"
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

phase_openapi_release() {
  section "Phase 6: OpenAPI release checks (production first, local second, required P0 paths, Bearer on protected /v1, write examples, success+error examples, no planned-only paths, no secret-like examples)"
  if ! command -v "${PY}" >/dev/null 2>&1; then
    die "${PY} not on PATH (required for OpenAPI verification)"
  fi
  "${PY}" tools/openapi_verify_release.py
  echo "OK: OpenAPI release verification"
}

phase_stale_p0_docs() {
  section "Phase 6b: Docs — no stale 'not applied' / unmounted P0 claims"
  bash scripts/check_stale_p0_docs.sh
  echo "OK: P0 doc contradiction check"
}

line_looks_placeholder_ok() {
  if [[ "$1" =~ (CHANGE_ME|PLACEHOLDER|REDACTED|YOUR_|INSERT_|REPLACE_ME|example\.com|myorg|ldtv\.dev|127\.0\.0\.1|localhost|stub-|stub_|example-password|documentation\ UUIDs|user-password) ]]; then
    return 0
  fi
  if [[ "$1" == *"<jwt>"* ]] || [[ "$1" == *"<opaque>"* ]]; then
    return 0
  fi
  return 1
}

phase_deployment_example_secret_scan() {
  section "Phase 7a: Deployment examples — block obvious live secrets"
  local f hits=0 ex_files=()
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
      if echo "$line" | grep -Eq 'eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+'; then
        echo "  BLOCKED (JWT-shaped token): $f"
        echo "    $line"
        hits=$((hits + 1))
        continue
      fi
    done <"$f"
  done

  if [[ "$hits" -gt 0 ]]; then
    die "deployment example secret scan failed (${hits} finding(s)); use CHANGE_ME_* placeholders only"
  fi
  echo "OK: deployment example files passed live-secret heuristics"

  section "Phase 7b: Deployment examples — no empty required assignments"
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

  echo "OK: deployment example env posture"
}

phase_docs_testdata_secret_scan() {
  section "Phase 7c: Docs and testdata — block obvious live secrets (optional heuristic)"
  local hits=0 f
  local files=()
  mapfile -t files < <(
    find docs testdata -type f \( -name '*.md' -o -name '*.json' \) \
      ! -path 'docs/swagger/swagger.json' 2>/dev/null | LC_ALL=C sort
  )
  for f in "${files[@]}"; do
    [[ -f "$f" ]] || continue
    while IFS= read -r line; do
      [[ "$line" =~ ^[[:space:]]*# ]] && continue
      [[ "$line" =~ ^[[:space:]]*\| ]] && continue
      [[ -z "${line// }" ]] && continue
      if line_looks_placeholder_ok "$line"; then
        continue
      fi
      if echo "$line" | grep -Eq 'sk_live_[a-zA-Z0-9]+|pk_live_[a-zA-Z0-9]+'; then
        echo "  BLOCKED (live payment key pattern): $f"
        hits=$((hits + 1))
        continue
      fi
      if echo "$line" | grep -Eq 'AKIA[0-9A-Z]{16}|ASIA[0-9A-Z]{16}'; then
        echo "  BLOCKED (AWS key id pattern): $f"
        hits=$((hits + 1))
        continue
      fi
      if echo "$line" | grep -Eq 'ghp_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]+'; then
        echo "  BLOCKED (GitHub token pattern): $f"
        hits=$((hits + 1))
        continue
      fi
      if echo "$line" | grep -Fq 'PRIVATE KEY-----'; then
        echo "  BLOCKED (PEM private key material): $f"
        hits=$((hits + 1))
        continue
      fi
      if echo "$line" | grep -Eq 'eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+'; then
        echo "  BLOCKED (JWT-shaped token in docs/testdata): $f"
        echo "    $line"
        hits=$((hits + 1))
        continue
      fi
    done <"$f"
  done
  if [[ "$hits" -gt 0 ]]; then
    die "docs/testdata secret heuristic failed (${hits} finding(s)); use placeholders only"
  fi
  echo "OK: docs/testdata heuristics"
}

phase_yaml_optional() {
  section "Phase 8: YAML structure (optional — PyYAML)"
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

main() {
  echo "Enterprise release verification (repo root: $ROOT)"
  phase_go_test
  phase_swagger
  phase_swagger_check
  phase_postman_check
  phase_shell_syntax
  phase_docker_compose
  phase_openapi_release
  phase_stale_p0_docs
  phase_deployment_example_secret_scan
  phase_docs_testdata_secret_scan
  phase_yaml_optional
  echo ""
  echo "=== All phases passed ==="
  echo "Pilot deploy may proceed per docs/runbooks/production-release-readiness.md (storm evidence still required for scale tiers)."
}

main "$@"
