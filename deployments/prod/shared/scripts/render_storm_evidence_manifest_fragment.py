#!/usr/bin/env python3
"""Emit a JSON fragment for production-deployment-manifest.storm_evidence (stdout).

Reads workflow/env variables set by validate_production_scale_storm_evidence.py outputs
and the deploy job (STORM_GATE_*). Prints JSON null when evidence was not validated
(pilot, rollback, bypass, or missing snapshot fields).
"""
from __future__ import annotations

import json
import os
import sys


def _truthy(v: str | None) -> bool:
    if v is None:
        return False
    return v.strip().lower() in ("1", "true", "yes", "on")


def _int_field(name: str) -> int | None:
    raw = (os.environ.get(name) or "").strip()
    if not raw:
        return None
    try:
        return int(raw)
    except ValueError:
        return None


def main() -> int:
    required = _truthy(os.environ.get("STORM_GATE_REQUIRED"))
    bypassed = _truthy(os.environ.get("STORM_GATE_BYPASSED"))
    result = (os.environ.get("STORM_GATE_RESULT") or "").strip().lower()

    if not required or bypassed or result != "pass":
        print(json.dumps(None))
        return 0

    scenario = (os.environ.get("STORM_EVIDENCE_SCENARIO") or "").strip()
    completed = (os.environ.get("STORM_EVIDENCE_COMPLETED_AT_UTC") or "").strip()
    if not scenario or not completed:
        print(json.dumps(None))
        return 0

    mc = _int_field("STORM_EVIDENCE_MACHINE_COUNT")
    epm = _int_field("STORM_EVIDENCE_EVENTS_PER_MACHINE")
    if mc is None or epm is None:
        print(json.dumps(None))
        return 0

    max_age = _int_field("STORM_EVIDENCE_MAX_AGE_DAYS_APPLIED")

    doc = {
        "scenario": scenario,
        "completed_at_utc": completed,
        "machine_count": mc,
        "events_per_machine": epm,
        "final_result": (os.environ.get("STORM_EVIDENCE_FINAL_RESULT") or "").strip() or None,
        "critical_lost": _int_field("STORM_EVIDENCE_CRITICAL_LOST"),
        "duplicate_critical_effects": _int_field("STORM_EVIDENCE_DUPLICATE_CRITICAL_EFFECTS"),
        "db_pool_result": (os.environ.get("STORM_EVIDENCE_DB_POOL_RESULT") or "").strip() or None,
        "health_result": (os.environ.get("STORM_EVIDENCE_HEALTH_RESULT") or "").strip() or None,
        "restart_result": (os.environ.get("STORM_EVIDENCE_RESTART_RESULT") or "").strip() or None,
        "max_age_days_applied": max_age,
    }

    if doc["critical_lost"] is None or doc["duplicate_critical_effects"] is None:
        print(json.dumps(None))
        return 0

    print(json.dumps(doc, separators=(",", ":")))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
