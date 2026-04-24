#!/usr/bin/env bash
# Validate production env flags that affect DB pool safety and topology.
# Usage: validate_production_deploy_inputs.sh [path/to/.env.app-node]
set -euo pipefail

norm_bool() {
	case "${1:-}" in
	1 | true | TRUE | yes | YES | on | ON) return 0 ;;
	*) return 1 ;;
	esac
}

ENV_FILE="${1:-}"
if [[ -n "${ENV_FILE}" ]]; then
	if [[ ! -f "${ENV_FILE}" ]]; then
		echo "error: env file not found: ${ENV_FILE}" >&2
		exit 1
	fi
	set -a
	# shellcheck disable=SC1090
	source "${ENV_FILE}"
	set +a
fi

if norm_bool "${ALLOW_APP_NODE_ON_DATA_NODE:-}"; then
	echo "WARNING: ALLOW_APP_NODE_ON_DATA_NODE=true — app workloads may share a host with the data-plane stack." >&2
	echo "WARNING: Estimated PostgreSQL clients = sum over all running processes of effective max pool size (per-role overrides apply), multiplied by app-node replicas." >&2
	echo "WARNING: Keep this total under roughly 60–70% of your managed pooler limit (e.g. Supabase session mode MaxClients) before adding nodes or colocating." >&2
fi

if norm_bool "${COLOCATE_APP_WITH_DATA_NODE:-}"; then
	if ! norm_bool "${ALLOW_APP_NODE_ON_DATA_NODE:-}"; then
		echo "error: COLOCATE_APP_WITH_DATA_NODE is true but ALLOW_APP_NODE_ON_DATA_NODE is not true — refusing silent colocated deploy." >&2
		exit 1
	fi
fi

if norm_bool "${ENABLE_APP_NODE_B:-}"; then
	echo "WARNING: ENABLE_APP_NODE_B=true — a second app node approximately doubles app-side DB pool usage versus one node." >&2
	echo "WARNING: Re-check total_connections = sum(effective_max_conns per process × number of app nodes running that stack)." >&2
fi

# Optional local mirror of CI fleet-scale storm gate (set VALIDATE_PRODUCTION_SCALE_STORM_GATE=true and storm env vars).
# Typical env: FLEET_SCALE_TARGET, TELEMETRY_STORM_EVIDENCE_FILE, STORM_EVIDENCE_MAX_AGE_DAYS (default 7 in the validator),
# ALLOW_SCALE_GATE_BYPASS, SCALE_GATE_BYPASS_REASON or BYPASS_REASON.
if norm_bool "${VALIDATE_PRODUCTION_SCALE_STORM_GATE:-}"; then
	_gate_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
	export ACTION_MODE="${ACTION_MODE:-deploy}"
	if ! "${_gate_dir}/validate_production_scale_storm_gate.sh"; then
		echo "error: production scale storm gate failed (see validate_production_scale_storm_evidence.py)" >&2
		exit 1
	fi
fi

echo "validate_production_deploy_inputs: ok"
