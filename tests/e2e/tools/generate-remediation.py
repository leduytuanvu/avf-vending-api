#!/usr/bin/env python3
"""Generate reports/remediation.md with structured failure rows."""

from __future__ import annotations

import hashlib
import json
import re
from pathlib import Path
from typing import Any


def redact_message(text: str) -> str:
    if not text:
        return text
    s = text
    s = re.sub(
        r"eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_.-]+",
        "***jwt***",
        s,
    )
    s = re.sub(r"(?i)\bBearer\s+[A-Za-z0-9._=-]{12,}", "Bearer ***", s)
    return s


def read_jsonl(path: Path) -> list[dict[str, Any]]:
    if not path.is_file():
        return []
    rows: list[dict[str, Any]] = []
    for line in path.read_text(encoding="utf-8-sig", errors="replace").splitlines():
        line = line.strip()
        if not line:
            continue
        try:
            rows.append(json.loads(line))
        except json.JSONDecodeError:
            continue
    return rows


def infer_protocol(step: str, msg: str) -> str:
    s = (step + " " + msg).lower()
    if "grpc" in s or s.startswith("grpc-"):
        return "gRPC"
    if "mqtt" in s or "mosquitto" in s:
        return "MQTT"
    if "newman" in s or "http:" in s or "rest:" in s or "/v1/" in msg:
        return "REST"
    if "phase8" in s:
        return "mixed"
    return "shell"


def guess_endpoint(step: str, msg: str) -> str:
    m = re.search(r"(POST|GET|PUT|PATCH|DELETE)\s+(/[^\s`]+)", msg)
    if m:
        return f"{m.group(1)} {m.group(2)}"
    m = re.search(r"(/v1/[^\s`\"]+)", msg)
    if m:
        return m.group(1)
    m = re.search(r"avf\.machine\.v1\.(\S+/\S+)", msg)
    if m:
        return m.group(1)
    m = re.search(r"topic[:\s]+(\S+)", msg, re.I)
    if m:
        return m.group(1)
    return "—"


def guess_evidence(run_dir: Path, step: str, protocol: str) -> str:
    rest_resp = run_dir / "rest" / f"{step}.response.json"
    if rest_resp.is_file():
        return str(rest_resp)
    if protocol == "gRPC":
        gdir = run_dir / "grpc"
        if gdir.is_dir():
            for p in sorted(gdir.glob("*.log")):
                return str(p)
    if protocol == "MQTT":
        mdir = run_dir / "mqtt"
        if mdir.is_dir():
            for p in sorted(mdir.glob("*.log")):
                return str(p)
    return str(run_dir / "events.jsonl")


def hint_for(msg: str) -> tuple[str, str, str]:
    mlow = msg.lower()
    if "401" in msg or "unauthenticated" in mlow:
        return (
            "Missing/expired bearer token or wrong audience.",
            "Refresh `ADMIN_TOKEN` / machine JWT; copy `secrets.private.json` from a good run.",
            "yes",
        )
    if "403" in msg or "forbidden" in mlow:
        return (
            "Principal lacks permission for route or org scope.",
            "Grant role (e.g. audit.read, inventory); align `organizationId`.",
            "yes",
        )
    if "connection refused" in mlow or " http 0" in mlow or "000" in msg:
        return (
            "API not listening on BASE_URL or wrong port.",
            "Start `cmd/api`; verify `BASE_URL` matches listen address.",
            "yes",
        )
    if "grpc" in mlow and ("unavailable" in mlow or "refused" in mlow):
        return (
            "gRPC listener not reachable at GRPC_ADDR.",
            "Enable gRPC in local config; check `GRPC_ADDR` and `grpc/*.log`.",
            "yes",
        )
    if "mqtt" in mlow or "broker" in mlow:
        return (
            "MQTT broker down, ACL, or wrong topic.",
            "Start mosquitto/EMQX; set `MQTT_*`; see mqtt-contract.md.",
            "yes",
        )
    if "503" in msg and "capability" in mlow:
        return (
            "Optional capability/outbox not configured (e.g. payment-session).",
            "Set commerce outbox env vars or skip scenario locally.",
            "yes",
        )
    if "already" in mlow or "collision" in mlow or "409" in msg:
        return (
            "Idempotency or unique constraint conflict / reused seed.",
            "Use `--fresh-data` or new IDs; rotate idempotency keys safely.",
            "no",
        )
    return (
        "See message and evidence artifacts.",
        "Consult `docs/testing/e2e-remediation-playbook.md` for the failing phase.",
        "caution",
    )


def main() -> int:
    import argparse

    ap = argparse.ArgumentParser()
    ap.add_argument("--run-dir", type=Path, required=True)
    ap.add_argument("--playbook", type=Path, required=True)
    args = ap.parse_args()
    rd = args.run_dir.resolve()
    rep = rd / "reports"
    rep.mkdir(parents=True, exist_ok=True)
    events = read_jsonl(rd / "events.jsonl")
    failed: list[dict[str, Any]] = [e for e in events if e.get("status") == "failed"]

    te = read_jsonl(rd / "test-events.jsonl")
    for row in te:
        st = str(row.get("status", "")).lower()
        if st in ("fail", "failed"):
            failed.append(
                {
                    "ts": row.get("ts", ""),
                    "step": f"{row.get('flow_id', 'flow')}-{row.get('step_name', 'step')}",
                    "status": "failed",
                    "message": f"{row.get('protocol', '')} {row.get('endpoint', '')} — {row.get('message', '')}",
                }
            )

    lines = [
        "# E2E remediation report",
        "",
        f"Run directory: `{rd}`",
        "",
        f"Structured hints for **{len(failed)}** failure(s). Full playbook: `docs/testing/e2e-remediation-playbook.md` (repo root).",
        "",
    ]
    if not failed:
        lines.extend(["## Result", "", "**No failed steps in `events.jsonl` / `test-events.jsonl`.**", ""])
        (rep / "remediation.md").write_text("\n".join(lines), encoding="utf-8")
        return 0

    for i, e in enumerate(failed, start=1):
        step = str(e.get("step", ""))
        msg = str(e.get("message", ""))
        fid = f"F-{i:03d}-{hashlib.sha256((step + msg).encode()).hexdigest()[:8]}"
        proto = infer_protocol(step, msg)
        endpoint = guess_endpoint(step, msg)
        cause, fix, reuse = hint_for(msg)
        evidence = guess_evidence(rd, step, proto)
        scenario = step.split("-")[0] if step else "unknown"
        lines.append(f"## {fid}")
        lines.append("")
        lines.append("| Field | Value |")
        lines.append("|-------|-------|")
        lines.append(f"| **failure_id** | `{fid}` |")
        lines.append(f"| **scenario** | `{scenario}` |")
        lines.append(f"| **step** | `{step}` |")
        lines.append(f"| **protocol** | {proto} |")
        lines.append(f"| **endpoint / RPC / topic** | `{endpoint}` |")
        lines.append("| **expected** | Harness step should pass (see scenario script). |")
        lines.append(f"| **actual** | {redact_message(msg)} |")
        lines.append(f"| **evidence file** | `{evidence}` |")
        lines.append(f"| **likely cause** | {cause} |")
        lines.append(f"| **suggested fix** | {fix} |")
        lines.append(
            f"| **--reuse-data safe** | **`{reuse}`** — use `yes`/`caution` when IDs/tokens are still valid; use `no` after collisions or unique-key errors (prefer `--fresh-data`). |"
        )
        lines.append(
            f"| **safe rerun** | `./tests/e2e/run-all-local.sh --reuse-data {rd}/test-data.json` |"
        )
        lines.append("")
        lines.append("---")
        lines.append("")

    (rep / "remediation.md").write_text("\n".join(lines), encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
