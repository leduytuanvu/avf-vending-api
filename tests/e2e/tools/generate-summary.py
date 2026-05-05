#!/usr/bin/env python3
"""Generate professional reports/summary.md for an E2E run directory."""

from __future__ import annotations

import argparse
import json
import re
from collections import Counter
from pathlib import Path
from typing import Any
from urllib.parse import urlparse


def redact_message(text: str) -> str:
    """Mask tokens/JWT fragments that sometimes appear in failure messages."""
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


def load_json(path: Path) -> Any | None:
    if not path.is_file():
        return None
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except json.JSONDecodeError:
        return None


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


def rest_table(run_dir: Path) -> str:
    rest = run_dir / "rest"
    if not rest.is_dir():
        return "_No `rest/` captures._\n"
    metas = sorted(rest.glob("*.meta.json"))
    if not metas:
        return "_No `rest/*.meta.json` files._\n"
    lines = [
        "| Step | Method | Path | HTTP | ms | Result | Evidence |",
        "|------|--------|------|------|-----|--------|----------|",
    ]
    for meta_path in metas:
        try:
            m = json.loads(meta_path.read_text(encoding="utf-8"))
        except json.JSONDecodeError:
            continue
        step = m.get("step", meta_path.stem.replace(".meta", ""))
        method = m.get("method", "—")
        path = m.get("path")
        if not path:
            reqp = rest / f"{step}.request.json"
            if reqp.is_file():
                try:
                    rq = json.loads(reqp.read_text(encoding="utf-8"))
                    path = rq.get("path")
                    if not path and rq.get("url"):
                        u = urlparse(str(rq["url"]))
                        path = u.path or "—"
                    if method == "—":
                        method = rq.get("method", "—")
                except json.JSONDecodeError:
                    path = "—"
            else:
                path = "—"
        code = m.get("httpStatus", "—")
        elapsed = m.get("elapsedMs", "—")
        result = m.get("result", "—")
        ev = f"`{step}.response.json`"
        lines.append(
            f"| `{step}` | {method} | `{path}` | {code} | {elapsed} | {result} | {ev} |"
        )
    return "\n".join(lines) + "\n"


def grpc_summary(rows: list[dict[str, Any]]) -> str:
    if not rows:
        return "_No `grpc-contract-results.jsonl` rows._\n"
    st = Counter(r.get("status", "?") for r in rows)
    lines = [f"- **Calls logged:** {len(rows)}", f"- **By status:** `{dict(st)}`", ""]
    lines.append("| flow_id | step | method | status | message |")
    lines.append("|---------|------|--------|--------|---------|")
    for r in rows[-40:]:
        lines.append(
            "| {flow} | {step} | {meth} | {st} | {msg} |".format(
                flow=r.get("flow_id", "—"),
                step=r.get("step", "—"),
                meth=r.get("method", "—"),
                st=r.get("status", "—"),
                msg=(
                    redact_message(str(r.get("message", "")))[:80] + "…"
                    if len(str(r.get("message", ""))) > 80
                    else redact_message(str(r.get("message", "")))
                ),
            )
        )
    if len(rows) > 40:
        lines.append(f"\n_… {len(rows) - 40} more rows in `reports/grpc-contract-results.jsonl`._\n")
    return "\n".join(lines) + "\n"


def mqtt_summary(rows: list[dict[str, Any]]) -> str:
    if not rows:
        return "_No `mqtt-contract-results.jsonl` rows._\n"
    lines = ["| flow_id | step | topic | status | message |", "|---------|------|-------|--------|---------|"]
    for r in rows[-30:]:
        lines.append(
            "| {f} | {s} | `{t}` | {st} | {m} |".format(
                f=r.get("flow_id", "—"),
                s=r.get("step", "—"),
                t=r.get("topic", "—"),
                st=r.get("status", "—"),
                m=redact_message(str(r.get("message", "")))[:60],
            )
        )
    return "\n".join(lines) + "\n"


def wa_summary(rows: list[dict[str, Any]]) -> str:
    if not rows:
        return "_No `wa-module-results.jsonl` (Web Admin Phase 4) rows._\n"
    st = Counter(r.get("status", "?") for r in rows)
    lines = [f"- **Rows:** {len(rows)}; **status counts:** `{dict(st)}`", ""]
    return "\n".join(lines) + "\n"


def va_summary(rows: list[dict[str, Any]]) -> str:
    if not rows:
        return "_No `va-rest-results.jsonl` rows._\n"
    st = Counter(r.get("status", "?") for r in rows)
    return f"- **Rows:** {len(rows)}; **status counts:** `{dict(st)}`\n\n"


def phase8_summary(rows: list[dict[str, Any]]) -> str:
    if not rows:
        return "_No Phase 8 `phase8-scenario-results.jsonl` (run scenarios 40–47 via `run-all-local.sh`)._\n"
    lines = [
        "| scenario_id | result | APIs / topics (excerpt) | evidence |",
        "|-------------|--------|---------------------------|----------|",
    ]
    for r in rows:
        apis = r.get("apis_topics_used") or []
        if isinstance(apis, list):
            apis_s = ", ".join(str(x) for x in apis[:5])
            if len(apis) > 5:
                apis_s += "…"
        else:
            apis_s = str(apis)[:80]
        ev = r.get("evidence_files") or []
        ev_s = "; ".join(str(x) for x in ev[:3]) if isinstance(ev, list) else str(ev)
        lines.append(
            "| {sid} | {res} | {apis} | `{ev}` |".format(
                sid=r.get("scenario_id", "—"),
                res=r.get("result", "—"),
                apis=apis_s or "—",
                ev=ev_s[:120] or "—",
            )
        )
    return "\n".join(lines) + "\n"


def postman_cov_summary(postman: dict[str, Any] | None) -> str:
    if not postman:
        return "_No `coverage-postman.json` — run `run-rest-local.sh` without `--readonly` after Newman._\n"
    return (
        f"- **total_requests:** {postman.get('total_requests', '—')}\n"
        f"- **covered_requests:** {postman.get('covered_requests', '—')}\n"
        f"- **uncovered_requests:** {postman.get('uncovered_requests', '—')}\n"
        f"- **excluded_requests:** {postman.get('excluded_requests', '—')}\n\n"
    )


def scenario_events_summary(events: list[dict[str, Any]]) -> str:
    """High-level step labels from events.jsonl."""
    pref: Counter[str] = Counter()
    for e in events:
        s = str(e.get("step", ""))
        m = re.match(r"^([a-z-]+)-", s, re.I)
        key = m.group(1) if m else s.split("-")[0] if s else "unknown"
        pref[key] += 1
    if not pref:
        return "_No step events._\n"
    lines = [f"- **Steps by prefix:** `{dict(pref)}`", ""]
    return "\n".join(lines)


def failures_block(failed: list[dict[str, Any]], run_dir: Path) -> str:
    if not failed:
        return "_None._\n"
    lines: list[str] = []
    for e in failed:
        step = e.get("step", "")
        msg = str(e.get("message", ""))
        lines.append(f"- **`{step}`:** {redact_message(msg)}")
        lines.append(f"  - See **`{run_dir}/reports/remediation.md`** for structured hints and **evidence** paths.")
    lines.append("")
    return "\n".join(lines)


def skips_block(skipped: list[dict[str, Any]]) -> str:
    if not skipped:
        return "_None._\n"
    return "\n".join(f"- `{e.get('step')}`: {e.get('message', '')}" for e in skipped) + "\n"


def next_actions(failed: list[dict[str, Any]], run_dir: Path) -> str:
    acts = [
        f"1. Open **`{run_dir}/reports/remediation.md`** for failure_id rows, evidence paths, and rerun commands.",
        f"2. Inspect **`{run_dir}/events.jsonl`** and per-protocol folders: `rest/`, `grpc/`, `mqtt/`.",
        "3. Fix environment (API up, migrations, tokens) per `docs/testing/e2e-remediation-playbook.md`.",
    ]
    if failed:
        steps = {str(e.get("step", "")) for e in failed}
        if any("grpc" in s.lower() for s in steps):
            acts.append("4. **gRPC:** verify `GRPC_ADDR`, `grpcurl`, and proto root; check `grpc/*.log`.")
        if any("mqtt" in s.lower() or "newman" in s.lower() for s in steps):
            acts.append("5. **MQTT / Newman:** verify broker, `mosquitto_*` clients, and `rest/newman-cli.log`.")
        if any("web-admin" in s.lower() for s in steps):
            acts.append("6. **Web admin:** confirm `ADMIN_TOKEN`, roles, and `reports/wa-module-results.jsonl`.")
    acts.append(
        f"7. Rerun: `./tests/e2e/run-all-local.sh --reuse-data {run_dir}/test-data.json` when scratch IDs are still valid."
    )
    return "\n".join(acts) + "\n"


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--run-dir", type=Path, required=True)
    ap.add_argument("--repo-root", type=Path, required=True)
    args = ap.parse_args()
    rd = args.run_dir.resolve()
    rep = rd / "reports"
    rep.mkdir(parents=True, exist_ok=True)

    run_meta = load_json(rd / "run-meta.json") or {}
    ctx = load_json(rep / "e2e-report-context.json") or {}
    cov = load_json(rep / "coverage.json") or {}
    events = cov.get("events") or read_jsonl(rd / "events.jsonl")
    failed = [e for e in events if e.get("status") == "failed"]
    skipped = [e for e in events if e.get("status") == "skipped"]

    red_path = rd / "test-data.redacted.json"
    test_data_red = load_json(red_path) if red_path.is_file() else {}
    cts = cov.get("counts") or {}

    postman = (cov.get("coverage") or {}).get("postman")
    grpc_rows = (cov.get("coverage") or {}).get("grpc", {}).get("results") or read_jsonl(rep / "grpc-contract-results.jsonl")
    mqtt_rows = (cov.get("coverage") or {}).get("mqtt", {}).get("results") or read_jsonl(rep / "mqtt-contract-results.jsonl")
    p8_rows = (cov.get("coverage") or {}).get("phase8", {}).get("results") or read_jsonl(rep / "phase8-scenario-results.jsonl")
    wa_rows = (cov.get("coverage") or {}).get("webAdminModules", {}).get("results") or read_jsonl(rep / "wa-module-results.jsonl")
    va_rows = (cov.get("coverage") or {}).get("vendingAppRest", {}).get("results") or read_jsonl(rep / "va-rest-results.jsonl")

    target = run_meta.get("e2eTarget", "local")
    generated_at = str(cov.get("generatedAt") or run_meta.get("startedAt") or "—")
    allow = str(ctx.get("allowWrites", "")).lower()
    safety = (
        f"- **E2E_TARGET:** `{target}`\n"
        f"- **E2E_ALLOW_WRITES (snapshot):** `{allow}`\n"
        f"- **Production confirmation present:** `{ctx.get('productionConfirmationSet', False)}`\n"
        f"- **Reuse mode:** `{ctx.get('reuseData', 'false')}` — source: `{ctx.get('reuseDataSource') or '—'}`\n\n"
    )

    keys_created = [k for k in sorted(test_data_red.keys()) if k not in ("e2eTestMachine",)]
    resources = ", ".join(f"`{k}`" for k in keys_created[:25])
    if len(keys_created) > 25:
        resources += f" … (+{len(keys_created) - 25} more)"

    tdump = json.dumps(test_data_red, indent=2, default=str)
    if len(tdump) > 12000:
        tdump = tdump[:12000] + "\n…"

    scen_cov = (cov.get("coverage") or {}).get("scenarioCoverage")
    scen_md = (
        f"```json\n{json.dumps(scen_cov, indent=2)}\n```\n\n"
        if scen_cov
        else "_Not present — ensure `merge-events.py` ran._\n\n"
    )

    md = f"""# E2E run summary

## Run metadata

- **Generated:** `{generated_at}` (UTC, from merged coverage when available)
- **Run directory:** `{rd}`
- **Runner:** `{run_meta.get("runner", "—")}`
- **Started:** `{run_meta.get("startedAt", "—")}`
- **Repository:** `{run_meta.get("repoRoot", args.repo_root)}`
- **PID:** `{run_meta.get("pid", "—")}`

## Environment

- **BASE_URL (snapshot):** `{ctx.get("baseUrl", "—")}`
- **GRPC_ADDR (snapshot):** `{ctx.get("grpcAddr", "—")}`
- **MQTT broker (snapshot):** `{ctx.get("mqttBroker", "—")}`
- **Postman:** collection/env paths are in shell env when Newman ran; see `rest/newman-cli.log`.

Full snapshot: `reports/e2e-report-context.json` (written at finalize; no secrets).

## Report artifacts (index)

Canonical paths are under **`reports/`** (this file); JSONL logs and redacted test data sit at the **run root** (`..` relative to here).

| Artifact | Path |
|----------|------|
| Step events (pass/fail/skip) | [`../events.jsonl`](../events.jsonl) |
| Scenario / module test events | [`../test-events.jsonl`](../test-events.jsonl) |
| Flow improvement findings (JSON Lines) | [`../improvement-findings.jsonl`](../improvement-findings.jsonl) |
| This summary | [`summary.md`](summary.md) |
| Remediation (hard failures + related improvement links) | [`remediation.md`](remediation.md) |
| Improvement rollup | [`improvement-summary.md`](improvement-summary.md) |
| Optimization backlog | [`optimization-backlog.md`](optimization-backlog.md) |
| Per-flow scorecard | [`flow-review-scorecard.json`](flow-review-scorecard.json) |
| Merged coverage | [`coverage.json`](coverage.json) |
| Public test data (redacted) | [`../test-data.redacted.json`](../test-data.redacted.json) |
| JUnit (when produced) | [`e2e-junit.xml`](e2e-junit.xml) |

Mirrors at run root (when present): `improvement-summary.md`, `optimization-backlog.md`, `flow-review-scorecard.json`, `summary.md`, `remediation.md`.

## Target safety mode

{safety}

## Test data used

Public/redacted view: **`test-data.redacted.json`** (same keys as `test-data.json` with tokens masked).

```json
{tdump}
```

## Created resources (captured IDs)

Key set in test-data: {resources or "—"}

See `test-data.json` locally (gitignored under `.e2e-runs/`) for full values when debugging.

## REST summary

Count of HTTP captures: **{len(list((rd / "rest").glob('*.meta.json'))) if (rd / "rest").is_dir() else 0}** meta files.

{rest_table(rd)}

Newman: `rest/newman-report.json`, `rest/newman-junit.xml`, `rest/newman-cli.log`.

## gRPC summary

{grpc_summary(grpc_rows)}

Evidence: `grpc/*.request.json`, `grpc/*.response.json`, `grpc/*.log`, `grpc/*.meta.json`.

## MQTT summary

{mqtt_summary(mqtt_rows)}

Evidence: `mqtt/*.publish.json`, `mqtt/*.log`, `reports/mqtt-contract-results.jsonl`.

## Web admin flow summary

{wa_summary(wa_rows)}

Detail: `reports/wa-module-results.jsonl`.

## Vending app flow summary

{va_summary(va_rows)}

Detail: `reports/va-rest-results.jsonl`.

## E2E scenario summary

{scenario_events_summary(events)}

### Phase 8 (40–47)

{phase8_summary(p8_rows)}

Raw: `reports/phase8-scenario-results.jsonl`.

## Failures

{failures_block(failed, rd)}

## Skips

{skips_block(skipped)}

## Coverage

Merged machine-readable: **`reports/coverage.json`**.

Step counts: **total** {cts.get("total", "—")}, **passed** {cts.get("passed", "—")}, **failed** {cts.get("failed", "—")}, **skipped** {cts.get("skipped", "—")}.

### Postman / API matrix coverage

{postman_cov_summary(postman if isinstance(postman, dict) else None)}

### Scenario coverage (merged)

{scen_md}

CI JUnit aggregation: **`reports/e2e-junit.xml`**.

## Next actions

{next_actions(failed, rd)}

---
*Playbook (relative to this file in `reports/`):* [`docs/testing/e2e-remediation-playbook.md`](../../../docs/testing/e2e-remediation-playbook.md)
"""
    (rep / "summary.md").write_text(md, encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
