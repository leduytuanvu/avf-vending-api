#!/usr/bin/env python3
"""Lightweight checks for validate_production_scale_storm_evidence strict rules."""
from __future__ import annotations

import importlib.util
import sys
import unittest
from datetime import datetime, timedelta, timezone
from pathlib import Path

_scripts = Path(__file__).resolve().parent
_spec = importlib.util.spec_from_file_location(
    "validate_production_scale_storm_evidence",
    _scripts / "validate_production_scale_storm_evidence.py",
)
if _spec is None or _spec.loader is None:
    raise RuntimeError("cannot load validate_production_scale_storm_evidence")
_mod = importlib.util.module_from_spec(_spec)
sys.modules["validate_production_scale_storm_evidence"] = _mod
_spec.loader.exec_module(_mod)
validate_payload = _mod.validate_payload


def _fresh_ts() -> str:
    return (datetime.now(timezone.utc) - timedelta(hours=1)).strftime("%Y-%m-%dT%H:%M:%SZ")


def _base_good() -> dict:
    return {
        "dry_run": False,
        "execute_load_test": True,
        "scenario": "1000x500",
        "machine_count": 1000,
        "events_per_machine": 500,
        "completed_at_utc": _fresh_ts(),
        "final_result": "pass",
        "critical_lost": 0,
        "duplicate_critical_effects": 0,
        "db_pool_result": "pass",
        "health_result": "pass",
        "restart_result": "pass",
    }


class TestStormEvidence(unittest.TestCase):
    def test_scale1000_accepts_exact_preset(self) -> None:
        errs = validate_payload(_base_good(), min_m=1000, min_e=500, max_age_days=7)
        self.assertEqual(errs, [])

    def test_scale1000_rejects_weaker_load(self) -> None:
        p = _base_good()
        p["machine_count"] = 100
        p["events_per_machine"] = 100
        p["scenario"] = "100x100"
        errs = validate_payload(p, min_m=1000, min_e=500, max_age_days=7)
        self.assertTrue(any("1000 machines" in e for e in errs))

    def test_scale100_accepts_stronger_evidence(self) -> None:
        p = _base_good()
        errs = validate_payload(p, min_m=100, min_e=100, max_age_days=7)
        self.assertEqual(errs, [])

    def test_requires_completed_at_utc(self) -> None:
        p = _base_good()
        del p["completed_at_utc"]
        errs = validate_payload(p, min_m=1000, min_e=500, max_age_days=7)
        self.assertTrue(any("completed_at_utc" in e for e in errs))

    def test_rejects_stale(self) -> None:
        p = _base_good()
        old = datetime.now(timezone.utc) - timedelta(days=10)
        p["completed_at_utc"] = old.strftime("%Y-%m-%dT%H:%M:%SZ")
        errs = validate_payload(p, min_m=1000, min_e=500, max_age_days=7)
        self.assertTrue(any("older than" in e for e in errs))

    def test_critical_lost_nonzero_fails(self) -> None:
        p = _base_good()
        p["critical_lost"] = 1
        errs = validate_payload(p, min_m=1000, min_e=500, max_age_days=7)
        self.assertTrue(any("critical_lost" in e for e in errs))

    def test_duplicate_critical_effects_required(self) -> None:
        p = _base_good()
        del p["duplicate_critical_effects"]
        errs = validate_payload(p, min_m=1000, min_e=500, max_age_days=7)
        self.assertTrue(any("duplicate_critical_effects must be present" in e for e in errs))

    def test_db_pool_must_be_pass_not_ok(self) -> None:
        p = _base_good()
        p["db_pool_result"] = "ok"
        errs = validate_payload(p, min_m=1000, min_e=500, max_age_days=7)
        self.assertTrue(any("db_pool_result must be pass" in e for e in errs))


if __name__ == "__main__":
    unittest.main()
