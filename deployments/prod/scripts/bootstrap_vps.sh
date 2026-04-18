#!/usr/bin/env bash
# One-time Ubuntu 24.04 host prep for the lean production stack.
# Run as root, but prefer using a non-root deploy user for normal day-2 operations afterward.
set -euo pipefail

log() {
	echo "==> $*"
}

warn() {
	echo "warning: $*" >&2
}

fail() {
	echo "error: $*" >&2
	exit 1
}

if [[ "$(id -u)" -ne 0 ]]; then
	fail "run as root (for example: sudo -i)"
fi

: "${AVF_DEPLOY_DIR:?Set AVF_DEPLOY_DIR to the absolute path of .../avf-vending-api/deployments/prod}"

if [[ "${AVF_DEPLOY_DIR}" != /* ]]; then
	fail "AVF_DEPLOY_DIR must be an absolute path, got: ${AVF_DEPLOY_DIR}"
fi

if [[ ! -d "${AVF_DEPLOY_DIR}" ]]; then
	fail "AVF_DEPLOY_DIR is not a directory: ${AVF_DEPLOY_DIR}"
fi

if [[ ! -f "${AVF_DEPLOY_DIR}/docker-compose.prod.yml" ]]; then
	fail "missing ${AVF_DEPLOY_DIR}/docker-compose.prod.yml; point AVF_DEPLOY_DIR at .../deployments/prod"
fi

DEPLOY_PARENT="$(dirname "${AVF_DEPLOY_DIR}")"
DEPLOY_USER="$(stat -c '%U' "${DEPLOY_PARENT}" 2>/dev/null || echo unknown)"
if [[ -z "${DEPLOY_USER}" || "${DEPLOY_USER}" == "UNKNOWN" || "${DEPLOY_USER}" == "unknown" ]]; then
	DEPLOY_USER="unknown"
fi

export DEBIAN_FRONTEND=noninteractive
log "install baseline host packages"
apt-get update -y
apt-get install -y ca-certificates curl git gnupg jq make ufw

if ! command -v docker >/dev/null 2>&1; then
	log "install Docker Engine + Compose plugin"
	curl -fsSL https://get.docker.com | sh
else
	log "Docker already present; skipping install"
fi

log "enable Docker service"
systemctl enable --now docker

log "optional swap (2 GiB file) if none configured"
if ! swapon --show | grep -q .; then
	if [[ ! -f /swapfile ]]; then
		fallocate -l 2G /swapfile || dd if=/dev/zero of=/swapfile bs=1M count=2048
		chmod 600 /swapfile
		mkswap /swapfile
		swapon /swapfile
		grep -q '^/swapfile ' /etc/fstab || echo '/swapfile none swap sw 0 0' >>/etc/fstab
		log "swapfile created and enabled"
	else
		log "swapfile already exists; enabling it"
		chmod 600 /swapfile
		mkswap /swapfile >/dev/null 2>&1 || true
		swapon /swapfile
		grep -q '^/swapfile ' /etc/fstab || echo '/swapfile none swap sw 0 0' >>/etc/fstab
	fi
else
	log "swap already configured; leaving it unchanged"
fi

log "apply UFW baseline (adjust separately if you use a non-standard SSH port)"
ufw allow OpenSSH
ufw allow 80/tcp
ufw allow 443/tcp
# MQTT TLS for the production broker path.
ufw allow 8883/tcp
if ! ufw --force enable; then
	warn "ufw enable returned non-zero; review firewall status manually"
fi

timedatectl set-timezone UTC || warn "could not set timezone to UTC"

log "ensure local deploy state directories exist"
mkdir -p "${AVF_DEPLOY_DIR}/backups" "${AVF_DEPLOY_DIR}/.deploy"

if [[ "${DEPLOY_USER}" != "root" && "${DEPLOY_USER}" != "unknown" ]]; then
	chown -R "${DEPLOY_USER}:${DEPLOY_USER}" "${AVF_DEPLOY_DIR}/backups" "${AVF_DEPLOY_DIR}/.deploy"
	log "made backups/ and .deploy writable by deploy user: ${DEPLOY_USER}"
else
	warn "could not infer a non-root deploy user from ${DEPLOY_PARENT}; root owns bootstrap-created state"
fi

unit_src="${AVF_DEPLOY_DIR}/systemd/avf-vending-prod.service"
if [[ ! -f "${unit_src}" ]]; then
	fail "missing unit file ${unit_src}"
fi

log "install systemd unit"
sed "s#AVF_DEPLOY_DIR_PLACEHOLDER#${AVF_DEPLOY_DIR}#g" "${unit_src}" >/etc/systemd/system/avf-vending-prod.service

systemctl daemon-reload
systemctl enable avf-vending-prod.service

if [[ -f "${AVF_DEPLOY_DIR}/.env.production" ]]; then
	log ".env.production found; starting avf-vending-prod.service"
	systemctl restart avf-vending-prod.service
else
	log ".env.production not present yet; service enabled but not started"
fi

if [[ "${DEPLOY_USER}" != "unknown" ]]; then
	log "day-2 operations should normally run as ${DEPLOY_USER}, not root"
fi
echo "bootstrap_vps: done — place .env.production in ${AVF_DEPLOY_DIR}, then run 'systemctl start avf-vending-prod' or 'bash ${AVF_DEPLOY_DIR}/scripts/deploy_prod.sh'"
