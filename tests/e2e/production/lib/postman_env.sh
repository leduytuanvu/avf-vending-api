#!/usr/bin/env bash
# shellcheck shell=bash
# Patch Postman environment from live run state for Newman.

prod_e2e_export_postman_runtime_state() {
  local dst="${PROD_E2E_RUN_DIR}/postman/runtime-state.json"
  mkdir -p "${PROD_E2E_RUN_DIR}/postman"
  prod_e2e_py -c "
import json, os, re
from pathlib import Path
state_path = Path(os.environ.get('PROD_E2E_STATE_JSON', ''))
prefix = os.environ.get('PROD_E2E_PREFIX', '')
state = {}
if state_path.is_file():
    state = json.loads(state_path.read_text(encoding='utf-8'))
keys = [
    'runId', 'baseUrl', 'siteId', 'machineId', 'productId', 'categoryId', 'brandId',
    'tagId', 'mediaId', 'planogramId', 'planogramRevision', 'operatorSessionId',
    'activationCodeId', 'sku',
]
doc = {'runId': os.environ.get('PROD_E2E_RUN_ID', ''), 'runPrefix': prefix, 'baseUrl': os.environ.get('BASE_URL', '')}
for k in keys:
    v = state.get(k, '')
    if v and v != '<redacted>':
        doc[k] = v
Path(os.environ['dst']).write_text(json.dumps(doc, indent=2) + '\n', encoding='utf-8')
print('POSTMAN_RUNTIME_STATE', os.environ['dst'])
" dst="$dst" PROD_E2E_STATE_JSON="${PROD_E2E_STATE_JSON:-${PROD_E2E_RUN_DIR}/state.json}"
}

prod_e2e_sync_postman_env() {
  local src="${PROD_E2E_REPO_ROOT}/postman/production/avf-production-e2e.postman_environment.json"
  local dst="${PROD_E2E_RUN_DIR}/postman/runtime.postman_environment.json"
  [[ -f "$src" ]] || { echo "missing Postman env: $src" >&2; return 1; }
  mkdir -p "${PROD_E2E_RUN_DIR}/postman"
  prod_e2e_state_sync_json || true
  prod_e2e_export_postman_runtime_state || true
  export src dst
  prod_e2e_py -c "
import json, os
from pathlib import Path
src = Path(os.environ['src'])
dst = Path(os.environ['dst'])
state_path = Path(os.environ.get('PROD_E2E_STATE_JSON', ''))
state = {}
if state_path.is_file():
    state = json.loads(state_path.read_text(encoding='utf-8'))
env = json.loads(src.read_text(encoding='utf-8'))
def pick(*keys):
    for k in keys:
        v = state.get(k) or os.environ.get(k, '')
        if v and v != '<redacted>':
            return str(v)
    return ''
mapping = {
    'baseUrl': os.environ.get('BASE_URL', ''),
    'adminEmail': os.environ.get('ADMIN_EMAIL', ''),
    'adminPassword': os.environ.get('ADMIN_PASSWORD', ''),
    'accessToken': pick('accessToken'),
    'machineAccessToken': pick('machineToken', 'machineAccessToken'),
    'machineRefreshToken': pick('machineRefreshToken'),
    'machineId': pick('machineId'),
    'runId': os.environ.get('PROD_E2E_RUN_ID', ''),
    'runPrefix': os.environ.get('PROD_E2E_PREFIX', ''),
    'categoryId': pick('categoryId'),
    'brandId': pick('brandId'),
    'tagId': pick('tagId'),
    'mediaId': pick('mediaId'),
    'productId': pick('productId'),
    'siteId': pick('siteId'),
    'activationCode': pick('activationCode'),
    'orderId': pick('orderId'),
    'paymentId': pick('paymentId'),
    'commandId': pick('commandId'),
    'webhookSecret': os.environ.get('COMMERCE_PAYMENT_WEBHOOK_SECRET', ''),
    'webhookEventId': pick('webhook_event_id', 'webhookEventId') or (os.environ.get('PROD_E2E_PREFIX', '') + '-wh'),
    'mediaSha256': os.environ.get('PROD_E2E_MEDIA_SHA256', ''),
    'operatorSessionId': pick('operatorSessionId'),
    'planogramId': pick('planogramId'),
    'planogramRevision': pick('planogramRevision'),
    'adminEmailInvalidTest': os.environ.get('ADMIN_EMAIL_INVALID_TEST', 'e2e-invalid@invalid.local'),
    'allowGatedWrites': 'true',
    'confirmProductionWrites': os.environ.get('E2E_PRODUCTION_WRITE_CONFIRMATION', ''),
    'newmanReuseShellState': 'true' if pick('siteId') else 'false',
}
for item in env.get('values', []):
    k = item.get('key')
    if k in mapping and mapping[k]:
        item['value'] = mapping[k]
dst.write_text(json.dumps(env, indent=2) + '\n', encoding='utf-8')
print('POSTMAN_ENV_SYNC', dst)
" src="$src" dst="$dst"
}
