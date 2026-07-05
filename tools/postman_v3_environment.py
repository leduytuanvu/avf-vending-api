#!/usr/bin/env python3
"""Deterministic v2.1 Postman environment JSON -> v3 *.environment.yaml exporter."""
from __future__ import annotations

import json
import re
from pathlib import Path
from typing import Any

try:
    import yaml
except ImportError:  # pragma: no cover
    yaml = None  # type: ignore[assignment]

SECRET_KEY_PARTS = (
    "password",
    "secret",
    "token",
    "api_key",
    "apikey",
    "webhook",
    "mqttpassword",
    "bearerey",
)

FORBIDDEN_ENV_KEY_PARTS = (
    "organization_id",
    "organizationid",
    "tenant_id",
    "tenantid",
    "scope_id",
    "scopeid",
)


def _is_secret_key(key: str) -> bool:
    kl = key.lower().replace("-", "_")
    if kl in ("adminpassword", "admin_password", "accesstoken", "refreshtoken", "machinetoken", "mtqqpassword", "mqtt_password"):
        return True
    return any(part in kl for part in SECRET_KEY_PARTS)


def _forbidden_key(key: str) -> bool:
    kl = key.lower().replace("-", "_")
    return any(bad in kl for bad in FORBIDDEN_ENV_KEY_PARTS)


def load_environment_json(path: Path) -> dict[str, Any]:
    return json.loads(path.read_text(encoding="utf-8"))


def export_environment_yaml(json_path: Path, yaml_path: Path, *, force_blank_secrets: bool = True) -> dict[str, Any]:
    if yaml is None:
        raise RuntimeError("PyYAML required (pip install pyyaml)")

    data = load_environment_json(json_path)
    name = str(data.get("name") or json_path.stem.replace(".postman_environment", ""))
    values_out: list[dict[str, Any]] = []

    for entry in data.get("values") or []:
        if not isinstance(entry, dict):
            continue
        key = str(entry.get("key") or "")
        if not key:
            continue
        if _forbidden_key(key):
            raise ValueError(f"forbidden environment key {key!r} in {json_path}")

        val = entry.get("value", "")
        if force_blank_secrets and _is_secret_key(key):
            val = ""
        elif val is None:
            val = ""
        else:
            val = str(val)

        out: dict[str, Any] = {
            "key": key,
            "value": val,
            "enabled": bool(entry.get("enabled", True)),
        }
        if entry.get("type"):
            out["type"] = entry["type"]
        values_out.append(out)

    payload = {"name": name, "values": values_out}
    yaml_path.parent.mkdir(parents=True, exist_ok=True)
    yaml_path.write_text(
        yaml.safe_dump(payload, sort_keys=False, allow_unicode=True, default_flow_style=False),
        encoding="utf-8",
        newline="\n",
    )
    return payload


def validate_no_secret_patterns(text: str, ctx: str) -> None:
    banned = (
        "DATABASE_URL=",
        "SUPABASE_",
        "JWT_SECRET=",
        "WEBHOOK_SECRET=",
        "Bearer eyJ",
    )
    for b in banned:
        if b in text:
            raise ValueError(f"forbidden pattern {b!r} in {ctx}")
    if re.search(r"(sk|pk)_(live|test)_[A-Za-z0-9]{20,}", text):
        raise ValueError(f"stripe-like key in {ctx}")
