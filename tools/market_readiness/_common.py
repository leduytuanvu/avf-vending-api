#!/usr/bin/env python3
"""Shared helpers for market readiness production verification."""

from __future__ import annotations

import json
import os
import sys
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
PROD_TEST = ROOT / "tools" / "production_full_test"
sys.path.insert(0, str(PROD_TEST))

from _common import (  # noqa: E402
    REPO,
    http_request,
    is_market_readiness_strict,
    market_report_dir,
    market_ready_prefix,
    new_request_id,
    production_machine_code,
    redact,
    utc_stamp,
    write_json,
)
from entity_registry import EntityRegistry  # noqa: E402

BACKUP_PATH = "deployments/prod/backups/backup-20260703T230257Z.dump"

FINGERPRINT_FIELD_KEYS = [
    ("androidId", "android_id"),
    ("androidSerial", "android_serial"),
    ("boardSerial", "board_serial"),
    ("serialNumber", "device_serial"),
    ("simSerial", "sim_serial"),
    ("simIccid", "sim_iccid"),
    ("simOperator", "sim_operator"),
    ("simCountryIso", "sim_country_iso"),
    ("manufacturer", None),
    ("brand", None),
    ("model", None),
    ("deviceModel", "device_model"),
    ("hardware", None),
    ("product", None),
    ("androidRelease", "android_release"),
    ("sdkInt", None),
    ("packageName", "package_name"),
    ("versionName", "version_name"),
    ("versionCode", None),
    ("appBuildSha", "app_build_sha"),
    ("bootId", "boot_id"),
    ("networkType", "network_type"),
    ("networkState", "network_state"),
]

TIMELINE_EVENT_TYPES = [
    "device.attachment.attached",
    "device.attachment.replaced",
    "runtime.app_session.started",
    "runtime.app_session.ended",
    "operator.session.started",
    "operator.session.ended",
    "machine.lifecycle.changed",
    "machine.sale_enabled.changed",
]

ATTACHMENT_SQL_COLUMNS = [
    "android_id",
    "android_serial",
    "board_serial",
    "device_serial",
    "sim_serial",
    "sim_iccid",
    "sim_operator",
    "sim_country_iso",
    "manufacturer",
    "brand",
    "model",
    "device_model",
    "hardware",
    "product",
    "android_release",
    "sdk_int",
    "package_name",
    "version_name",
    "version_code",
    "app_build_sha",
    "boot_id",
    "network_type",
    "network_state",
    "previous_attachment_id",
    "status",
    "reason",
]


def bundle_dir() -> Path:
    explicit = os.environ.get("MARKET_READINESS_BUNDLE_DIR", "").strip()
    if explicit:
        p = Path(explicit)
        p.mkdir(parents=True, exist_ok=True)
        return p
    return market_report_dir()


def setup_market_env(prefix: str = "") -> None:
    os.environ.setdefault("PRODUCTION_SUITE", "market_readiness")
    os.environ.setdefault("MARKET_READINESS_STRICT", "1")
    os.environ.setdefault("PRODUCTION_FULL_TEST_STRICT", "1")
    if not os.environ.get("PRODUCTION_FULL_TEST_UTC"):
        os.environ["PRODUCTION_FULL_TEST_UTC"] = utc_stamp()
    if prefix:
        os.environ["PRODUCTION_TEST_PREFIX"] = prefix
    elif not os.environ.get("PRODUCTION_TEST_PREFIX"):
        os.environ["PRODUCTION_TEST_PREFIX"] = market_ready_prefix()


def record_pre_destructive_backup(out: Path) -> None:
    backup = REPO / BACKUP_PATH
    payload = {
        "recorded_at_utc": datetime.now(timezone.utc).isoformat(),
        "backup_path": str(backup) if backup.is_file() else BACKUP_PATH,
        "backup_exists_locally": backup.is_file(),
        "deploy_run": "28686916171",
        "deploy_sha": "277a3ad4dbe34f204704ed4c3d713ec49bff4ec2",
        "note": "Operator confirmed inline backup before runtime-fleet destructive CRUD",
    }
    write_json(out / "PRE_DESTRUCTIVE_BACKUP.json", payload)


def admin_headers(token: str) -> dict[str, str]:
    return {
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json",
        "Accept": "application/json",
        "X-Request-ID": new_request_id(),
    }


def build_full_fingerprint(prefix: str, variant: str) -> dict:
    """Build fingerprint payload exercising DeviceIdentityFromFingerprint keys."""
    base = f"{prefix}-{variant}"
    fp: dict = {
        "androidId": f"{base}-aid",
        "androidSerial": f"{base}-aserial",
        "boardSerial": f"{base}-board",
        "serialNumber": f"{base}-serial",
        "simSerial": f"{base}-simserial",
        "simIccid": f"89{base[-15:].replace('-', '0')[:18]}",
        "simOperator": "VF",
        "simCountryIso": "VN",
        "manufacturer": "AVF",
        "brand": "AVFBoard",
        "model": "TCN-TEST",
        "deviceModel": "AVF-VEND-PRO",
        "hardware": "qcom",
        "product": "avf_vending",
        "androidRelease": "13",
        "sdkInt": 33,
        "packageName": "dev.avf.vending.prodtest",
        "versionName": "1.0.0-market",
        "versionCode": 10001,
        "appBuildSha": "51485f55" + "0" * 32,
        "bootId": f"{base}-boot",
        "networkType": "wifi",
        "networkState": "connected",
    }
    if variant.endswith("_snake"):
        snake = {}
        for camel, snake_key in FINGERPRINT_FIELD_KEYS:
            val = fp.get(camel)
            if val is None:
                continue
            snake[snake_key or camel] = val
        return snake
    return fp


def write_matrix_result(out: Path, name: str, rows: list[dict], *, pass_count: int, fail_count: int) -> None:
    payload = {
        "matrix": name,
        "pass_count": pass_count,
        "fail_count": fail_count,
        "rows": rows,
        "at_utc": datetime.now(timezone.utc).isoformat(),
    }
    write_json(out / f"{name}.json", payload)
    md = f"# {name}\n\npass={pass_count} fail={fail_count}\n\n"
    md += "\n".join(f"- {r.get('case', r.get('name', '?'))}: {'PASS' if r.get('pass') else 'FAIL'}" for r in rows)
    md += "\n"
    (out / f"{name}.md").write_text(md, encoding="utf-8")
