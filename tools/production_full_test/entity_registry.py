#!/usr/bin/env python3
"""Production test entity registry."""

from __future__ import annotations

import json
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

from _common import report_dir, redact, write_json


class EntityRegistry:
    def __init__(self, path: Path | None = None) -> None:
        self.path = path or (report_dir() / "PRODUCTION_TEST_ENTITY_REGISTRY.json")
        self.data: dict[str, Any] = {
            "schema_version": "production-test-entity-registry-v1",
            "created_at_utc": datetime.now(timezone.utc).isoformat(),
            "prefix": "",
            "entities": {},
            "writes": [],
        }
        if self.path.is_file():
            self.data = json.loads(self.path.read_text(encoding="utf-8"))

    def set_prefix(self, prefix: str) -> None:
        self.data["prefix"] = prefix

    def set(self, key: str, value: str, *, entity_type: str = "id", meta: dict | None = None) -> None:
        self.data["entities"][key] = {"id": value, "type": entity_type, "meta": meta or {}}

    def get(self, key: str) -> str:
        ent = self.data["entities"].get(key) or {}
        return str(ent.get("id") or "")

    def as_substitution_map(self) -> dict[str, str]:
        out: dict[str, str] = {}
        for k, v in self.data.get("entities", {}).items():
            if isinstance(v, dict) and v.get("id"):
                out[k] = str(v["id"])
        return out

    def record_write(
        self,
        *,
        surface: str,
        action: str,
        request_id: str,
        correlation_id: str,
        entity_id: str,
        request_body: Any,
        response_body: Any,
        final_state: str = "created",
    ) -> None:
        self.data["writes"].append(
            {
                "surface": surface,
                "action": action,
                "request_id": request_id,
                "correlation_id": correlation_id,
                "entity_id": entity_id,
                "request_body": redact(json.dumps(request_body, default=str)[:4000]),
                "response_body": redact(str(response_body)[:4000]),
                "final_state": final_state,
                "at_utc": datetime.now(timezone.utc).isoformat(),
            }
        )

    def save(self) -> None:
        write_json(self.path, self.data)
