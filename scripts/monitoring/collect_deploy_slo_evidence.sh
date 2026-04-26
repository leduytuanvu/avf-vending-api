#!/usr/bin/env bash
# Optional deploy SLO / observability evidence for production rollouts.
# - Exits non-zero when DEPLOY_SLO_CRITICAL=1 and public liveness+readiness are
#   not both HTTP 2xx while a public BASE_URL is set.
# - No paid third-party services; only curl and optional SSH to app nodes.
# - Does not log secrets: no auth headers, no .env, no keys in output.
set -Eeuo pipefail

JSON_MODE=0
PHASE=""
OUT_PATH=""

usage() {
	cat <<'EOF'
Usage: collect_deploy_slo_evidence.sh --json --phase PHASE [--out PATH]

  --phase  pre_deploy | post_node_a | post_final
  --out    Write JSON to PATH (default: stdout)
  --json   Required.

See docs/operations/deploy-monitoring-slo.md for environment variables.
EOF
}

while [[ $# -gt 0 ]]; do
	case "$1" in
		--json)
			JSON_MODE=1
			shift
			;;
		--phase)
			PHASE="${2-}"
			shift 2
			;;
		--out)
			OUT_PATH="${2-}"
			shift 2
			;;
		--help | -h)
			usage
			exit 0
			;;
		*)
			echo "error: unknown argument: $1" >&2
			exit 2
			;;
	esac
done

if [[ "${JSON_MODE}" != "1" ]]; then
	echo "error: --json is required" >&2
	exit 2
fi

case "${PHASE}" in
	pre_deploy | post_node_a | post_final) ;;
	*)
		echo "error: invalid --phase" >&2
		exit 2
		;;
esac

base_raw="${BASE_URL:-${SMOKE_BASE_URL:-${PRODUCTION_PUBLIC_BASE_URL:-}}}"
base_raw="${base_raw#"${base_raw%%[![:space:]]*}"}"
base_raw="${base_raw%${base_raw##*[![:space:]]}}"
base_url="${base_raw%/}"

critical_mode=0
[[ "${DEPLOY_SLO_CRITICAL:-0}" == "1" ]] && critical_mode=1

deploy_b="${DEPLOY_APP_NODE_B:-true}"
if [[ "${deploy_b}" != "true" && "${deploy_b}" != "false" ]]; then
	deploy_b="true"
fi

final_smoke_path="${FINAL_SMOKE_JSON:-deployment-evidence/smoke-cluster-final.json}"
node_a_smoke_path="${NODE_A_SMOKE_JSON:-deployment-evidence/smoke-app-node-a.json}"
ssh_user="${SSH_USER:-}"
ssh_port="${SSH_PORT:-22}"
prod_root="${PRODUCTION_DEPLOY_ROOT:-}"
app_a="${APP_NODE_A_HOST:-}"
app_b="${APP_NODE_B_HOST:-}"
# shellcheck disable=SC2206
read -r -a ssh_opts_a <<< "${SSH_OPTS:-}"

frag_dir="${TMPDIR:-/tmp}/slo-$$"
mkdir -p "${frag_dir}"
cleanup() { rm -rf "${frag_dir}" || true; }
trap cleanup EXIT

# --- Public HTTP: readiness + liveness + version (unauthenticated) ---
# shellcheck disable=SC2310
http_probe() {
	local url="$1"
	local pfx="$2"
	local errf bdf
	errf="${pfx}.err"
	bdf="${pfx}.body"
	: >"${errf}"
	set +e
	local code
	code="$(
		curl -sS -L -o "${bdf}" -w '%{http_code}' --max-time 25 \
			-H "Accept: */*" -H "User-Agent: avf-slo-collect/1" \
			"${url}" 2>"${errf}"
	)" || code=""
	local t
	t="$(
		curl -sS -L -o /dev/null -w '%{time_total}' --max-time 25 \
			-H "Accept: */*" -H "User-Agent: avf-slo-collect/1" \
			"${url}" 2>/dev/null
	)" || t=""
	set -e
	if [[ -z "${code}" ]]; then
		printf 'unavailable' >"${pfx}.code"
		printf '' >"${pfx}.time"
		{ cat "${errf}" 2>/dev/null || true; } | head -c 500 >"${pfx}.err"
		: >"${bdf}"
		return 0
	fi
	if ! [[ "${t}" =~ ^[0-9.]+$ ]]; then
		t=""
	fi
	printf '%s' "${code}" >"${pfx}.code"
	if [[ -n "${t}" ]]; then
		printf '%s' "${t}" >"${pfx}.time"
	else
		printf '' >"${pfx}.time"
	fi
	: >"${errf}"
}

if [[ -n "${base_url}" ]]; then
	http_probe "${base_url}/health/ready" "${frag_dir}/r"
	http_probe "${base_url}/health/live" "${frag_dir}/l"
	http_probe "${base_url}/version" "${frag_dir}/v"
else
	for p in r l v; do
		printf 'unavailable' >"${frag_dir}/${p}.code"
		printf '' >"${frag_dir}/${p}.time"
		printf 'no_public_base_url' >"${frag_dir}/${p}.err"
		: >"${frag_dir}/${p}.body"
	done
fi

# --- Optional SSH to each app node: compose ps, df, memory summary, log error count ---
# shellcheck disable=SC2310
run_remote() {
	local host="$1"
	local outf="$2"
	[[ -n "${host}" && -n "${ssh_user}" && -n "${prod_root}" ]] || {
		printf 'unavailable' >"${outf}.tag"
		return 0
	}
	set +e
	# shellcheck disable=SC2029
	ssh -p "${ssh_port}" -o BatchMode=yes -o ConnectTimeout=20 "${ssh_opts_a[@]:+${ssh_opts_a[@]}}" \
		"${ssh_user}@${host}" "bash -s" -- "${prod_root}/deployments/prod/app-node" >"${outf}.out" 2>"${outf}.err" <<'RSCRIPT'
set -euo pipefail
APP_DIR="${1:-}"
cd "${APP_DIR}" || { echo "__SLO_FAIL__cd"; exit 0; }
echo __SLO_DF__
df -P / 2>&1 | head -n 5
echo __SLO_MEM__
{ free -b 2>&1 | head -n 4; } || { grep -E '^(MemTotal|MemAvailable):' /proc/meminfo 2>&1; } || true
echo __SLO_PS__
docker compose -f docker-compose.app-node.yml ps 2>&1 | head -n 200
echo __SLO_ERRC__
LOGS="$(docker compose -f docker-compose.app-node.yml logs --since 30m api 2>/dev/null || true)"
n=0
if [ -n "${LOGS}" ]; then
	set +e
	n=$(printf '%s' "${LOGS}" | grep -cE 'ERROR|FATAL')
	set -e
	n=${n:-0}
fi
printf '%s\n' "${n}"
RSCRIPT
	local rc=$?
	set -e
	if [[ "${rc}" -ne 0 ]]; then
		{
			head -c 1200 <"${outf}.err" 2>/dev/null || true
			head -c 400 <"${outf}.out" 2>/dev/null || true
		} | tr '\r' ' ' | sed -E 's/(Bearer[[:space:]]+)[-A-Za-z0-9._~+/=]+/\1<redacted>/g' >"${outf}.serr"
		printf 'ssh_fail' >"${outf}.tag"
		return 0
	fi
	printf 'ok' >"${outf}.tag"
}

run_remote "${app_a}" "${frag_dir}/a"
if [[ "${deploy_b}" == "true" && -n "${app_b}" ]]; then
	run_remote "${app_b}" "${frag_dir}/b"
else
	printf 'skipped_no_second_node' >"${frag_dir}/b.tag"
	: >"${frag_dir}/b.out" "${frag_dir}/b.err" "${frag_dir}/b.serr"
fi

# --- Assemble JSON with Python (single pass; no secrets) ---
SLO_OUT_PATH="${OUT_PATH:-}" \
	SLO_BASE_URL="${base_url}" \
	SLO_FRAG="${frag_dir}" \
	SLO_PHASE="${PHASE}" \
	SLO_CRITICAL_MODE="${critical_mode}" \
	SLO_FINAL_SMOKE_PATH="${final_smoke_path}" \
	SLO_NODE_A_SMOKE_PATH="${node_a_smoke_path}" \
	python3 - <<'PY'
import json
import os
import re
import time
from pathlib import Path

schema_version = 1
frag = os.environ["SLO_FRAG"]
phase = os.environ["SLO_PHASE"]
critical = os.environ.get("SLO_CRITICAL_MODE", "0") == "1"
base_url = os.environ.get("SLO_BASE_URL", "").strip()

def readf(name: str) -> str:
    p = Path(frag) / name
    if p.exists():
        return p.read_text(encoding="utf-8", errors="replace")
    return ""

def read_body_max(name: str, max_bytes: int = 2048) -> str:
    raw = readf(name)
    if not raw:
        return ""
    return raw[:max_bytes]

def status_from_code(code: str) -> str:
    c = (code or "").strip()
    if c == "unavailable" or c == "":
        return "unavailable"
    if c.isdigit() and c.startswith("2"):
        return "pass"
    if c.isdigit():
        return "fail"
    return "unavailable"

def read_smoke(path_str: str):
    p = Path(path_str)
    if not p.is_file():
        return "unavailable", "missing", False
    try:
        data = json.loads(p.read_text(encoding="utf-8"))
    except Exception as exc:
        return "unavailable", f"invalid_json:{type(exc).__name__}", False
    st = data.get("overall_status") or "unavailable"
    return st, "ok", True

node_a_p = os.environ["SLO_NODE_A_SMOKE_PATH"]
final_p = os.environ["SLO_FINAL_SMOKE_PATH"]
na_s, na_note, _na_ok = read_smoke(node_a_p)
fn_s, fn_note, _fn_ok = read_smoke(final_p)

# Phase-specific smoke display (no fake success for missing optional files)
if phase == "pre_deploy":
    smoke_a = {
        "ref_path": node_a_p,
        "overall_status": "not_run",
        "note": "not_applicable",
    }
    if base_url:
        smoke_f = {
            "ref_path": final_p,
            "overall_status": "not_run",
            "note": "not_applicable_pre_deploy",
        }
    else:
        smoke_f = {
            "ref_path": final_p,
            "overall_status": "unavailable",
            "note": "no_public_base_url",
        }
elif phase == "post_node_a":
    smoke_a = {"ref_path": node_a_p, "overall_status": na_s, "note": na_note}
    smoke_f = {
        "ref_path": final_p,
        "overall_status": "not_run",
        "note": "not_applicable_before_final_smoke",
    }
else:
    smoke_a = {"ref_path": node_a_p, "overall_status": na_s, "note": na_note}
    smoke_f = {"ref_path": final_p, "overall_status": fn_s, "note": fn_note}

ready_code = readf("r.code").strip()
live_code = readf("l.code").strip()
ver_code = readf("v.code").strip()
ready_t = readf("r.time").strip()
live_t = readf("l.time").strip()
ver_t = readf("v.time").strip()
ready_err = readf("r.err").strip()
live_err = readf("l.err").strip()
ver_err = readf("v.err").strip()
ver_body = read_body_max("v.body", 4000).strip()

version_obj = {
    "http_code": ver_code or "unavailable",
    "response_time_s_sample": None if not ver_t else float(ver_t) if re.match(r"^[0-9.]+$", ver_t) else None,
    "status": status_from_code(ver_code),
    "info_truncated": ver_body,
}
if not base_url:
    version_obj = {
        "http_code": "unavailable",
        "response_time_s_sample": None,
        "status": "unavailable",
        "info_truncated": "",
        "note": "no_public_base_url",
    }
elif not ver_body:
    if status_from_code(ver_code) == "pass":
        version_obj["info_truncated"] = ""

critical_health_ok = True
if base_url and critical:
    for c in (readf("r.code"), readf("l.code")):
        cc = c.strip()
        if cc == "unavailable" or (cc.isdigit() and not cc.startswith("2")):
            critical_health_ok = False
            break

def parse_remote(tag_path: str, out_path: str, err_path: str) -> dict:
    tag = Path(tag_path).read_text(encoding="utf-8").strip() if Path(tag_path).is_file() else "missing"
    if tag == "unavailable":
        return {"status": "unavailable", "note": "ssh_or_host_unconfigured"}
    if tag == "ssh_fail":
        serr = Path(err_path).read_text(encoding="utf-8", errors="replace") if Path(err_path).is_file() else ""
        serr = serr[:800]
        return {"status": "unavailable", "note": "ssh_command_failed", "detail": serr}
    if tag == "skipped_no_second_node":
        return {"status": "skipped", "note": "second_app_node_not_in_scope"}
    if tag != "ok":
        return {"status": "unavailable", "note": f"unknown_tag_{tag}"}
    raw = Path(out_path).read_text(encoding="utf-8", errors="replace")
    m_err = re.search(r"__SLO_ERRC__\s*\n([0-9]+)\s*(?:$|\Z)", raw, re.M | re.S)
    err_count = m_err.group(1) if m_err else None
    m_ps = re.search(r"__SLO_PS__\s*\n(.*?)(?:\n__SLO_ERRC__|\Z)", raw, re.S)
    m_df = re.search(r"__SLO_DF__\s*\n(.*?)(?:\n__SLO_MEM__|\Z)", raw, re.S)
    m_mem = re.search(r"__SLO_MEM__\s*\n(.*?)(?:\n__SLO_PS__|\Z)", raw, re.S)
    return {
        "status": "ok",
        "docker_compose_ps_head": (m_ps.group(1).strip()[:6000] if m_ps else ""),
        "disk_free_head": (m_df.group(1).strip()[:2000] if m_df else ""),
        "memory_head": (m_mem.group(1).strip()[:2000] if m_mem else ""),
        "recent_errorish_line_count_30m_api": int(err_count) if err_count and err_count.isdigit() else None,
    }

nodes = {
    "app_node_a": parse_remote(f"{frag}/a.tag", f"{frag}/a.out", f"{frag}/a.serr"),
    "app_node_b": parse_remote(f"{frag}/b.tag", f"{frag}/b.out", f"{frag}/b.serr"),
}

out = {
    "schema_version": schema_version,
    "phase": phase,
    "collected_at_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
    "collector": "collect_deploy_slo_evidence.sh",
    "public_base_url_configured": bool(base_url),
    "critical": {
        "mode_enabled": critical,
        "assessment": "pass" if (not base_url or not critical or critical_health_ok) else "fail",
        "public_health_ready": {
            "http_code": readf("r.code").strip() or "unavailable",
            "response_time_s_sample": (float(ready_t) if re.match(r"^[0-9.]+$", (ready_t or "").strip()) else None),
            "status": status_from_code(ready_code),
            "error_detail": (readf("r.err")[:500] if readf("r.err") else None),
        },
        "public_health_live": {
            "http_code": readf("l.code").strip() or "unavailable",
            "response_time_s_sample": (float(live_t) if re.match(r"^[0-9.]+$", (live_t or "").strip()) else None),
            "status": status_from_code(live_code),
            "error_detail": (readf("l.err")[:500] if readf("l.err") else None),
        },
    },
    "optional": {
        "public_version": version_obj,
        "public_smoke": {
            "node_a_smoke_json": smoke_a,
            "final_cluster_smoke_json": smoke_f,
        },
        "vps": nodes,
    },
    "policies": {
        "no_paid_hosting_monitoring": True,
        "optional_metrics_marked_unavailable_when_missing": True,
    },
}
out_path = os.environ.get("SLO_OUT_PATH", "").strip()
text = json.dumps(out, indent=2, ensure_ascii=True) + "\n"
if out_path:
    outp = Path(out_path)
    outp.parent.mkdir(parents=True, exist_ok=True)
    outp.write_text(text, encoding="utf-8")
else:
    print(text, end="")
rc = 0
if critical and base_url and not critical_health_ok:
    rc = 1
raise SystemExit(rc)
PY
exit $?
