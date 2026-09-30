#!/usr/bin/env bash
# Sync Let's Encrypt certificate for mqtt.ldtv.dev into EMQX TLS material and restart broker.
# Requires certbot and an existing or issuable LE cert on the data node (HTTP-01 on :80).
set -Eeuo pipefail

NODE_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROD_ROOT="$(cd "${NODE_ROOT}/.." && pwd)"
SHARED_ROOT="$(cd "${NODE_ROOT}/../shared" && pwd)"
# shellcheck source=../../shared/scripts/lib_release.sh
source "${SHARED_ROOT}/scripts/lib_release.sh"

EMQX_DIR="${PROD_ROOT}/emqx"
CERT_DIR="${EMQX_DIR}/certs"
MQTT_DOMAIN="${MQTT_TLS_DOMAIN:-mqtt.ldtv.dev}"
LE_DIR="/etc/letsencrypt/live/${MQTT_DOMAIN}"
LEGACY_ENV="${PROD_ROOT}/.env.production"
LEGACY_COMPOSE="${PROD_ROOT}/docker-compose.prod.yml"
CADDY_STOPPED=0

read_env_value() {
	local key="$1"
	local file="$2"
	grep -E "^${key}=" "${file}" 2>/dev/null | tail -n 1 | cut -d= -f2- || true
}

resolve_acme_email() {
	local email="${CADDY_ACME_EMAIL:-}"
	if [[ -z "${email}" && -f "${PROD_ROOT}/app-node/.env.app-node" ]]; then
		email="$(read_env_value CADDY_ACME_EMAIL "${PROD_ROOT}/app-node/.env.app-node")"
	fi
	if [[ -z "${email}" && -f "${LEGACY_ENV}" ]]; then
		email="$(read_env_value CADDY_ACME_EMAIL "${LEGACY_ENV}")"
	fi
	printf '%s' "${email}"
}

free_http_port() {
	fuser -k 80/tcp 2>/dev/null || true
	sleep 1
}

pause_edge_http() {
	if [[ -f "${LEGACY_ENV}" && -f "${LEGACY_COMPOSE}" ]]; then
		if docker compose --env-file "${LEGACY_ENV}" -f "${LEGACY_COMPOSE}" ps --status running --services 2>/dev/null | grep -qx caddy; then
			note "stop legacy caddy for ACME HTTP-01"
			docker compose --env-file "${LEGACY_ENV}" -f "${LEGACY_COMPOSE}" stop caddy
			CADDY_STOPPED=1
		fi
	fi
	free_http_port
}

resume_edge_http() {
	if [[ "${CADDY_STOPPED}" -eq 1 && -f "${LEGACY_ENV}" && -f "${LEGACY_COMPOSE}" ]]; then
		note "start legacy caddy after ACME"
		docker compose --env-file "${LEGACY_ENV}" -f "${LEGACY_COMPOSE}" start caddy || true
	fi
}

trap resume_edge_http EXIT

require_file "${EMQX_DIR}/base.hocon"
require_dir "${CERT_DIR}"

ACME_EMAIL="$(resolve_acme_email)"
[[ -n "${ACME_EMAIL}" && "${ACME_EMAIL}" != *"@example.com"* ]] || {
	fail "CADDY_ACME_EMAIL must be set to a real address (not ops@example.com) in .env.app-node or environment"
}

if ! command -v certbot >/dev/null 2>&1; then
	note "install certbot"
	apt-get update -qq
	DEBIAN_FRONTEND=noninteractive apt-get install -y -qq certbot
fi

issue_or_renew_cert() {
	certbot certonly --standalone --non-interactive --agree-tos \
		--email "${ACME_EMAIL}" -d "${MQTT_DOMAIN}" \
		--preferred-challenges http --http-01-port 80 "$@"
}

note "ensure LE certificate for ${MQTT_DOMAIN}"
pause_edge_http
if [[ ! -f "${LE_DIR}/fullchain.pem" ]]; then
	issue_or_renew_cert
else
	issue_or_renew_cert --force-renewal
fi

[[ -f "${LE_DIR}/fullchain.pem" && -f "${LE_DIR}/privkey.pem" ]] || fail "missing LE cert under ${LE_DIR}"
openssl x509 -in "${LE_DIR}/fullchain.pem" -noout -checkend 86400 >/dev/null 2>&1 \
	|| fail "LE certificate for ${MQTT_DOMAIN} is still expired or expires within 24h"

note "sync LE material into ${CERT_DIR}"
cp "${LE_DIR}/fullchain.pem" "${CERT_DIR}/server.crt"
cp "${LE_DIR}/privkey.pem" "${CERT_DIR}/server.key"
cp "${LE_DIR}/chain.pem" "${CERT_DIR}/ca.crt"
cp "${LE_DIR}/fullchain.pem" "${CERT_DIR}/cert.pem"
cp "${LE_DIR}/privkey.pem" "${CERT_DIR}/key.pem"
cp "${LE_DIR}/chain.pem" "${CERT_DIR}/cacert.pem"
chmod 644 "${CERT_DIR}/server.crt" "${CERT_DIR}/ca.crt" "${CERT_DIR}/cert.pem" "${CERT_DIR}/cacert.pem"
chmod 600 "${CERT_DIR}/server.key" "${CERT_DIR}/key.pem"

openssl x509 -in "${CERT_DIR}/server.crt" -noout -dates -subject

note "recreate EMQX to load renewed TLS material"
ENV_FILE="${NODE_ROOT}/.env.data-node"
COMPOSE_FILE="${NODE_ROOT}/docker-compose.data-node.yml"
COMPOSE=(docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}")
require_file "${ENV_FILE}"
require_file "${COMPOSE_FILE}"
"${COMPOSE[@]}" up -d --no-deps --force-recreate emqx
run_script "${NODE_ROOT}/scripts/bootstrap_emqx_data_node.sh"
bash "${NODE_ROOT}/scripts/install_emqx_acl.sh"

note "verify public TLS listener presents renewed cert"
for i in $(seq 1 30); do
	if openssl s_client -connect "127.0.0.1:8883" -servername "${MQTT_DOMAIN}" </dev/null 2>/dev/null \
		| openssl x509 -noout -checkend 86400 >/dev/null 2>&1; then
		echo "renew_emqx_mqtt_tls: PASS"
		exit 0
	fi
	sleep 2
done

fail "EMQX TLS listener did not present a cert valid for at least 24h"
