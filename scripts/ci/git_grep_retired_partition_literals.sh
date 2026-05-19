#!/usr/bin/env bash
# Mechanical gate: retired partition-style literals must not appear in tracked sources.
# Pattern pieces are assembled at runtime so this script stays free of contiguous forbidden tokens.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

US='_'
HY='-'
scope='scope'
org='organization'
tenant='tenant'
id='id'
type='type'
channel='channel'
broker='broker'
admin='admin'

Scope='Scope'
Org='Organization'
Tenant='Tenant'
ID='ID'

pat_scope_under_id="${scope}${US}${id}"
pat_scope_camel_id="${scope}Id"
pat_scope_title_id="${Scope}${ID}"
pat_scope_under_type="${scope}${US}${type}"
pat_scope_title_type="${Scope}Type"
pat_channel_under_scope="${channel}${US}${scope}"
pat_broker_under_scope="${broker}${US}${scope}"
pat_org_under_id="${org}${US}${id}"
pat_org_camel_id="${org}Id"
pat_org_title_id="${Org}${ID}"
pat_tenant_under_id="${tenant}${US}${id}"
pat_tenant_camel_id="${tenant}Id"
pat_tenant_title_id="${Tenant}${ID}"
pat_org_short_under_admin="org${US}${admin}"
pat_tenant_hyphen_scoped="${tenant}${HY}scoped"
pat_org_hyphen_scoped="org${HY}scoped"
pat_canary_org_under_id="canary${US}${org}${US}${id}"

EXT_PATTERN="${pat_scope_under_id}|${pat_scope_camel_id}|${pat_scope_title_id}|${pat_scope_under_type}|${pat_scope_title_type}|${pat_channel_under_scope}|${pat_broker_under_scope}|${pat_org_under_id}|${pat_org_camel_id}|${pat_org_title_id}|${pat_tenant_under_id}|${pat_tenant_camel_id}|${pat_tenant_title_id}|${pat_org_short_under_admin}|${pat_tenant_hyphen_scoped}|${pat_org_hyphen_scoped}|${pat_canary_org_under_id}|E2E_ORGANIZATION_ID|DevOrganizationID"

echo "### Extended alternation gate (expect no stdout; exit 1 = clean)"
set +e
git grep -nE "${EXT_PATTERN}" -- . ':!vendor/**' ':!node_modules/**' ':!.git/**'
ec_ext=$?
set -e

echo "### Primary contiguous gate (partition column token = scope + US + id; expect no stdout; exit 1 = clean)"
set +e
git grep -n "${pat_scope_under_id}" -- . ':!vendor/**' ':!node_modules/**' ':!.git/**'
ec_pri=$?
set -e

if [[ "$ec_ext" -eq 0 ]] || [[ "$ec_pri" -eq 0 ]]; then
  echo "FAIL: retired partition literals matched (see lines above)." >&2
  exit 2
fi

if [[ "$ec_ext" -ne 1 ]] || [[ "$ec_pri" -ne 1 ]]; then
  echo "FAIL: unexpected git grep exit (ext=${ec_ext} pri=${ec_pri})." >&2
  exit 3
fi

echo "PASS: no matches."
