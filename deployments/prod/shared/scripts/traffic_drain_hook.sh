#!/usr/bin/env bash
# Optional pre-restart traffic drain. Invoked on the app node before caddy is stopped in release_app_node.sh.
# TRAFFIC_DRAIN_MODE: none | caddy | external-lb
# TRAFFIC_DRAIN_WAIT_SECONDS: sleep after hook (0 = none)
# TRAFFIC_DRAIN_EXTERNAL_SCRIPT: for external-lb, path to operator script on the VPS
#
# This hook does not fake success. external-lb without a script fails fast.
# none: no LB drain — records that global zero-downtime is not claimed (see docs).
# caddy: the actual in-compose drain is the subsequent `docker compose stop caddy` in release_app_node.sh.
set -Eeuo pipefail

SHARED_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=./lib_release.sh
source "${SHARED_ROOT}/scripts/lib_release.sh"

MODE="${TRAFFIC_DRAIN_MODE:-none}"
WAIT="${TRAFFIC_DRAIN_WAIT_SECONDS:-0}"

case "${MODE}" in
none)
	echo "traffic_drain_hook: TRAFFIC_DRAIN_MODE=none — no external/LB drain before app restart; in-node caddy stop (next step) is the only local traffic pause. zero_downtime_claim: false at global/LB scope." >&2
	;;
caddy)
	echo "traffic_drain_hook: TRAFFIC_DRAIN_MODE=caddy — drain is implemented by stopping the node caddy service next (release_app_node.sh); this is node-local, not a cloud LB weight change." >&2
	;;
external-lb)
	if [[ -z "${TRAFFIC_DRAIN_EXTERNAL_SCRIPT:-}" ]]; then
		echo "traffic_drain_hook: error: TRAFFIC_DRAIN_MODE=external-lb requires TRAFFIC_DRAIN_EXTERNAL_SCRIPT set to an existing operator hook on this host" >&2
		exit 1
	fi
	run_script "${TRAFFIC_DRAIN_EXTERNAL_SCRIPT}"
	;;
*)
	echo "traffic_drain_hook: error: invalid TRAFFIC_DRAIN_MODE='${MODE}' (use none, caddy, or external-lb)" >&2
	exit 1
	;;
esac

if [[ "${WAIT}" =~ ^[0-9]+$ && "${WAIT}" -gt 0 ]]; then
	echo "traffic_drain_hook: waiting ${WAIT}s (TRAFFIC_DRAIN_WAIT_SECONDS)" >&2
	sleep "${WAIT}"
fi
