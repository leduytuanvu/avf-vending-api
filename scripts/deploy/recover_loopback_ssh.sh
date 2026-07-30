#!/usr/bin/env bash
# Restore loopback SSH on app-node A (self-hosted runner co-located with production).
# Used by deploy-prod and production-sshd-recover-and-deploy when github-runner must SSH to root@127.0.0.1.
set -Eeuo pipefail

fail() {
  echo "error: $*" >&2
  exit 1
}

[[ -n "${SSH_IDENTITY_FILE:-}" ]] || fail "SSH_IDENTITY_FILE is required"
[[ -f "${SSH_IDENTITY_FILE}" ]] || fail "SSH identity file not found: ${SSH_IDENTITY_FILE}"
[[ -n "${SSH_USER:-}" ]] || fail "SSH_USER is required"

SSH_PORT="${SSH_PORT:-22}"
PRIV_HELPER="${PRIV_HELPER:-/tmp/avf-prod-root.sh}"

run_privileged() {
  local cmd="$1"
  if [[ -x "${PRIV_HELPER}" ]]; then
    PRODUCTION_ROOT_PASSWORD="${PRODUCTION_ROOT_PASSWORD:-}" SSH_USER="${SSH_USER}" "${PRIV_HELPER}" "${cmd}"
    return $?
  fi
  if [[ "$(id -un)" == "${SSH_USER}" ]]; then
    bash -lc "${cmd}"
  elif sudo -n true 2>/dev/null; then
    sudo bash -lc "${cmd}"
  elif [[ -n "${PRODUCTION_ROOT_PASSWORD:-}" ]]; then
    printf '%s\n' "${PRODUCTION_ROOT_PASSWORD}" | su - "${SSH_USER}" -c "${cmd}"
  else
    fail "no privileged execution path (set PRIV_HELPER or PRODUCTION_ROOT_PASSWORD)"
  fi
}

pub_key="$(ssh-keygen -y -f "${SSH_IDENTITY_FILE}")"
user_home="$(getent passwd "${SSH_USER}" | cut -d: -f6 || true)"
[[ -n "${user_home}" ]] || user_home="/root"

run_privileged "set -Eeuo pipefail
  auth_dir='${user_home}/.ssh'
  auth_file=\"\${auth_dir}/authorized_keys\"
  mkdir -p \"\${auth_dir}\"
  chmod 700 \"\${auth_dir}\"
  touch \"\${auth_file}\"
  grep -Fq '${pub_key}' \"\${auth_file}\" 2>/dev/null || echo '${pub_key} github-actions-production-deploy' >> \"\${auth_file}\"
  chmod 600 \"\${auth_file}\"
  command -v fail2ban-client >/dev/null 2>&1 && fail2ban-client status sshd >/dev/null 2>&1 && {
    fail2ban-client set sshd unbanip 127.0.0.1 2>/dev/null || true
    fail2ban-client unban --all 2>/dev/null || true
  } || true
  install -d -m 755 /run/sshd
  sshd -t
  systemctl reset-failed ssh ssh.socket sshd 2>/dev/null || true
  systemctl stop ssh.socket 2>/dev/null || true
  systemctl disable ssh.socket 2>/dev/null || true
  systemctl enable ssh.service 2>/dev/null || systemctl enable sshd 2>/dev/null || true
  systemctl start ssh.service 2>/dev/null || systemctl start sshd 2>/dev/null || true
  sleep 2
  systemctl is-active ssh.service 2>/dev/null || systemctl is-active sshd 2>/dev/null || systemctl is-active ssh 2>/dev/null"

mkdir -p "${HOME}/.ssh"
chmod 700 "${HOME}/.ssh"
touch "${HOME}/.ssh/known_hosts"
ssh-keyscan -T 10 -p "${SSH_PORT}" -H 127.0.0.1 >> "${HOME}/.ssh/known_hosts" 2>/dev/null || true

set +e
ssh -o BatchMode=yes -o ConnectTimeout=10 -o StrictHostKeyChecking=yes \
  -i "${SSH_IDENTITY_FILE}" -p "${SSH_PORT}" "${SSH_USER}@127.0.0.1" "printf 'loopback-ssh-ok\n'"
probe_rc=$?
set -e

if [[ "${probe_rc}" -ne 0 ]]; then
  echo "error: loopback SSH probe failed for ${SSH_USER}@127.0.0.1 (rc=${probe_rc})" >&2
  exit 1
fi

echo "Loopback SSH probe passed"
