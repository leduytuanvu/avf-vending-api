#!/usr/bin/env python3
"""Generate optimization-backlog.md from improvement-findings.jsonl."""

from __future__ import annotations

import argparse
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


def owner_for(f: dict[str, Any]) -> str:
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


def acceptance(f: dict[str, Any]) -> str:
    return (
        f"Resolve `{f.get('endpoint_or_rpc_or_topic')}` per recommendation; "
        f"add regression test or matrix row; verify no new P0 for same symptom."
    )


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--run-dir", type=Path, required=True)
    args = ap.parse_args()
    rd = args.run_dir.resolve()
    rep = rd / "reports"
    rep.mkdir(parents=True, exist_ok=True)

    findings = read_jsonl(rd / "improvement-findings.jsonl")
    buckets = {"P0": [], "P1": [], "P2": [], "P3": []}
    for f in findings:
        s = str(f.get("severity", "P3"))
        if s in buckets:
            buckets[s].append(f)

    lines = [
        "# Optimization backlog (from E2E flow review)",
        "",
        "Generated from **`improvement-findings.jsonl`**. Check items when addressed.",
        "",
    ]

    def section(title: str, sev: str) -> None:
        lines.append(f"## {title}")
        lines.append("")
        rows = buckets[sev]
        if not rows:
            lines.append("_None._\n")
            return
        for f in rows:
            own = owner_for(f)
            ev = str(f.get("evidence_file", "—"))
            lines.append(
                f"- [ ] **`{f.get('finding_id', '')}`** — flow `{f.get('flow_id', '')}` — "
                f"`{str(f.get('endpoint_or_rpc_or_topic', ''))[:72]}`\n"
                f"  - **Evidence:** `{ev}`\n"
                f"  - **Change:** {f.get('recommendation', '')}\n"
                f"  - **Acceptance:** {acceptance(f)}\n"
                f"  - **Owner:** {own}\n"
            )
        lines.append("")

    section("P0 — Fix now", "P0")
    section("P1 — Fix before pilot", "P1")
    section("P2 — Optimize next sprint", "P2")
    section("P3 — Cleanup", "P3")

    text = "\n".join(lines)
    (rd / "optimization-backlog.md").write_text(text, encoding="utf-8")
    (rep / "optimization-backlog.md").write_text(text, encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
