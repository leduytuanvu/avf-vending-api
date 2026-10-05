#!/usr/bin/env bash
# Safe production disk hygiene for app-node hosts (Docker, logs, migrate backups, runner temp).
set -Eeuo pipefail

KEEP_MIGRATION_BACKUPS="${KEEP_MIGRATION_BACKUPS:-5}"
JOURNAL_VACUUM_TIME="${JOURNAL_VACUUM_TIME:-7d}"
TMP_AVF_MAX_AGE_DAYS="${TMP_AVF_MAX_AGE_DAYS:-3}"
RUNNER_WORK_MAX_AGE_DAYS="${RUNNER_WORK_MAX_AGE_DAYS:-7}"
MIGRATION_LOG_DIR="${MIGRATION_LOG_DIR:-/opt/avf-vending-api/deployments/prod/logs/migrations}"

note() { printf '[disk-hygiene] %s\n' "$*"; }

section() {
	echo ""
	echo "========== $* =========="
}

report_disk() {
	section "df"
	df -hT / /var /opt 2>/dev/null || df -h /
	section "docker system df"
	docker system df 2>/dev/null || true
	section "top disk (depth 1)"
	for p in /var/lib/docker /opt /var/log /tmp /home; do
		[[ -d "${p}" ]] || continue
		du -xh --max-depth=1 "${p}" 2>/dev/null | sort -hr | head -8 || true
	done
}

trim_migration_backups() {
	[[ -d "${MIGRATION_LOG_DIR}" ]] || {
		note "no migration log dir at ${MIGRATION_LOG_DIR}"
		return 0
	}
	mapfile -t dumps < <(find "${MIGRATION_LOG_DIR}" -maxdepth 1 -type f -name 'backup-*.dump' -printf '%T@ %p\n' 2>/dev/null | sort -rn | cut -d' ' -f2-)
	local count="${#dumps[@]}"
	if [[ "${count}" -le "${KEEP_MIGRATION_BACKUPS}" ]]; then
		note "migration backups: ${count} (keep ${KEEP_MIGRATION_BACKUPS}) — nothing to delete"
		return 0
	fi
	local i
	for ((i = KEEP_MIGRATION_BACKUPS; i < count; i++)); do
		note "remove old migration backup: ${dumps[$i]}"
		rm -f "${dumps[$i]}" || true
	done
	# Trim old migrate logs (keep same count as backups)
	mapfile -t logs < <(find "${MIGRATION_LOG_DIR}" -maxdepth 1 -type f -name 'migrate-*.log' -printf '%T@ %p\n' 2>/dev/null | sort -rn | cut -d' ' -f2-)
	for ((i = KEEP_MIGRATION_BACKUPS; i < ${#logs[@]}; i++)); do
		note "remove old migrate log: ${logs[$i]}"
		rm -f "${logs[$i]}" || true
	done
}

clean_runner_workdirs() {
	local base age_sec
	age_sec=$((RUNNER_WORK_MAX_AGE_DAYS * 86400))
	for base in /home/*/actions-runner/_work /opt/actions-runner/_work /var/actions-runner/_work; do
		[[ -d "${base}" ]] || continue
		note "prune stale runner work under ${base} (older than ${RUNNER_WORK_MAX_AGE_DAYS}d)"
		find "${base}" -mindepth 1 -maxdepth 1 -type d -mtime +"${RUNNER_WORK_MAX_AGE_DAYS}" -print -exec rm -rf {} + 2>/dev/null || true
	done
}

clean_tmp_avf() {
	note "remove /tmp/avf-* older than ${TMP_AVF_MAX_AGE_DAYS}d"
	find /tmp -maxdepth 1 -type d -name 'avf-*' -mtime +"${TMP_AVF_MAX_AGE_DAYS}" -print -exec rm -rf {} + 2>/dev/null || true
	find /tmp -maxdepth 1 -type d -name 'avf-vending-*' -mtime +"${TMP_AVF_MAX_AGE_DAYS}" -print -exec rm -rf {} + 2>/dev/null || true
}

docker_prune_safe() {
	note "docker container prune (stopped)"
	docker container prune -f 2>/dev/null || true
	note "docker image prune (dangling)"
	docker image prune -f 2>/dev/null || true
	note "docker system prune (unused images, networks, build cache — not volumes)"
	docker system prune -af 2>/dev/null || true
	if docker buildx version >/dev/null 2>&1; then
		note "docker buildx prune"
		docker buildx prune -af 2>/dev/null || true
	fi
	note "docker builder prune"
	docker builder prune -af 2>/dev/null || true
}

journal_vacuum() {
	if command -v journalctl >/dev/null 2>&1; then
		note "journal vacuum (${JOURNAL_VACUUM_TIME})"
		journalctl --vacuum-time="${JOURNAL_VACUUM_TIME}" 2>/dev/null || true
	fi
}

apt_clean() {
	if command -v apt-get >/dev/null 2>&1; then
		note "apt-get clean"
		apt-get clean -y 2>/dev/null || true
	fi
}

main() {
	section "BEFORE"
	report_disk
	trim_migration_backups
	clean_tmp_avf
	clean_runner_workdirs
	journal_vacuum
	apt_clean
	docker_prune_safe
	section "AFTER"
	report_disk
	note "done"
}

main "$@"
