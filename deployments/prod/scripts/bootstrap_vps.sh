#!/usr/bin/env bash
# One-time Ubuntu 24.04 host prep for the lean production stack (run as root).
# Does not disable SSH password authentication (do that manually if you enforce key-only auth).
set -euo pipefail

if [[ "$(id -u)" -ne 0 ]]; then
	echo "error: run as root (sudo -i)" >&2
	exit 1
fi

: "${AVF_DEPLOY_DIR:?Set AVF_DEPLOY_DIR to the absolute path of .../avf-vending-api/deployments/prod}"

if [[ ! -d "${AVF_DEPLOY_DIR}" ]]; then
	echo "error: AVF_DEPLOY_DIR is not a directory: ${AVF_DEPLOY_DIR}" >&2
	exit 1
fi

export DEBIAN_FRONTEND=noninteractive
apt-get update -y
apt-get install -y ca-certificates curl git gnupg jq make ufw

if ! command -v docker >/dev/null 2>&1; then
	echo "==> install Docker Engine + Compose plugin"
	curl -fsSL https://get.docker.com | sh
fi

systemctl enable --now docker

echo "==> optional swap (2G file) if none configured"
if ! swapon --show | grep -q .; then
	if [[ ! -f /swapfile ]]; then
		fallocate -l 2G /swapfile || dd if=/dev/zero of=/swapfile bs=1M count=2048
		chmod 600 /swapfile
		mkswap /swapfile
		swapon /swapfile
		grep -q '^/swapfile ' /etc/fstab || echo '/swapfile none swap sw 0 0' >>/etc/fstab
	fi
fi

echo "==> UFW baseline (adjust if you use non-standard SSH port)"
ufw allow OpenSSH
ufw allow 80/tcp
ufw allow 443/tcp
# MQTT plaintext (devices). Comment out if you only accept tunneled MQTT.
ufw allow 1883/tcp
ufw --force enable || true

timedatectl set-timezone UTC || true

mkdir -p "${AVF_DEPLOY_DIR}/backups" "${AVF_DEPLOY_DIR}/.deploy"

unit_src="${AVF_DEPLOY_DIR}/systemd/avf-vending-prod.service"
if [[ ! -f "${unit_src}" ]]; then
	echo "error: missing unit file ${unit_src}" >&2
	exit 1
fi

sed "s#AVF_DEPLOY_DIR_PLACEHOLDER#${AVF_DEPLOY_DIR}#g" "${unit_src}" >/etc/systemd/system/avf-vending-prod.service

systemctl daemon-reload
systemctl enable avf-vending-prod.service

if [[ -f "${AVF_DEPLOY_DIR}/.env.production" ]]; then
	echo "==> .env.production found; starting systemd service"
	systemctl restart avf-vending-prod.service
else
	echo "==> .env.production not present yet; service enabled but not started"
fi

echo "bootstrap_vps: done — place .env.production in ${AVF_DEPLOY_DIR}, then: systemctl start avf-vending-prod (or run scripts/deploy_prod.sh)"
