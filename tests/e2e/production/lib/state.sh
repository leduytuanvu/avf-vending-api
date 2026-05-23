#!/usr/bin/env bash
# shellcheck shell=bash
# Persist captured IDs to state.env and state.json (no secrets).

prod_e2e_state_sync_json() {
  [[ -f "${PROD_E2E_STATE_FILE}" ]] || return 0
  PROD_E2E_STATE_JSON="${PROD_E2E_RUN_DIR}/state.json"
  export PROD_E2E_STATE_JSON
  prod_e2e_py -c "
import json, os, re
from pathlib import Path
env_path = Path(os.environ['PROD_E2E_STATE_FILE'])
out = Path(os.environ.get('PROD_E2E_STATE_JSON', env_path.parent / 'state.json'))
data = {}
if env_path.is_file():
    for line in env_path.read_text(encoding='utf-8').splitlines():
        line = line.strip()
        if not line or line.startswith('#'):
            continue
        m = re.match(r'^([A-Za-z_][A-Za-z0-9_]*)=(.*)$', line)
        if not m:
            continue
        k, v = m.group(1), m.group(2)
        if v.startswith(\"'\") and v.endswith(\"'\"):
            v = v[1:-1]
        if re.search(r'token|password|secret', k, re.I):
            v = '<redacted>' if v else ''
        data[k] = v
out.write_text(json.dumps(data, indent=2, sort_keys=True) + '\n', encoding='utf-8')
"
}

prod_e2e_state_reload_key() {
  local key="$1"
  local line
  line="$(grep "^${key}=" "${PROD_E2E_STATE_FILE}" 2>/dev/null | tail -1 || true)"
  [[ -n "$line" ]] || return 0
  # Values are written with %q; eval restores them safely (incl. long JWTs on Git Bash).
  eval "export ${line}"
}

prod_e2e_state_set() {
  local key="$1"
  local val="$2"
  touch "${PROD_E2E_STATE_FILE}"
  grep -v "^${key}=" "${PROD_E2E_STATE_FILE}" 2>/dev/null >"${PROD_E2E_STATE_FILE}.tmp" || true
  printf '%s=%q\n' "$key" "$val" >>"${PROD_E2E_STATE_FILE}.tmp"
  mv "${PROD_E2E_STATE_FILE}.tmp" "${PROD_E2E_STATE_FILE}"
  if [[ ${#val} -le 512 ]]; then
    # shellcheck disable=SC2086
    export "${key}=${val}"
  else
    prod_e2e_state_reload_key "$key"
  fi
  if [[ "$key" == "accessToken" ]]; then
    grep -v "^ADMIN_TOKEN=" "${PROD_E2E_STATE_FILE}" 2>/dev/null >"${PROD_E2E_STATE_FILE}.tmp" || true
    printf 'ADMIN_TOKEN=%q\n' "$val" >>"${PROD_E2E_STATE_FILE}.tmp"
    mv "${PROD_E2E_STATE_FILE}.tmp" "${PROD_E2E_STATE_FILE}"
    prod_e2e_state_reload_key ADMIN_TOKEN
  fi
  if [[ "$key" == "machineToken" ]]; then
    export MACHINE_TOKEN="$val"
  fi
  if [[ "$key" == "machineRefreshToken" ]]; then
    export MACHINE_REFRESH_TOKEN="$val"
  fi
  prod_e2e_state_sync_json
}
