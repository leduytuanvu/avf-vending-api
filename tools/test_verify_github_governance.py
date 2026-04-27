"""Unit tests for required status check alias resolution (no GitHub API)."""

from __future__ import annotations

import sys
import unittest
from pathlib import Path

# Import verifier from the same directory when run as `python -m unittest` from repo root.
_TOOLS = Path(__file__).resolve().parent
if str(_TOOLS) not in sys.path:
    sys.path.insert(0, str(_TOOLS))

import verify_github_governance as vg  # noqa: E402


class TestRequiredStatusAliases(unittest.TestCase):
    def test_develop_policy_satisfied_by_short_ruleset_contexts(self) -> None:
        api_ctx = {
            "Workflow and Script Quality",
            "GitHub repository governance",
            "Go CI Gates",
            "Docker Compose Config Validation",
            "Secret Scan",
            "Go Vulnerability Scan",
            "Deployment and Config Scan",
        }
        self.assertEqual(
            vg._missing_recommended_status_checks(vg.DEVELOP_RECOMMENDED_CONTEXTS, api_ctx),
            [],
        )

    def test_main_policy_satisfied_by_mixed_full_and_short(self) -> None:
        api_ctx = {
            "CI / Workflow and Script Quality",
            "CI / GitHub repository governance",
            "Go CI Gates",
            "CI / Docker Compose Config Validation",
            "Secret Scan",
            "Go Vulnerability Scan",
            "Deployment and Config Scan",
            "verify-enterprise-release",
            "Security Release Signal",
        }
        self.assertEqual(
            vg._missing_recommended_status_checks(vg.MAIN_RECOMMENDED_CONTEXTS, api_ctx),
            [],
        )

    def test_missing_reports_canonical_not_alias(self) -> None:
        api_ctx: set[str] = set()
        m = vg._missing_recommended_status_checks(
            ("CI / Go CI Gates", "Security / Secret Scan"), api_ctx
        )
        self.assertEqual(
            m,
            ["CI / Go CI Gates", "Security / Secret Scan"],
        )


if __name__ == "__main__":
    unittest.main()
