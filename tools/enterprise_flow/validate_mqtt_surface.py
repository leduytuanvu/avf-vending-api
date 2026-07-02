#!/usr/bin/env python3
"""Compare MQTT topics in code vs docs/api/mqtt-contract.md."""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parents[2]
TOPICS_GO = REPO / "internal" / "platform" / "mqtt" / "topics.go"
MQTT_DOC = REPO / "docs" / "api" / "mqtt-contract.md"
EXCEPTIONS = Path(__file__).resolve().parent / "accepted_surface_exceptions.json"


def extract_rel_paths_from_go() -> set[str]:
    text = TOPICS_GO.read_text(encoding="utf-8")
    paths: set[str] = set()
    for name in ["EnterpriseDevicePublishRelPaths", "LegacyDevicePublishRelPaths"]:
        m = re.search(rf"var {name} = \[\]string\{{([^}}]+)\}}", text, re.S)
        if m:
            for item in re.findall(r'"([^"]+)"', m.group(1)):
                paths.add(item)
    if m := re.search(r'EnterpriseSubscribeRelPath\s*=\s*"([^"]+)"', text):
        paths.add(m.group(1))
    return paths


def extract_rel_paths_from_doc() -> set[str]:
    text = MQTT_DOC.read_text(encoding="utf-8")
    paths: set[str] = set()
    for m in re.finditer(r"`([a-z][a-z0-9_/\-]+)`", text):
        p = m.group(1)
        if "/" in p and not p.startswith("avf/") and not p.startswith("MQTT"):
            paths.add(p)
    return paths


def main() -> int:
    code = extract_rel_paths_from_go()
    doc = extract_rel_paths_from_doc()
    exc = json.loads(EXCEPTIONS.read_text(encoding="utf-8"))
    allowed_code_only = set(exc.get("mqtt_code_only", []))
    allowed_docs_only = set(exc.get("mqtt_docs_only", []))

    missing_in_code = sorted(doc - code - allowed_docs_only)
    missing_in_docs = sorted(code - doc - allowed_code_only)

    report = {
        "enterprise_publish_count": len(code),
        "docs_topic_count": len(doc),
        "missing_in_code": missing_in_code,
        "missing_in_docs": missing_in_docs,
        "code_only_accepted": list(allowed_code_only),
    }

    out_dir = REPO / "reports" / "enterprise-flow"
    ts_dirs = sorted(out_dir.glob("*"), reverse=True)
    target = ts_dirs[0] if ts_dirs else out_dir
    (target / "MQTT_SURFACE_COVERAGE.json").write_text(json.dumps(report, indent=2), encoding="utf-8")
    (target / "MQTT_SURFACE_COVERAGE.md").write_text(
        f"# MQTT Surface Coverage\n\n- code paths: {len(code)}\n- missing_in_code: {len(missing_in_code)}\n- missing_in_docs: {len(missing_in_docs)}\n",
        encoding="utf-8",
    )

    # Allow doc fuzzy match — only fail on critical enterprise publish paths missing from docs
    critical = {"commands/ack", "commands/receipt", "presence", "state/heartbeat", "telemetry", "events/vend"}
    fail_docs = [p for p in critical if p in code and p not in doc and p not in allowed_code_only]
    if fail_docs:
        print("MQTT FAIL: critical paths missing from docs:", fail_docs, file=sys.stderr)
        return 1
    print("MQTT surface validation OK")
    return 0


if __name__ == "__main__":
    sys.exit(main())
