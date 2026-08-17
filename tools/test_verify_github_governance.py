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

_FULL_MAIN: set[str] = {
    "CI / Workflow and Script Quality",
    "CI / GitHub repository governance",
    "Go CI Gates",
    "CI / Docker Compose Config Validation",
    "Secret Scan",
    "Go Vulnerability Scan",
    "Deployment and Config Scan",
    "verify-enterprise-release",
}

_FULL_DEVELOP: set[str] = {
    "Workflow and Script Quality",
    "GitHub repository governance",
    "Go CI Gates",
    "Docker Compose Config Validation",
    "Secret Scan",
    "Go Vulnerability Scan",
    "Deployment and Config Scan",
}


class TestRequiredStatusAliases(unittest.TestCase):
    def test_develop_policy_satisfied_by_short_ruleset_contexts(self) -> None:
        self.assertEqual(
            vg._missing_recommended_status_checks(
                vg.DEVELOP_RECOMMENDED_CONTEXTS, set(_FULL_DEVELOP)
            ),
            [],
        )

    def test_main_policy_satisfied_by_mixed_full_and_short_without_security_release(
        self,
    ) -> None:
        """main ruleset: Security Release Signal is not required; CI + security + enterprise are."""
        self.assertEqual(
            vg._missing_recommended_status_checks(
                vg.MAIN_RECOMMENDED_CONTEXTS, set(_FULL_MAIN)
            ),
            [],
        )
        # Explicit: omitting post-merge Security Release still passes main policy
        self.assertEqual(
            vg._missing_recommended_status_checks(
                vg.MAIN_RECOMMENDED_CONTEXTS,
                set(_FULL_MAIN) | {"Security Release Signal"},
            ),
            [],
        )

    def test_main_missing_enterprise_release_fails(self) -> None:
        ctx = set(_FULL_MAIN) - {"verify-enterprise-release"}
        missing = vg._missing_recommended_status_checks(vg.MAIN_RECOMMENDED_CONTEXTS, ctx)
        self.assertEqual(
            missing,
            ["Enterprise release verification / verify-enterprise-release"],
        )

    def test_develop_missing_ci_or_security_still_fails(self) -> None:
        almost = set(_FULL_DEVELOP) - {"Secret Scan"}
        missing = vg._missing_recommended_status_checks(
            vg.DEVELOP_RECOMMENDED_CONTEXTS, almost
        )
        self.assertEqual(missing, ["Security / Secret Scan"])

    def test_gh_stderr_http_status_and_free_plan_classic_protection(self) -> None:
        err = "gh: Upgrade to GitHub Pro or make this repository public to enable this feature. (HTTP 403)"
        self.assertEqual(vg._http_status_from_gh_stderr(1, err), 403)
        self.assertTrue(vg._is_github_free_classic_protection_unavailable(err))
        self.assertTrue(vg._is_github_permission_denied_error(err))

    def test_rulesets_unreadable_downgrades_missing_environment_reviewers(self) -> None:
        errors: list[str] = []
        warnings: list[str] = []
        # Simulate truncated environment payload the limited CI token returns.
        env = {
            "protection_rules": [{"type": "branch_policy"}],
            "deployment_branch_policy": {"protected_branches": False, "custom_branch_policies": True},
        }
        orig = vg.github_get_json

        def fake_get(path: str, token: str):
            if path.endswith("/environments/production"):
                return 0, env, ""
            if path.endswith("/deployment-branch-policies"):
                return 0, {"branch_policies": [{"name": "main"}]}, ""
            return orig(path, token)

        vg.github_get_json = fake_get  # type: ignore[method-assign]
        try:
            vg._check_environment_production(
                "o", "r", "tok", errors, warnings, rulesets_status="forbidden"
            )
        finally:
            vg.github_get_json = orig  # type: ignore[method-assign]
        self.assertEqual(errors, [])
        self.assertTrue(any("required_reviewers" in w for w in warnings))

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
