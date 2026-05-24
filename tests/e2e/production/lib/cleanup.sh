#!/usr/bin/env bash
# shellcheck shell=bash
# Production E2E cleanup attestation — only documents E2E-PROD-${RUN_ID} resources.

prod_e2e_cleanup_attestation_path() {
  printf '%s' "${PROD_E2E_RUN_DIR}/cleanup-attestation.json"
}

prod_e2e_run_cleanup_attestation() {
  [[ "${MODE:-}" == "live" ]] || return 0
  [[ -n "${PROD_E2E_RUN_DIR:-}" ]] || return 0

  local attestation
  attestation="$(prod_e2e_cleanup_attestation_path)"
  local prefix="${PROD_E2E_PREFIX:-E2E-PROD-${PROD_E2E_RUN_ID}}"
  local state_json="${PROD_E2E_STATE_JSON:-${PROD_E2E_RUN_DIR}/state.json}"
  local attempted="true"
  local status="pass"
  local note="attestation_only — no automated DELETE/archive APIs wired for production E2E yet"

  prod_e2e_py -c "
import json, os, re
from pathlib import Path
from datetime import datetime, timezone

run_dir = Path(os.environ['PROD_E2E_RUN_DIR'])
prefix = os.environ.get('PROD_E2E_PREFIX', '')
state_path = Path(os.environ.get('PROD_E2E_STATE_JSON', str(run_dir / 'state.json')))
attestation = run_dir / 'cleanup-attestation.json'

state = {}
if state_path.is_file():
    try:
        state = json.loads(state_path.read_text(encoding='utf-8'))
    except json.JSONDecodeError:
        state = {}

resource_keys = [
    'siteId', 'machineId', 'productId', 'categoryId', 'brandId', 'tagId', 'mediaId',
    'planogramId', 'activationCodeId', 'operatorSessionId',
]
resources = {}
for k in resource_keys:
    v = state.get(k, '')
    if v and v != '<redacted>':
        resources[k] = v

non_e2e = []
for k, v in state.items():
    if re.search(r'token|password|secret', k, re.I):
        continue
    if isinstance(v, str) and v and not v.startswith('E2E-PROD-') and k in ('sku',):
        if not str(v).startswith(prefix):
            non_e2e.append(k)

doc = {
    'runId': os.environ.get('PROD_E2E_RUN_ID', ''),
    'prefix': prefix,
    'attempted': True,
    'status': 'pass' if not non_e2e else 'fail',
    'mode': 'attestation_only',
    'finishedUtc': datetime.now(timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ'),
    'resourcesCreated': resources,
    'resourcesDeleted': [],
    'resourcesIntentionallyRetained': list(resources.keys()),
    'retainReason': 'production E2E harness has no safe automated DELETE endpoints; E2E-PROD resources may be archived manually',
    'nonE2eMutationCheck': 'pass' if not non_e2e else 'fail',
    'nonE2eMutationKeys': non_e2e,
    'note': os.environ.get('PROD_E2E_CLEANUP_NOTE', ''),
}
attestation.write_text(json.dumps(doc, indent=2) + '\n', encoding='utf-8')
print('CLEANUP_ATTESTATION', attestation)
" \
    PROD_E2E_RUN_DIR="${PROD_E2E_RUN_DIR}" \
    PROD_E2E_PREFIX="${prefix}" \
    PROD_E2E_STATE_JSON="${state_json}" \
    PROD_E2E_CLEANUP_NOTE="${note}" || status="fail"

  echo "CLEANUP_ATTESTATION status=${status} path=${attestation}"
}

prod_e2e_cleanup_trap() {
  prod_e2e_state_sync_json || true
  prod_e2e_run_cleanup_attestation || true
}
