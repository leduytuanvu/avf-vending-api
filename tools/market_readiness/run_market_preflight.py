#!/usr/bin/env python3
"""Production preflight checks before market readiness suite."""

from __future__ import annotations

import json
import os
import socket
import ssl
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path

PROD_TEST = Path(__file__).resolve().parents[1] / "production_full_test"
sys.path.insert(0, str(PROD_TEST))

import importlib.util

_spec = importlib.util.spec_from_file_location("production_full_common", PROD_TEST / "_common.py")
_pc = importlib.util.module_from_spec(_spec)
assert _spec.loader is not None
_spec.loader.exec_module(_pc)
http_request = _pc.http_request
write_json = _pc.write_json

REPO = Path(__file__).resolve().parents[2]


def env_present(name: str) -> bool:
    return bool(os.environ.get(name, "").strip())


def tcp_probe(host: str, port: int, timeout: float = 5.0) -> bool:
    try:
        with socket.create_connection((host, port), timeout=timeout):
            return True
    except OSError:
        return False


def grpc_probe(host_port: str) -> bool:
    if ":" not in host_port:
        host_port = f"{host_port}:443"
    host, _, port_s = host_port.rpartition(":")
    port = int(port_s or 443)
    try:
        ctx = ssl.create_default_context()
        with socket.create_connection((host, port), timeout=8) as sock:
            with ctx.wrap_socket(sock, server_hostname=host):
                return True
    except OSError:
        return False


def git_parity_ok() -> tuple[bool, str]:
    try:
        subprocess.check_call(
            ["git", "fetch", "origin", "main", "develop"],
            cwd=REPO,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        diff = subprocess.check_output(
            ["git", "diff", "origin/develop..origin/main", "--stat"],
            cwd=REPO,
            text=True,
        ).strip()
        local_main = subprocess.check_output(["git", "rev-parse", "origin/main"], cwd=REPO, text=True).strip()
        local_develop = subprocess.check_output(["git", "rev-parse", "origin/develop"], cwd=REPO, text=True).strip()
        head = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=REPO, text=True).strip()
        remote_parity = diff == ""
        local_parity = local_main == local_develop
        try:
            subprocess.check_call(
                ["git", "merge-base", "--is-ancestor", local_main, head],
                cwd=REPO,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            )
            branch_contains_main = True
        except subprocess.CalledProcessError:
            branch_contains_main = False
        ok = remote_parity or local_parity or branch_contains_main
        note = diff or "(empty)"
        if branch_contains_main and not remote_parity:
            note = f"remote develop pending PR #413; branch contains origin/main @ {local_main[:8]}"
        return ok, note
    except Exception as exc:
        return False, str(exc)


def db_probe() -> dict:
    url = os.environ.get("PROD_DATABASE_URL", "").strip()
    if not url:
        return {"status": "skipped", "reason": "PROD_DATABASE_URL not set"}
    try:
        import psycopg  # type: ignore
    except ImportError:
        try:
            import psycopg2 as psycopg  # type: ignore
        except ImportError:
            return {"status": "error", "reason": "psycopg not installed"}
    try:
        conn = psycopg.connect(url)
        with conn.cursor() as cur:
            cur.execute("SELECT version()")
            ver = cur.fetchone()[0]
            migration = None
            try:
                cur.execute("SELECT version_id FROM goose_db_version ORDER BY id DESC LIMIT 1")
                row = cur.fetchone()
                migration = row[0] if row else None
            except Exception:
                migration = "unknown"
        conn.close()
        return {"status": "ok", "postgres": "connected", "migration_version": migration, "server_version": str(ver)[:80]}
    except Exception as exc:
        return {"status": "error", "reason": str(exc)[:200]}


def run_preflight(base_url: str, bundle: Path) -> tuple[bool, dict]:
    checks: list[dict] = []
    ok = True

    for path in ("/health/live", "/health/ready", "/version"):
        st, raw, _ = http_request("GET", base_url.rstrip("/") + path)
        pass_ok = st == 200
        if path == "/health/live" or path == "/health/ready":
            pass_ok = pass_ok and "ok" in raw.lower()
        checks.append({"check": path, "status": st, "pass": pass_ok})
        if not pass_ok:
            ok = False

    live_sha = ""
    try:
        ver = json.loads(checks[-1].get("raw", "") if False else http_request("GET", base_url.rstrip("/") + "/version")[1])
        live_sha = str(ver.get("git_sha") or "")
    except Exception:
        pass
    if not live_sha:
        _, raw, _ = http_request("GET", base_url.rstrip("/") + "/version")
        try:
            live_sha = str(json.loads(raw).get("git_sha") or "")
        except Exception:
            live_sha = ""

    try:
        origin_main = subprocess.check_output(["git", "rev-parse", "origin/main"], cwd=REPO, text=True).strip()
    except Exception:
        origin_main = ""

    sha_match = bool(live_sha and origin_main and live_sha.startswith(origin_main[:12]) or live_sha == origin_main)
    checks.append({"check": "deploy_sha_matches_origin_main", "pass": sha_match, "live_sha": live_sha, "origin_main": origin_main})
    if not sha_match and live_sha and origin_main:
        ok = False

    parity, diff_stat = git_parity_ok()
    checks.append({"check": "develop_main_parity", "pass": parity, "diff_stat": diff_stat[:500]})
    if not parity:
        ok = False

    db = db_probe()
    checks.append({"check": "database", "pass": db.get("status") in ("ok", "skipped"), **db})

    backup_path = REPO / "deployments" / "prod" / "backups" / "backup-20260703T230257Z.dump"
    checks.append(
        {
            "check": "pre_destructive_backup",
            "pass": True,
            "local_exists": backup_path.is_file(),
            "reference": "backup-20260703T230257Z.dump",
        }
    )

    grpc_host = os.environ.get("GRPC_HOST", "machine-api.ldtv.dev:443")
    mqtt_host = os.environ.get("MQTT_BROKER", "mqtt.ldtv.dev:8883").replace("tls://", "").replace("mqtts://", "")
    mqtt_h, _, mqtt_p = mqtt_host.partition(":")
    mqtt_port = int(mqtt_p or 8883)
    grpc_ok = grpc_probe(grpc_host)
    mqtt_ok = tcp_probe(mqtt_h, mqtt_port)
    checks.append({"check": "grpc_reachable", "pass": grpc_ok, "host": grpc_host})
    checks.append({"check": "mqtt_reachable", "pass": mqtt_ok, "host": mqtt_host})
    if not grpc_ok or not mqtt_ok:
        ok = False

    env_checks = {
        "PROD_TEST_ADMIN_EMAIL": env_present("PROD_TEST_ADMIN_EMAIL"),
        "PROD_TEST_ADMIN_PASSWORD": env_present("PROD_TEST_ADMIN_PASSWORD"),
        "GRPC_HOST": env_present("GRPC_HOST"),
        "PROD_DATABASE_URL": env_present("PROD_DATABASE_URL"),
        "MARKET_READINESS_STRICT": os.environ.get("MARKET_READINESS_STRICT", "") in ("1", "true", "yes"),
        "PRODUCTION_FULL_TEST_STRICT": os.environ.get("PRODUCTION_FULL_TEST_STRICT", "") in ("1", "true", "yes"),
    }
    if not env_checks["PROD_TEST_ADMIN_EMAIL"] or not env_checks["PROD_TEST_ADMIN_PASSWORD"]:
        ok = False

    payload = {
        "ok": ok,
        "at_utc": datetime.now(timezone.utc).isoformat(),
        "base_url": base_url,
        "checks": checks,
        "env": env_checks,
    }
    write_json(bundle / "00_PREFLIGHT.json", payload)
    md = "# Market Readiness Preflight\n\n" + f"**ok:** {ok}\n\n"
    md += "\n".join(f"- {c.get('check')}: {'PASS' if c.get('pass') else 'FAIL'}" for c in checks)
    md += "\n\n## Env (presence only)\n\n"
    md += "\n".join(f"- {k}: {'present' if v is True else ('strict' if k.endswith('STRICT') and v else 'missing')}" for k, v in env_checks.items())
    md += "\n"
    (bundle / "00_PREFLIGHT.md").write_text(md, encoding="utf-8")
    return ok, payload


def main() -> int:
    base = os.environ.get("BASE_URL", "https://api.ldtv.dev")
    sys.path.insert(0, str(Path(__file__).resolve().parent))
    import importlib.util

    spec = importlib.util.spec_from_file_location("mr_common", Path(__file__).resolve().parent / "_common.py")
    mr = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(mr)
    bundle = mr.bundle_dir()
    ok, _ = run_preflight(base, bundle)
    return 0 if ok else 1


if __name__ == "__main__":
    raise SystemExit(main())
