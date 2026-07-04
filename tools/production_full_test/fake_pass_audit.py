#!/usr/bin/env python3
"""Fake-pass audit for production full verification bundle."""

from __future__ import annotations

import json
import os
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

from _common import report_dir, write_json

FORBIDDEN_STATUSES = {"PASS_CONTRACT_ONLY", "SKIPPED", "UNTESTED"}


def audit_bundle(out: Path) -> dict:
    findings: list[dict] = []
    fake_pass_risk = False

    rest = out / "REST_FULL_TEST_MATRIX.json"
    if rest.is_file():
        data = json.loads(rest.read_text(encoding="utf-8"))
        for op in data.get("operations", []):
            if op.get("status") in FORBIDDEN_STATUSES and op.get("pass"):
                findings.append({"surface": "REST", "item": op.get("path"), "issue": "forbidden pass status"})
                fake_pass_risk = True
            if op.get("pass") and not op.get("evidence_path") and op.get("http_status") is None:
                findings.append({"surface": "REST", "item": op.get("path"), "issue": "pass without evidence"})
                fake_pass_risk = True
            ev = op.get("evidence_path")
            if ev and not (out / ev).is_file() and not Path(ev).is_file():
                findings.append({"surface": "REST", "item": op.get("path"), "issue": f"missing evidence {ev}"})
                fake_pass_risk = True

    for surface, fname in [("gRPC", "GRPC_FULL_TEST_MATRIX.json"), ("MQTT", "MQTT_FULL_TEST_MATRIX.json")]:
        p = out / fname
        if not p.is_file():
            findings.append({"surface": surface, "item": fname, "issue": "missing matrix"})
            fake_pass_risk = True
            continue
        data = json.loads(p.read_text(encoding="utf-8"))
        key = "operations" if surface == "gRPC" else "topics"
        for row in data.get(key, []):
            if row.get("pass") and row.get("status") == "UNTESTED":
                findings.append({"surface": surface, "item": str(row), "issue": "untested marked pass"})
                fake_pass_risk = True
            if surface == "MQTT" and row.get("pass") and row.get("connect_ok") is not True:
                tail = str(row.get("tail") or "")
                if not tail.startswith("negative/"):
                    findings.append({"surface": "MQTT", "item": row.get("topic"), "issue": "pass without connect_ok evidence"})
                    fake_pass_risk = True

    reg_path = out / "PRODUCTION_TEST_ENTITY_REGISTRY.json"
    if reg_path.is_file():
        reg = json.loads(reg_path.read_text(encoding="utf-8"))
        entities = reg.get("entities") or {}
        machine_token = str((entities.get("machineToken") or {}).get("id") or "")
        mqtt_password = str((entities.get("mqttPassword") or {}).get("id") or "")
        if machine_token and mqtt_password and machine_token == mqtt_password:
            findings.append({"surface": "bootstrap", "item": "mqttPassword", "issue": "machineToken used as mqttPassword"})
            fake_pass_risk = True

    stale = 0
    for name in out.glob("**/*.jsonl"):
        try:
            text = name.read_text(encoding="utf-8")
        except OSError:
            continue
        if "20260702T195405Z" in text and out.name != "20260702T195405Z":
            stale += 1
            findings.append({"surface": "evidence", "item": str(name.relative_to(out)), "issue": "stale evidence path from prior UTC bundle"})
            fake_pass_risk = True

    from pathlib import Path as _Path

    _mr = _Path(__file__).resolve().parents[1] / "market_readiness"
    if _mr.is_dir() and os.environ.get("MARKET_READINESS_STRICT", "").strip().lower() in ("1", "true", "yes"):
        bundle = _Path(os.environ.get("MARKET_READINESS_BUNDLE_DIR", "")) if os.environ.get("MARKET_READINESS_BUNDLE_DIR") else None
        sec = out / "SECURITY_AUTH_TEST_RESULTS.json"
        if sec.is_file():
            data = json.loads(sec.read_text(encoding="utf-8"))
            for rule in data.get("rules", []):
                if rule.get("actual") == "SKIPPED" and rule.get("pass"):
                    findings.append({"surface": "security", "item": rule.get("rule"), "issue": "skipped counted as pass in market strict mode"})
                    fake_pass_risk = True
        if bundle and bundle.is_dir():
            required = [
                "PRE_DESTRUCTIVE_BACKUP.json",
                "FINGERPRINT_REATTACH_PASS_1.json",
                "TECHNICIAN_RBAC_PASS_1.json",
                "FLEET_TIMELINE_PASS_1.json",
            ]
            if os.environ.get("PROD_DATABASE_URL", "").strip():
                required.insert(2, "DB_DIRECT_PASS_1.json")
            for req in required:
                if not (bundle / req).is_file():
                    findings.append({"surface": "market_bundle", "item": req, "issue": "missing market readiness artifact"})
                    fake_pass_risk = True

    return {
        "fakePassRisk": fake_pass_risk,
        "missingEvidenceCount": sum(1 for f in findings if "missing evidence" in f.get("issue", "")),
        "staleEvidenceCount": stale,
        "findings": findings,
    }


def main() -> int:
    out = report_dir()
    result = audit_bundle(out)
    write_json(out / "FAKE_PASS_AUDIT.json", result)
    (out / "FAKE_PASS_AUDIT.md").write_text(
        f"# Fake Pass Audit\n\nfakePassRisk={result['fakePassRisk']}\nfindings={len(result['findings'])}\n",
        encoding="utf-8",
    )
    return 1 if result["fakePassRisk"] else 0


if __name__ == "__main__":
    raise SystemExit(main())
