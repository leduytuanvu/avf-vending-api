#!/usr/bin/env python3
"""Validate telemetry-storm-result.json for production fleet-scale rollout gate.

Reads configuration from environment (CI or local). Writes GitHub Actions outputs when
GITHUB_OUTPUT is set.

Scale targets require evidence that meets **minimum** load thresholds:
  - scale-100:   machine_count >= 100 and events_per_machine >= 100
  - scale-500:   machine_count >= 500 and events_per_machine >= 200
  - scale-1000:  machine_count >= 1000 and events_per_machine >= 500

Strict gate (non-bypass) requires canonical fields including completed_at_utc,
final_result=pass, result dimensions, and duplicate_critical_effects present.
"""
from __future__ import annotations

import json
import os
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple


def _truthy(v: str | None) -> bool:
    if v is None:
        return False
    return v.strip().lower() in ("1", "true", "yes", "on")


def _emit_output(key: str, value: str) -> None:
    path = os.environ.get("GITHUB_OUTPUT")
    if not path:
        return
    with open(path, "a", encoding="utf-8") as fh:
        if "\n" in value:
            fh.write(f"{key}<<__STORM_GATE_EOF__\n")
            fh.write(value)
            fh.write("\n__STORM_GATE_EOF__\n")
        else:
            fh.write(f"{key}={value}\n")


def _emit_empty_evidence_snapshot(max_age_days: int) -> None:
    _emit_output("storm_evidence_scenario", "")
    _emit_output("storm_evidence_completed_at_utc", "")
    _emit_output("storm_evidence_machine_count", "")
    _emit_output("storm_evidence_events_per_machine", "")
    _emit_output("storm_evidence_final_result", "")
    _emit_output("storm_evidence_critical_lost", "")
    _emit_output("storm_evidence_duplicate_critical_effects", "")
    _emit_output("storm_evidence_db_pool_result", "")
    _emit_output("storm_evidence_health_result", "")
    _emit_output("storm_evidence_restart_result", "")
    _emit_output("storm_evidence_max_age_days_applied", str(max_age_days))


def _emit_evidence_snapshot(payload: Dict[str, Any], max_age_days: int) -> None:
    _emit_output("storm_evidence_scenario", str(payload.get("scenario") or "").strip())
    _emit_output("storm_evidence_completed_at_utc", str(payload.get("completed_at_utc") or "").strip())
    _emit_output("storm_evidence_machine_count", str(payload.get("machine_count", "")))
    _emit_output("storm_evidence_events_per_machine", str(payload.get("events_per_machine", "")))
    _emit_output("storm_evidence_final_result", str(payload.get("final_result") or "").strip())
    _emit_output("storm_evidence_critical_lost", str(payload.get("critical_lost", "")))
    _emit_output("storm_evidence_duplicate_critical_effects", str(payload.get("duplicate_critical_effects", "")))
    _emit_output("storm_evidence_db_pool_result", str(payload.get("db_pool_result") or "").strip())
    _emit_output("storm_evidence_health_result", str(payload.get("health_result") or "").strip())
    _emit_output("storm_evidence_restart_result", str(payload.get("restart_result") or "").strip())
    _emit_output("storm_evidence_max_age_days_applied", str(max_age_days))


def _strict_pass_label(v: Any) -> bool:
    if isinstance(v, str) and v.strip().lower() == "pass":
        return True
    return False


def _num_zero(v: Any) -> bool:
    if v is None:
        return False
    if isinstance(v, bool):
        return v is False
    try:
        return abs(float(v)) < 1e-9
    except (TypeError, ValueError):
        return False


def _min_machines_events(fleet: str) -> Optional[Tuple[int, int]]:
    return {
        "scale-100": (100, 100),
        "scale-500": (500, 200),
        "scale-1000": (1000, 500),
    }.get(fleet.strip())


def _scenario_meets_threshold(payload: Dict[str, Any], min_m: int, min_e: int) -> List[str]:
    errs: List[str] = []
    try:
        m = int(payload["machine_count"])
        e = int(payload["events_per_machine"])
    except (KeyError, TypeError, ValueError):
        return [
            "machine_count and events_per_machine must be integers "
            f"(got machine_count={payload.get('machine_count')!r}, events_per_machine={payload.get('events_per_machine')!r})"
        ]
    if m < min_m or e < min_e:
        errs.append(
            f"evidence must meet at least {min_m} machines × {min_e} events per machine "
            f"(got machine_count={m}, events_per_machine={e})"
        )
    return errs


def _parse_utc(ts: str) -> datetime:
    s = ts.strip()
    if s.endswith("Z"):
        s = s[:-1] + "+00:00"
    dt = datetime.fromisoformat(s)
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=timezone.utc)
    return dt.astimezone(timezone.utc)


def validate_payload(
    payload: Dict[str, Any],
    *,
    min_m: int,
    min_e: int,
    max_age_days: int,
) -> List[str]:
    errs: List[str] = []

    dr = payload.get("dry_run")
    if dr is True or _truthy(str(dr or "")):
        errs.append("dry_run must be false in scale evidence")
    ex = payload.get("execute_load_test")
    if ex is not True and not _truthy(str(ex or "")):
        errs.append("execute_load_test must be true in scale evidence")

    errs.extend(_scenario_meets_threshold(payload, min_m, min_e))

    scen = str(payload.get("scenario") or "").strip()
    if not scen:
        errs.append("scenario must be a non-empty string")

    ts = payload.get("completed_at_utc")
    if not isinstance(ts, str) or not ts.strip():
        errs.append("completed_at_utc is required (non-empty ISO-8601 UTC string)")
    else:
        ts = ts.strip()
        try:
            then = _parse_utc(ts)
            age = datetime.now(timezone.utc) - then
            if age.total_seconds() < 0:
                errs.append("storm evidence completed_at_utc is in the future")
            elif age.total_seconds() > max_age_days * 86400:
                errs.append(
                    f"storm evidence is older than {max_age_days} days "
                    f"(completed_at_utc={ts!r}, age_seconds={int(age.total_seconds())})"
                )
        except ValueError as ex:
            errs.append(f"invalid completed_at_utc timestamp: {ex}")

    fr = payload.get("final_result")
    if not _strict_pass_label(fr):
        errs.append(f"final_result must be pass (got {fr!r})")

    for ck in ("critical_expected", "critical_accepted"):
        if ck not in payload:
            errs.append(f"{ck} is required in scale storm evidence JSON")

    if not _num_zero(payload.get("critical_lost")):
        errs.append(f"critical_lost must be 0 (got {payload.get('critical_lost')!r})")

    if "duplicate_critical_effects" not in payload:
        errs.append("duplicate_critical_effects must be present and 0")
    else:
        dc = payload.get("duplicate_critical_effects")
        if not _num_zero(dc):
            errs.append(f"duplicate_critical_effects must be 0 (got {dc!r})")

    for key in ("db_pool_result", "health_result", "restart_result"):
        if not _strict_pass_label(payload.get(key)):
            errs.append(f"{key} must be pass (got {payload.get(key)!r})")

    wh = payload.get("worker_health_result")
    if wh is not None and str(wh).strip():
        wls = str(wh).lower()
        if wls not in ("skipped", "pass", "unknown"):
            errs.append(f"worker_health_result must be pass, skipped, or unknown (got {wh!r})")

    return errs


def main() -> int:
    action_mode = (os.environ.get("ACTION_MODE") or "deploy").strip().lower()
    fleet = (os.environ.get("FLEET_SCALE_TARGET") or "pilot").strip().lower()
    bypass = _truthy(os.environ.get("ALLOW_SCALE_GATE_BYPASS"))
    bypass_reason = (os.environ.get("SCALE_GATE_BYPASS_REASON") or os.environ.get("BYPASS_REASON") or "").strip()
    max_age_raw = (os.environ.get("STORM_EVIDENCE_MAX_AGE_DAYS") or "7").strip()
    evidence_file = (os.environ.get("TELEMETRY_STORM_EVIDENCE_FILE") or "").strip()

    try:
        max_age_days = int(max_age_raw)
        if max_age_days < 1:
            raise ValueError
    except ValueError:
        print(
            "validate_production_scale_storm_evidence: error: STORM_EVIDENCE_MAX_AGE_DAYS must be a positive int",
            file=sys.stderr,
        )
        return 1

    _emit_output("fleet_scale_target", fleet)

    if action_mode == "rollback":
        _emit_output("storm_gate_required", "false")
        _emit_output("storm_gate_result", "not-required")
        _emit_output("storm_gate_evidence_path", "")
        _emit_output("storm_gate_bypassed", "false")
        _emit_output("storm_gate_bypass_reason", "")
        _emit_empty_evidence_snapshot(max_age_days)
        print("validate_production_scale_storm_evidence: rollback - storm gate not required")
        return 0

    if fleet == "pilot":
        _emit_output("storm_gate_required", "false")
        _emit_output("storm_gate_result", "not-required")
        _emit_output("storm_gate_evidence_path", evidence_file)
        _emit_output("storm_gate_bypassed", "false")
        _emit_output("storm_gate_bypass_reason", "")
        _emit_empty_evidence_snapshot(max_age_days)
        print("validate_production_scale_storm_evidence: pilot - storm gate not required")
        return 0

    me = _min_machines_events(fleet)
    if not me:
        print(f"validate_production_scale_storm_evidence: error: unknown FLEET_SCALE_TARGET={fleet!r}", file=sys.stderr)
        return 1

    min_m, min_e = me
    _emit_output("storm_gate_required", "true")

    if bypass:
        if not bypass_reason:
            print(
                "validate_production_scale_storm_evidence: error: ALLOW_SCALE_GATE_BYPASS=true requires "
                "non-empty SCALE_GATE_BYPASS_REASON or BYPASS_REASON",
                file=sys.stderr,
            )
            return 1
        _emit_output("storm_gate_result", "bypassed")
        _emit_output("storm_gate_evidence_path", evidence_file)
        _emit_output("storm_gate_bypassed", "true")
        _emit_output("storm_gate_bypass_reason", bypass_reason)
        _emit_empty_evidence_snapshot(max_age_days)
        print("validate_production_scale_storm_evidence: WARNING: fleet scale storm gate BYPASSED (operator override).")
        print(f"validate_production_scale_storm_evidence: bypass reason: {bypass_reason}")
        return 0

    _emit_output("storm_gate_bypassed", "false")
    _emit_output("storm_gate_bypass_reason", "")

    if not evidence_file:
        print(
            "validate_production_scale_storm_evidence: error: scale deploy requires storm evidence "
            "(set TELEMETRY_STORM_EVIDENCE_FILE via repo path or downloaded artifact)",
            file=sys.stderr,
        )
        return 1

    path = Path(evidence_file)
    if not path.is_file():
        print(f"validate_production_scale_storm_evidence: error: evidence file not found: {path}", file=sys.stderr)
        return 1

    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as ex:
        print(f"validate_production_scale_storm_evidence: error: cannot read JSON: {ex}", file=sys.stderr)
        return 1

    if not isinstance(payload, dict):
        print("validate_production_scale_storm_evidence: error: evidence root must be a JSON object", file=sys.stderr)
        return 1

    errs = validate_payload(payload, min_m=min_m, min_e=min_e, max_age_days=max_age_days)
    if errs:
        for e in errs:
            print(f"validate_production_scale_storm_evidence: error: {e}", file=sys.stderr)
        _emit_output("storm_gate_result", "fail")
        _emit_output("storm_gate_evidence_path", str(path.resolve()))
        _emit_empty_evidence_snapshot(max_age_days)
        return 1

    _emit_output("storm_gate_result", "pass")
    _emit_output("storm_gate_evidence_path", str(path.resolve()))
    _emit_evidence_snapshot(payload, max_age_days)
    print(f"validate_production_scale_storm_evidence: pass ({path})")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
