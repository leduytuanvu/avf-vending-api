#!/usr/bin/env python3
"""Merge E2E coverage artifacts (Postman, gRPC, MQTT, scenarios) and write coverage.json, e2e-junit.xml, test-data.redacted.json."""

from __future__ import annotations

import argparse
import json
import re
import time
from collections import Counter
from pathlib import Path
from typing import Any
from xml.sax.saxutils import escape as xml_esc


def read_jsonl(path: Path) -> list[dict[str, Any]]:
    if not path.is_file():
        return []
    out: list[dict[str, Any]] = []
    for line in path.read_text(encoding="utf-8-sig", errors="replace").splitlines():
        line = line.strip()
        if not line:
            continue
        try:
            out.append(json.loads(line))
        except json.JSONDecodeError:
            continue
    return out


def read_json(path: Path) -> Any | None:
    if not path.is_file():
        return None
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except json.JSONDecodeError:
        return None


SENSITIVE_KEY = re.compile(
    r"(token|secret|password|authorization|apikey|api_key|credential|jdbc|dsn)$",
    re.I,
)


def looks_like_jwt(s: str) -> bool:
    return bool(s) and s.startswith("eyJ") and s.count(".") >= 2


def redact_value(v: Any) -> Any:
    if isinstance(v, dict):
        return {k: "***" if SENSITIVE_KEY.search(k) else redact_value(val) for k, val in v.items()}
    if isinstance(v, list):
        return [redact_value(x) for x in v]
    if isinstance(v, str):
        if looks_like_jwt(v):
            return "***jwt***"
        if len(v) > 80 and re.fullmatch(r"[A-Za-z0-9+/=_-]+", v):
            return v[:4] + "***" + v[-4:]
    return v


def write_redacted_test_data(run_dir: Path) -> None:
    td = run_dir / "test-data.json"
    if not td.is_file():
        return
    try:
        raw = json.loads(td.read_text(encoding="utf-8"))
    except json.JSONDecodeError:
        return
    red = redact_value(raw)
    out = run_dir / "test-data.redacted.json"
    out.write_text(json.dumps(red, indent=2) + "\n", encoding="utf-8")


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--run-dir", type=Path, required=True)
    args = ap.parse_args()
    rd: Path = args.run_dir.resolve()
    rep = rd / "reports"
    rep.mkdir(parents=True, exist_ok=True)

    events = read_jsonl(rd / "events.jsonl")
    test_events = read_jsonl(rd / "test-events.jsonl")

    postman = read_json(rep / "coverage-postman.json")
    grpc = read_jsonl(rep / "grpc-contract-results.jsonl")
    mqtt = read_jsonl(rep / "mqtt-contract-results.jsonl")
    p8 = read_jsonl(rep / "phase8-scenario-results.jsonl")
    wa = read_jsonl(rep / "wa-module-results.jsonl")
    va = read_jsonl(rep / "va-rest-results.jsonl")

    counts = {
        "total": len(events),
        "passed": sum(1 for e in events if e.get("status") == "passed"),
        "failed": sum(1 for e in events if e.get("status") == "failed"),
        "skipped": sum(1 for e in events if e.get("status") == "skipped"),
    }

    step_prefixes: Counter[str] = Counter()
    for e in events:
        s = str(e.get("step", ""))
        m = re.match(r"^([a-z0-9-]+)", s, re.I)
        step_prefixes[m.group(1) if m else "unknown"] += 1

    p8_outcomes = Counter(str(r.get("result", "?")) for r in p8)
    scenario_coverage = {
        "e2eHarnessStepPrefixes": dict(step_prefixes),
        "phase8ScenarioCount": len(p8),
        "phase8ResultsByOutcome": dict(p8_outcomes),
    }

    merged = {
        "generatedAt": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "runDir": str(rd),
        "eventsFile": str(rd / "events.jsonl"),
        "testEventsFile": str(rd / "test-events.jsonl"),
        "counts": counts,
        "events": events,
        "testEvents": test_events,
        "coverage": {
            "postman": postman,
            "grpc": {"results": grpc, "count": len(grpc)},
            "mqtt": {"results": mqtt, "count": len(mqtt)},
            "phase8": {"results": p8, "count": len(p8)},
            "webAdminModules": {"results": wa, "count": len(wa)},
            "vendingAppRest": {"results": va, "count": len(va)},
            "scenarioCoverage": scenario_coverage,
        },
    }
    (rep / "coverage.json").write_text(json.dumps(merged, indent=2) + "\n", encoding="utf-8")

    write_redacted_test_data(rd)

    cases: list[str] = []
    for ev in events:
        step = str(ev.get("step", "unknown"))
        safe_name = re.sub(r"[^\w.-]+", "_", step)[:120] or "step"
        st = ev.get("status", "")
        msg = xml_esc(str(ev.get("message", "")))
        block = f'  <testcase classname="e2e" name="{xml_esc(safe_name)}" time="0">\n'
        if st == "failed":
            block += f'    <failure message="failed">{msg}</failure>\n'
        elif st == "skipped":
            block += "    <skipped/>\n"
        block += "  </testcase>\n"
        cases.append(block)
    ts = len(events)
    fc = sum(1 for e in events if e.get("status") == "failed")
    junit = (
        '<?xml version="1.0" encoding="UTF-8"?>\n'
        f'<testsuite name="e2e" tests="{ts}" failures="{fc}" errors="0" time="1.0">\n'
        + "".join(cases)
        + "</testsuite>\n"
    )
    (rep / "e2e-junit.xml").write_text(junit, encoding="utf-8")

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
