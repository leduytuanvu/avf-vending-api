#!/usr/bin/env python3
"""Generate improvement-summary.md and flow-review-scorecard.json from improvement-findings.jsonl."""

from __future__ import annotations

import argparse
import json
from collections import defaultdict
from pathlib import Path
from typing import Any

SEV_PEN = {"P0": 50, "P1": 25, "P2": 10, "P3": 3}


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


def score_dims(findings_for_flow: list[dict[str, Any]]) -> dict[str, int]:
    dims = {k: 100 for k in ("correctness_score", "automation_score", "production_safety_score", "observability_score")}
    for f in findings_for_flow:
        sev = str(f.get("severity", "P3"))
        pen = SEV_PEN.get(sev, 3)
        for k in dims:
            dims[k] = max(0, dims[k] - pen)
    return dims


def flow_event_status(events: list[dict[str, Any]], flow_id: str) -> str:
    st_worst = "pass"
    for e in events:
        if str(e.get("flow_id", "")) != flow_id:
            continue
        st = str(e.get("status", "")).lower()
        if st in ("fail", "failed"):
            return "fail"
        if st in ("skip", "skipped"):
            st_worst = "skip"
    return st_worst


def owner_hint(category: str) -> str:
    c = category.lower()
    if c in ("api_contract", "response_shape", "request_shape", "missing_field", "missing_endpoint", "idempotency", "retry_semantics", "offline_sync"):
        return "backend"
    if c in ("protocol_mismatch", "rest_grpc_mismatch", "mqtt_contract"):
        return "backend"
    if c in ("docs_gap", "postman_gap"):
        return "docs"
    if c in ("test_data_gap", "flow_design", "unnecessary_complexity", "flaky", "performance"):
        return "qa"
    if c in ("production_safety", "security"):
        return "backend"
    if c == "observability":
        return "devops"
    if c == "cleanup":
        return "qa"
    return "backend"


def backend_hints(findings: list[dict[str, Any]]) -> list[str]:
    out: list[str] = []
    for f in findings:
        if f.get("severity") in ("P0", "P1") and owner_hint(str(f.get("category", ""))) == "backend":
            out.append(
                f"- **{f.get('finding_id')}:** `{f.get('endpoint_or_rpc_or_topic')}` — {str(f.get('recommendation', ''))[:200]}"
            )
    return out[:25]


def doc_hints(findings: list[dict[str, Any]]) -> list[str]:
    out: list[str] = []
    for f in findings:
        if str(f.get("category")) in ("docs_gap", "postman_gap"):
            out.append(f"- **{f.get('finding_id')}:** {f.get('symptom', '')[:160]}")
    return out[:20]


def cursor_prompts(findings: list[dict[str, Any]]) -> list[str]:
    prompts: list[str] = []
    for f in findings[:12]:
        prompts.append(
            f"Review API/design for `{f.get('endpoint_or_rpc_or_topic')}` ({f.get('category')} / {f.get('severity')}): "
            f"{f.get('symptom', '')[:120]} — propose fix: {f.get('recommendation', '')[:120]}"
        )
    return prompts


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--run-dir", type=Path, required=True)
    ap.add_argument("--repo-root", type=Path, required=True)
    args = ap.parse_args()
    rd = args.run_dir.resolve()
    rep = rd / "reports"
    rep.mkdir(parents=True, exist_ok=True)

    findings = read_jsonl(rd / "improvement-findings.jsonl")
    events = read_jsonl(rd / "test-events.jsonl")
    _ = load_json(rep / "coverage.json")
    _ = load_json(rd / "test-data.redacted.json")

    by_sev = defaultdict(list)
    by_flow: dict[str, list[dict[str, Any]]] = defaultdict(list)
    by_proto = defaultdict(list)
    by_cat = defaultdict(list)
    by_scenario: dict[str, list[dict[str, Any]]] = defaultdict(list)
    for f in findings:
        by_sev[str(f.get("severity", "?"))].append(f)
        by_flow[str(f.get("flow_id", "unknown"))].append(f)
        by_proto[str(f.get("protocol", "?"))].append(f)
        by_cat[str(f.get("category", "unknown"))].append(f)
        by_scenario[str(f.get("scenario_id", "unknown"))].append(f)

    flow_ids = set(by_flow.keys())
    for e in events:
        fid = str(e.get("flow_id", ""))
        if fid:
            flow_ids.add(fid)

    scorecard: list[dict[str, Any]] = []
    for fid in sorted(flow_ids):
        ff = by_flow.get(fid, [])
        counts = {k: 0 for k in ("P0", "P1", "P2", "P3")}
        for f in ff:
            s = str(f.get("severity", "P3"))
            if s in counts:
                counts[s] += 1
        dims = score_dims(ff)
        scorecard.append(
            {
                "flow_id": fid,
                "status": flow_event_status(events, fid),
                "correctness_score": dims["correctness_score"],
                "automation_score": dims["automation_score"],
                "production_safety_score": dims["production_safety_score"],
                "observability_score": dims["observability_score"],
                "findings": counts,
            }
        )

    (rd / "flow-review-scorecard.json").write_text(json.dumps(scorecard, indent=2) + "\n", encoding="utf-8")
    (rep / "flow-review-scorecard.json").write_text(json.dumps(scorecard, indent=2) + "\n", encoding="utf-8")

    n0, n1, n2, n3 = len(by_sev["P0"]), len(by_sev["P1"]), len(by_sev["P2"]), len(by_sev["P3"])
    ntot = len(findings)
    exec_sum = [
        f"Recorded **{ntot}** improvement finding(s): **P0={n0}**, **P1={n1}**, **P2={n2}**, **P3={n3}**.",
        "",
        "These are **not necessarily test failures**; they track API/flow/design debt discovered during the run.",
        "",
    ]
    if n0:
        exec_sum.append("**P0 items** indicate risk to money, inventory, or reliable vending — address before production rollout.")
        exec_sum.append("")
    if ntot == 0:
        opt_note = "no"
    elif n0 > 0:
        opt_note = "no (P0 present)"
    else:
        opt_note = "yes"
    exec_sum.append(
        f"- **Run technically passed but needs optimization:** **{opt_note}** (non-zero step failures are separate; see remediation.md)"
    )

    def md_table(title: str, rows: list[dict[str, Any]]) -> str:
        lines = [f"## {title}", ""]
        if not rows:
            lines.append("_None._\n")
            return "\n".join(lines)
        lines.append("| ID | Severity | Flow | Step | Endpoint | Symptom |")
        lines.append("|----|----------|------|------|----------|---------|")
        for f in rows:
            lines.append(
                f"| `{f.get('finding_id','')}` | {f.get('severity','')} | {f.get('flow_id','')} | `{f.get('step_name','')}` | `{str(f.get('endpoint_or_rpc_or_topic',''))[:48]}` | {str(f.get('symptom',''))[:60]} |"
            )
        lines.append("")
        return "\n".join(lines)

    parts = [
        "# E2E flow improvement summary",
        "",
        "## Executive summary",
        "",
        *exec_sum,
        "",
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

    parts.append("## Findings by scenario (script)")
    parts.append("")
    for sid in sorted(by_scenario.keys()):
        parts.append(f"### `{sid}`")
        parts.append("")
        for f in by_scenario[sid]:
            parts.append(
                f"- **{f.get('finding_id')}** ({f.get('severity')} / {f.get('category')}): {f.get('symptom')}"
            )
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
    parts.append("## API / RPC / topic improvement list")
    parts.append("")
    for f in findings[:40]:
        parts.append(
            f"- `{f.get('protocol')}` `{f.get('endpoint_or_rpc_or_topic')}` — **{f.get('finding_id')}** ({f.get('severity')}): {f.get('symptom', '')[:200]}"
        )
    if len(findings) > 40:
        parts.append(f"\n_… {len(findings) - 40} more in `improvement-findings.jsonl`._\n")

    parts.append("## Suspected backend changes needed")
    parts.append("")
    parts.extend(backend_hints(findings) or ["_None flagged._"])
    parts.append("")
    parts.append("## Suspected docs / Postman changes needed")
    parts.append("")
    parts.extend(doc_hints(findings) or ["_None flagged._"])
    parts.append("")
    parts.append("## Suggested next Cursor prompts")
    parts.append("")
    for p in cursor_prompts(findings):
        parts.append(f"- {p}")
    parts.append("")

    out_md = "\n".join(parts)
    (rd / "improvement-summary.md").write_text(out_md, encoding="utf-8")
    (rep / "improvement-summary.md").write_text(out_md, encoding="utf-8")

    # Patch main summary.md
    summ = rep / "summary.md"
    if summ.is_file():
        body = summ.read_text(encoding="utf-8")
        if "## Flow improvement review" not in body:
            block = f"""

## Flow improvement review

- **Findings file:** `improvement-findings.jsonl` — **total {ntot}** (P0={n0}, P1={n1}, P2={n2}, P3={n3})
- **Technically passed but needs optimization:** **{opt_note}** (treat **no** when there are no findings or any **P0** row exists)
- **Detail:** [`improvement-summary.md`](../improvement-summary.md) (copy also under `reports/`)
- **Backlog:** [`optimization-backlog.md`](../optimization-backlog.md)
- **Scorecard:** [`flow-review-scorecard.json`](../flow-review-scorecard.json)
"""
            summ.write_text(body.rstrip() + block + "\n", encoding="utf-8")

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
