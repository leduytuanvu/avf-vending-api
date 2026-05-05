#!/usr/bin/env python3
"""Generate optimization-backlog.md from improvement-findings.jsonl."""

from __future__ import annotations

import argparse
import json
from pathlib import Path
from typing import Any


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


def is_noise_finding(f: dict[str, Any]) -> bool:
    fid = str(f.get("finding_id", ""))
    if fid.startswith("E2E-NO-FINDINGS"):
        return True
    if str(f.get("flow_id", "")) == "_e2e_review_marker":
        return True
    return False


def title_for(f: dict[str, Any]) -> str:
    s = str(f.get("symptom", "")).strip()
    if not s:
        s = str(f.get("endpoint_or_rpc_or_topic", "")).strip()
    if len(s) > 100:
        s = s[:97] + "…"
    return s or "(no title)"


def suggested_owner_for(f: dict[str, Any]) -> str:
    o = str(f.get("suggested_owner", "")).strip()
    if o:
        return o
    c = str(f.get("category", "")).lower()
    if c in ("docs_gap", "postman_gap"):
        return "docs"
    if c in ("test_data_gap", "flaky", "flow_design", "cleanup", "unnecessary_complexity"):
        return "qa"
    if c in ("performance", "observability"):
        return "devops"
    if c in ("security", "production_safety"):
        return "backend"
    if "grpc" in str(f.get("protocol", "")).lower() or "mqtt" in str(f.get("protocol", "")).lower():
        return "backend"
    return "backend"


def acceptance_criteria(f: dict[str, Any]) -> str:
    return (
        f"Resolve `{f.get('endpoint_or_rpc_or_topic')}` per recommendation; "
        f"add regression test or matrix row; verify no new P0 for the same symptom."
    )


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--run-dir", type=Path, required=True)
    args = ap.parse_args()
    rd = args.run_dir.resolve()
    rep = rd / "reports"
    rep.mkdir(parents=True, exist_ok=True)

    findings = [f for f in read_jsonl(rd / "improvement-findings.jsonl") if not is_noise_finding(f)]
    buckets: dict[str, list[dict[str, Any]]] = {"P0": [], "P1": [], "P2": [], "P3": []}
    for f in findings:
        s = str(f.get("severity", "P3"))
        if s in buckets:
            buckets[s].append(f)

    lines: list[str] = ["# Optimization Backlog", ""]

    def section(heading: str, sev: str) -> None:
        lines.append(f"## {heading}")
        lines.append("")
        rows = buckets[sev]
        if not rows:
            lines.append("_None._")
            lines.append("")
            return
        for f in rows:
            fid = f.get("finding_id", "")
            ttl = title_for(f)
            own = suggested_owner_for(f)
            ev = str(f.get("evidence_file", "—"))
            lines.append(f"- [ ] **{fid}** — {ttl}")
            lines.append(f"  - Flow: `{f.get('flow_id', '')}`")
            lines.append(f"  - Endpoint/RPC/topic: `{f.get('endpoint_or_rpc_or_topic', '')}`")
            lines.append(f"  - Impact: {f.get('impact', '')}")
            lines.append(f"  - Evidence: `{ev}`")
            lines.append(f"  - Recommended change: {f.get('recommendation', '')}")
            lines.append(f"  - Acceptance criteria: {acceptance_criteria(f)}")
            lines.append(f"  - Suggested owner: **{own}**")
            lines.append("")
        lines.append("")

    section("P0 Fix Now", "P0")
    section("P1 Fix Before Pilot", "P1")
    section("P2 Optimize Next Sprint", "P2")
    section("P3 Cleanup", "P3")

    text = "\n".join(lines).rstrip() + "\n"
    (rd / "optimization-backlog.md").write_text(text, encoding="utf-8")
    (rep / "optimization-backlog.md").write_text(text, encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
