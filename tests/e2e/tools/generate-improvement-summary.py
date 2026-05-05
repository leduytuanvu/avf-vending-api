#!/usr/bin/env python3
"""Generate improvement-summary.md and patch reports/summary.md (Flow Improvement Findings)."""

from __future__ import annotations

import argparse
import json
import os
from collections import defaultdict
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


def load_json(path: Path) -> Any | None:
    if not path.is_file():
        return None
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except json.JSONDecodeError:
        return None


def is_noise_finding(f: dict[str, Any]) -> bool:
    fid = str(f.get("finding_id", ""))
    if fid.startswith("E2E-NO-FINDINGS"):
        return True
    if str(f.get("flow_id", "")) == "_e2e_review_marker":
        return True
    return False


def owner_from_category(category: str) -> str:
    c = category.lower()
    if c in ("api_contract", "response_shape", "missing_field", "missing_endpoint", "idempotency", "retry_semantics", "offline_sync"):
        return "backend"
    if c in ("protocol_mismatch", "rest_grpc_mismatch", "mqtt_contract"):
        return "backend"
    if c in ("docs_gap", "postman_gap"):
        return "docs"
    if c in ("test_data_gap", "flow_design", "unnecessary_complexity", "flaky", "performance", "cleanup"):
        return "qa"
    if c in ("production_safety", "security"):
        return "backend"
    if c == "observability":
        return "devops"
    return "backend"


def suggested_audience(f: dict[str, Any]) -> str:
    o = str(f.get("suggested_owner", "")).strip().lower()
    if o in ("backend", "android", "admin-web", "qa", "docs", "devops", "unknown"):
        return o
    cat = str(f.get("category", ""))
    if cat.lower() == "offline_sync":
        return "android"
    ep = str(f.get("endpoint_or_rpc_or_topic", "")).lower()
    if "/v1/admin" in ep or "/admin/" in ep:
        return "admin-web"
    return owner_from_category(cat)


def md_table(title: str, rows: list[dict[str, Any]]) -> str:
    lines = [f"## {title}", ""]
    if not rows:
        lines.append("_None._\n")
        return "\n".join(lines)
    lines.append("| ID | Severity | Flow | Step | Endpoint | Symptom |")
    lines.append("|----|----------|------|------|----------|---------|")
    for f in rows:
        lines.append(
            f"| `{f.get('finding_id', '')}` | {f.get('severity', '')} | {f.get('flow_id', '')} | `{f.get('step_name', '')}` | `{str(f.get('endpoint_or_rpc_or_topic', ''))[:48]}` | {str(f.get('symptom', ''))[:60]} |"
        )
    lines.append("")
    return "\n".join(lines)


def hint_lines(findings: list[dict[str, Any]], audiences: set[str]) -> list[str]:
    out: list[str] = []
    for f in findings:
        if suggested_audience(f) in audiences:
            out.append(
                f"- **{f.get('finding_id')}** (`{f.get('severity')}`): `{f.get('endpoint_or_rpc_or_topic')}` — {str(f.get('recommendation', ''))[:220]}"
            )
    return out


def cursor_prompts(findings: list[dict[str, Any]]) -> list[str]:
    prompts: list[str] = []
    for f in findings[:12]:
        prompts.append(
            f"Review API/design for `{f.get('endpoint_or_rpc_or_topic')}` ({f.get('category')} / {f.get('severity')}): "
            f"{str(f.get('symptom', ''))[:120]} — propose fix: {str(f.get('recommendation', ''))[:120]}"
        )
    return prompts


def env_truthy(name: str, default: str = "true") -> bool:
    v = os.environ.get(name, default).strip().lower()
    return v in ("1", "true", "yes", "on")


def patch_main_summary(summ: Path, n0: int, n1: int, n2: int, n3: int) -> None:
    if not summ.is_file():
        return
    body = summ.read_text(encoding="utf-8")

    p0_gate = env_truthy("E2E_FAIL_ON_P0_FINDINGS", "true")
    warn_line = ""
    if n0 > 0 and p0_gate:
        warn_line = "\n> **Technical tests may have passed, but P0 flow findings require fixes.**\n"

    block = f"""## Flow Improvement Findings
{warn_line}
- P0: **{n0}**
- P1: **{n1}**
- P2: **{n2}**
- P3: **{n3}**
- improvement-findings.jsonl: [`improvement-findings.jsonl`](../improvement-findings.jsonl)
- improvement-summary.md: [`improvement-summary.md`](improvement-summary.md)
- optimization-backlog.md: [`optimization-backlog.md`](optimization-backlog.md)
- flow-review-scorecard.json: [`flow-review-scorecard.json`](flow-review-scorecard.json)
"""

    markers = ("## Flow Improvement Findings", "## Flow improvement review")
    start = -1
    used = ""
    for m in markers:
        idx = body.find(m)
        if idx != -1 and (start == -1 or idx < start):
            start = idx
            used = m
    if start != -1:
        after = body[start + len(used) :]
        nl = after.find("\n## ")
        if nl == -1:
            body = body[:start] + block.rstrip() + "\n"
        else:
            body = body[:start] + block.rstrip() + after[nl:]
    else:
        body = body.rstrip() + "\n\n" + block.rstrip() + "\n"

    summ.write_text(body, encoding="utf-8")


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--run-dir", type=Path, required=True)
    ap.add_argument("--repo-root", type=Path, required=True)
    args = ap.parse_args()
    rd = args.run_dir.resolve()
    rep = rd / "reports"
    rep.mkdir(parents=True, exist_ok=True)

    all_findings = read_jsonl(rd / "improvement-findings.jsonl")
    findings = [f for f in all_findings if not is_noise_finding(f)]
    events = read_jsonl(rd / "test-events.jsonl")
    cov = load_json(rep / "coverage.json") or load_json(rd / "coverage.json")
    red = load_json(rd / "test-data.redacted.json")

    by_sev: dict[str, list[dict[str, Any]]] = defaultdict(list)
    by_flow: dict[str, list[dict[str, Any]]] = defaultdict(list)
    by_proto: dict[str, list[dict[str, Any]]] = defaultdict(list)
    by_cat: dict[str, list[dict[str, Any]]] = defaultdict(list)
    for f in findings:
        by_sev[str(f.get("severity", "?"))].append(f)
        by_flow[str(f.get("flow_id", "unknown"))].append(f)
        by_proto[str(f.get("protocol", "?"))].append(f)
        by_cat[str(f.get("category", "unknown"))].append(f)

    n0, n1, n2, n3 = len(by_sev["P0"]), len(by_sev["P1"]), len(by_sev["P2"]), len(by_sev["P3"])
    ntot = len(findings)

    cov_note = "_Coverage JSON not present (run merge-events.py for full merge)._"
    if isinstance(cov, dict):
        cts = cov.get("counts") or {}
        cov_note = (
            f"Merged **coverage.json** present: step counts total={cts.get('total', '—')}, "
            f"passed={cts.get('passed', '—')}, failed={cts.get('failed', '—')}, skipped={cts.get('skipped', '—')}."
        )
    red_note = (
        "**test-data.redacted.json** present (masked capture snapshot)."
        if isinstance(red, dict)
        else "_test-data.redacted.json not present this run._"
    )
    ev_note = f"**test-events.jsonl:** {len(events)} row(s) for scenario-level context."

    exec_sum = [
        f"Recorded **{ntot}** improvement finding(s) (noise markers excluded): **P0={n0}**, **P1={n1}**, **P2={n2}**, **P3={n3}**.",
        "",
        "These rows are **not necessarily test failures**; they track API/flow/design debt discovered during the run.",
        "",
        f"- {cov_note}",
        f"- {red_note}",
        f"- {ev_note}",
        "",
    ]
    if n0:
        exec_sum.append("**P0** items indicate risk to money, inventory, or reliable vending — address before production rollout.")
        exec_sum.append("")
    if ntot == 0:
        opt_note = "no"
    elif n0 > 0:
        opt_note = "no (P0 present)"
    else:
        opt_note = "yes"
    exec_sum.append(f"- **Run technically passed but needs optimization:** **{opt_note}** (step failures are tracked separately in remediation.md).")
    exec_sum.append("")

    sev_counts = "\n".join(
        [
            "## Findings count by severity",
            "",
            "| Severity | Count |",
            "|----------|-------|",
            f"| P0 | {n0} |",
            f"| P1 | {n1} |",
            f"| P2 | {n2} |",
            f"| P3 | {n3} |",
            "",
        ]
    )

    parts = [
        "# E2E flow improvement summary",
        "",
        "## Executive summary",
        "",
        *exec_sum,
        sev_counts,
        md_table("P0 findings", by_sev["P0"]),
        md_table("P1 findings", by_sev["P1"]),
        md_table("P2 findings", by_sev["P2"]),
        md_table("P3 findings", by_sev["P3"]),
        "## Findings by flow",
        "",
    ]
    for fid in sorted(by_flow.keys()):
        parts.append(f"### `{fid}`")
        parts.append("")
        for f in by_flow[fid]:
            parts.append(f"- **{f.get('finding_id')}** ({f.get('severity')}): {f.get('symptom')} — _{f.get('recommendation')}_")
        parts.append("")

    parts.append("## Findings by protocol")
    parts.append("")
    for p in sorted(by_proto.keys()):
        parts.append(f"- **{p}:** {len(by_proto[p])}")
    parts.append("")
    parts.append("## Findings by category")
    parts.append("")
    for c in sorted(by_cat.keys()):
        parts.append(f"- **{c}:** {len(by_cat[c])}")
    parts.append("")

    parts.append("## Backend/API changes likely needed")
    parts.append("")
    parts.extend(hint_lines(findings, {"backend"}) or ["_None flagged for this audience._"])
    parts.append("")
    parts.append("## Android app changes likely needed")
    parts.append("")
    parts.extend(hint_lines(findings, {"android"}) or ["_None flagged for this audience._"])
    parts.append("")
    parts.append("## Admin Web changes likely needed")
    parts.append("")
    parts.extend(hint_lines(findings, {"admin-web"}) or ["_None flagged for this audience._"])
    parts.append("")
    parts.append("## QA/test-data changes likely needed")
    parts.append("")
    parts.extend(hint_lines(findings, {"qa", "devops"}) or ["_None flagged for this audience._"])
    parts.append("")
    parts.append("## Docs/Postman changes likely needed")
    parts.append("")
    parts.extend(hint_lines(findings, {"docs"}) or ["_None flagged for this audience._"])
    parts.append("")
    unknown_only = [f for f in findings if suggested_audience(f) == "unknown"]
    if unknown_only:
        parts.append("## Owner unknown (triage)")
        parts.append("")
        for f in unknown_only[:30]:
            parts.append(f"- **{f.get('finding_id')}:** {f.get('symptom', '')[:200]}")
        parts.append("")
    parts.append("## Suggested next Cursor prompts")
    parts.append("")
    for p in cursor_prompts(findings):
        parts.append(f"- {p}")
    parts.append("")

    out_md = "\n".join(parts)
    (rd / "improvement-summary.md").write_text(out_md, encoding="utf-8")
    (rep / "improvement-summary.md").write_text(out_md, encoding="utf-8")

    patch_main_summary(rep / "summary.md", n0, n1, n2, n3)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
