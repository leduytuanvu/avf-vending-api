#!/usr/bin/env bash
# REQUIRES EXPLICIT OPERATOR APPROVAL — NOT EXECUTED BY CI
#
# Terminate verified orphaned db-audit python3 shim recursion processes.
# Default mode is --dry-run (print decisions only).
#
# Usage:
#   bash scripts/ops/terminate-verified-db-audit-orphans.sh
#   bash scripts/ops/terminate-verified-db-audit-orphans.sh --execute
#   bash scripts/ops/terminate-verified-db-audit-orphans.sh --execute --min-age-seconds 300
set -Eeuo pipefail

DRY_RUN=1
MIN_AGE_SECONDS=300
TERM_WAIT_SECONDS=5

usage() {
	cat <<'EOF'
usage: terminate-verified-db-audit-orphans.sh [--dry-run|--execute] [--min-age-seconds SECS]

  --dry-run            Print SKIP/KILL decisions only (default)
  --execute            Send SIGTERM/SIGKILL to verified orphan PIDs
  --min-age-seconds    Minimum process age before termination (default: 300)
EOF
}

log() {
	echo "terminate-verified-db-audit-orphans: $*"
}

while [[ $# -gt 0 ]]; do
	case "$1" in
	--dry-run)
		DRY_RUN=1
		shift
		;;
	--execute)
		DRY_RUN=0
		shift
		;;
	--min-age-seconds)
		[[ $# -ge 2 ]] || {
			echo "missing value for --min-age-seconds" >&2
			exit 2
		}
		MIN_AGE_SECONDS="$2"
		shift 2
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		echo "unknown argument: $1" >&2
		usage >&2
		exit 2
		;;
	esac
done

proc_age_seconds() {
	local pid="$1"
	local etimes uptime start_ticks clk_tck age

	etimes="$(ps -p "${pid}" -o etimes= 2>/dev/null | tr -d ' ')"
	if [[ -n "${etimes}" && "${etimes}" =~ ^[0-9]+$ ]]; then
		echo "${etimes}"
		return 0
	fi

	uptime="$(awk '{print int($1)}' /proc/uptime 2>/dev/null || true)"
	start_ticks="$(awk '{print $22}' "/proc/${pid}/stat" 2>/dev/null || true)"
	clk_tck="$(getconf CLK_TCK 2>/dev/null || echo 100)"
	if [[ -z "${uptime}" || -z "${start_ticks}" ]]; then
		echo 0
		return 0
	fi
	age=$((uptime - start_ticks / clk_tck))
	if [[ "${age}" -lt 0 ]]; then
		age=0
	fi
	echo "${age}"
}

proc_cmdline() {
	local pid="$1"
	local cmdline

	cmdline="$(tr '\0' ' ' <"/proc/${pid}/cmdline" 2>/dev/null || true)"
	if [[ -n "${cmdline//[[:space:]]/}" ]]; then
		printf '%s' "${cmdline}"
		return 0
	fi
	ps -p "${pid}" -o args= 2>/dev/null || true
}

in_docker_cgroup() {
	local pid="$1"
	if grep -Eq '(docker|containerd|kubepods)' "/proc/${pid}/cgroup" 2>/dev/null; then
		return 0
	fi
	return 1
}

is_audit_shim_orphan() {
	local cmdline="$2"

	[[ "${cmdline}" == *".db-destroy-evidence/.bin/python3"* ]]
}

kill_verified_pid() {
	local pid="$1"
	local signal="$2"
	local label="$3"
	if [[ "${DRY_RUN}" -eq 1 ]]; then
		log "DRY-RUN ${label} pid=${pid} signal=${signal}"
		return 0
	fi
	log "${label} pid=${pid} signal=${signal}"
	kill "-${signal}" "${pid}" 2>/dev/null || true
}

candidate_pids=()
append_candidate_pid() {
	local pid="$1" existing
	[[ "${pid}" =~ ^[0-9]+$ ]] || return 0
	for existing in "${candidate_pids[@]}"; do
		[[ "${existing}" == "${pid}" ]] && return 0
	done
	candidate_pids+=("${pid}")
}

while IFS= read -r line; do
	[[ -n "${line}" ]] || continue
	append_candidate_pid "${line%% *}"
done < <(pgrep -af 'db-destroy-evidence|verify_database_environment\.sh' 2>/dev/null || true)

while IFS= read -r pid; do
	[[ -n "${pid}" ]] || continue
	append_candidate_pid "${pid}"
done < <(pgrep -f '\.db-destroy-evidence/.bin/python3' 2>/dev/null || true)

if [[ "${#candidate_pids[@]}" -eq 0 ]]; then
	log "no candidate processes found"
	exit 0
fi

verified=()
for pid in "${candidate_pids[@]}"; do
	cmdline="$(proc_cmdline "${pid}")"
	if [[ -z "${cmdline}" ]]; then
		log "SKIP pid=${pid} reason=missing_cmdline"
		continue
	fi
	if in_docker_cgroup "${pid}"; then
		log "SKIP pid=${pid} reason=docker_cgroup cmd=${cmdline}"
		continue
	fi
	if ! is_audit_shim_orphan "${pid}" "${cmdline}"; then
		log "SKIP pid=${pid} reason=not_verified_orphan cmd=${cmdline}"
		continue
	fi
	age="$(proc_age_seconds "${pid}")"
	if [[ "${age}" -lt "${MIN_AGE_SECONDS}" ]]; then
		log "SKIP pid=${pid} reason=too_young age=${age}s cmd=${cmdline}"
		continue
	fi
	log "VERIFY pid=${pid} age=${age}s cmd=${cmdline}"
	verified+=("${pid}")
done

if [[ "${#verified[@]}" -eq 0 ]]; then
	log "no verified orphan processes to terminate"
	exit 0
fi

for pid in "${verified[@]}"; do
	kill_verified_pid "${pid}" TERM "KILL-CANDIDATE"
done

if [[ "${DRY_RUN}" -eq 1 ]]; then
	log "dry-run complete (${#verified[@]} verified candidate(s))"
	exit 0
fi

sleep "${TERM_WAIT_SECONDS}"

for pid in "${verified[@]}"; do
	cmdline="$(proc_cmdline "${pid}")"
	if [[ -z "${cmdline}" ]]; then
		log "RESOLVED pid=${pid} after SIGTERM"
		continue
	fi
	if is_audit_shim_orphan "${pid}" "${cmdline}"; then
		kill_verified_pid "${pid}" KILL "SIGKILL-REMAINING"
	else
		log "RESOLVED pid=${pid} cmd changed after SIGTERM"
	fi
done

cat <<'EOF'

Post-mitigation verification (run manually):
  uptime; cat /proc/loadavg
  ps -eo pid,ppid,user,%cpu,%mem,etime,cmd --sort=-%cpu | head -30
  docker stats --no-stream
  pgrep -af 'db-destroy-evidence|verify_database_environment' || echo none

If CPU remains high after orphans = 0, STOP and re-open RCA.
EOF
