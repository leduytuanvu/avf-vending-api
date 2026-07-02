#!/usr/bin/env python3
"""MQTT topic inventory from docs and platform/mqtt code."""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path

from _inventory_common import write_inventory

REPO = Path(__file__).resolve().parents[2]
MQTT_DOC = REPO / "docs" / "api" / "mqtt-contract.md"
MQTT_PKG = REPO / "internal" / "platform" / "mqtt"
EXCEPTIONS = Path(__file__).resolve().parent / "accepted_surface_exceptions.json"


def topics_from_docs() -> set[str]:
    if not MQTT_DOC.is_file():
        return set()
    text = MQTT_DOC.read_text(encoding="utf-8", errors="replace")
    return set(re.findall(r"`([a-z0-9._/{}\-]+)`", text))


def topics_from_code() -> set[str]:
    topics: set[str] = set()
    for go in MQTT_PKG.rglob("*.go"):
        text = go.read_text(encoding="utf-8", errors="replace")
        topics.update(re.findall(r'"(?:[a-z]+\.)?[a-z0-9._/{}\-]+"', text))
        topics.update(re.findall(r"`([a-z0-9._/{}\-]+)`", text))
    return {t.strip('"') for t in topics if "/" in t or "." in t}


def main() -> int:
    exc = json.loads(EXCEPTIONS.read_text(encoding="utf-8"))
    docs = topics_from_docs()
    code = topics_from_code()
    all_topics = sorted(docs | code)
    items = [
        {
            "topic": t,
            "in_docs": t in docs,
            "in_code": t in code,
            "layout": "enterprise" if "devices/" in t else "legacy_or_shared",
        }
        for t in all_topics
    ]
    payload = {
        "topic_count": len(items),
        "docs_only": sorted(docs - code),
        "code_only": sorted(code - docs),
        "accepted_code_only": exc.get("mqtt_code_only", []),
        "topics": items,
    }
    md = [
        "# MQTT Inventory",
        "",
        f"- topic_count: **{payload['topic_count']}**",
        f"- docs_only: **{len(payload['docs_only'])}**",
        f"- code_only: **{len(payload['code_only'])}**",
        "",
    ]
    out = write_inventory("MQTT_INVENTORY", md, payload)
    print(f"MQTT inventory written to {out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
