#!/usr/bin/env python3
"""Emit flow-review-scorecard.json — one row per (flow_id, scenario_id) with scores and finding_counts."""

from __future__ import annotations

import argparse
import json
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


def is_noise_finding(f: dict[str, Any]) -> bool:
    fid = str(f.get("finding_id", ""))
    if fid.startswith("E2E-NO-FINDINGS"):
        return True
    if str(f.get("flow_id", "")) == "_e2e_review_marker":
        return True
    return False


def scenario_from_event(e: dict[str, Any]) -> str:
    ri = e.get("resource_ids")
    if isinstance(ri, str):
        try:
            ri = json.loads(ri)
        except json.JSONDecodeError:
            ri = {}
    if not isinstance(ri, dict):
        ri = {}
    sid = str(ri.get("scenario_id") or "").strip()
    return sid if sid else "unknown"


def event_status_for_row(events: list[dict[str, Any]], flow_id: str, scenario_id: str) -> str:
    st_worst = "pass"
    for e in events:
        if str(e.get("flow_id", "")) != flow_id:
            continue
        if scenario_id != "unknown":
            if scenario_from_event(e) != scenario_id:
                continue
        st = str(e.get("status", "")).lower()
        if st in ("fail", "failed"):
            return "fail"
        if st in ("skip", "skipped"):
            st_worst = "skip"
    return st_worst


def score_dims(findings_for_row: list[dict[str, Any]]) -> dict[str, int]:
    dims = {k: 100 for k in ("correctness_score", "automation_score", "production_safety_score", "observability_score")}
    for f in findings_for_row:
        sev = str(f.get("severity", "P3"))
        pen = SEV_PEN.get(sev, 3)
        for k in dims:
            dims[k] = max(0, dims[k] - pen)
    return dims


def finding_counts(findings_for_row: list[dict[str, Any]]) -> dict[str, int]:
    counts = {"P0": 0, "P1": 0, "P2": 0, "P3": 0}
    for f in findings_for_row:
        s = str(f.get("severity", "P3"))
        if s in counts:
            counts[s] += 1
    return counts


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--run-dir", type=Path, required=True)
    ap.add_argument("--repo-root", type=Path, default=None)
    args = ap.parse_args()
    rd = args.run_dir.resolve()
    rep = rd / "reports"
    rep.mkdir(parents=True, exist_ok=True)

    findings = [f for f in read_jsonl(rd / "improvement-findings.jsonl") if not is_noise_finding(f)]
    events = read_jsonl(rd / "test-events.jsonl")

    keys: set[tuple[str, str]] = set()
    for f in findings:
        fid = str(f.get("flow_id", "")).strip()
        if not fid:
            continue
        sid = str(f.get("scenario_id", "unknown")).strip() or "unknown"
        keys.add((fid, sid))
    for e in events:
        fid = str(e.get("flow_id", "")).strip()
        if not fid:
            continue
        keys.add((fid, scenario_from_event(e)))

    rows: list[dict[str, Any]] = []
    for flow_id, scenario_id in sorted(keys):
        ff = [
            f
            for f in findings
            if str(f.get("flow_id", "")) == flow_id
            and (str(f.get("scenario_id", "unknown")).strip() or "unknown") == scenario_id
        ]

        dims = score_dims(ff)
        counts = finding_counts(ff)
        rows.append(
            {
                "flow_id": flow_id,
                "scenario_id": scenario_id,
                "status": event_status_for_row(events, flow_id, scenario_id),
                "correctness_score": dims["correctness_score"],
                "automation_score": dims["automation_score"],
                "production_safety_score": dims["production_safety_score"],
                "observability_score": dims["observability_score"],
                "finding_counts": counts,
            }
        )

    text = json.dumps(rows, indent=2) + "\n"
    (rd / "flow-review-scorecard.json").write_text(text, encoding="utf-8")
    (rep / "flow-review-scorecard.json").write_text(text, encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
