#!/usr/bin/env python3
"""Unit tests for canonical production E2E evidence failure counting."""
from __future__ import annotations

import sys
import tempfile
import unittest
from pathlib import Path

SCRIPTS = Path(__file__).resolve().parent
sys.path.insert(0, str(SCRIPTS))

from count_e2e_evidence_failures import count_failures  # noqa: E402

FIXTURES = Path(__file__).resolve().parent.parent / "fixtures"


class CountEvidenceFailuresTests(unittest.TestCase):
    def test_zero_failures_pass_table(self) -> None:
        md = FIXTURES / "result-md-pass.sample.md"
        self.assertEqual(count_failures(md), 0)

    def test_one_fail_in_flow_table(self) -> None:
        md = FIXTURES / "result-md-one-fail.sample.md"
        self.assertEqual(count_failures(md), 1)

    def test_optional_fail_counts_as_failure(self) -> None:
        with tempfile.NamedTemporaryFile("w", suffix=".md", delete=False, encoding="utf-8") as f:
            f.write(
                "| id | label | protocol | status | evidence |\n"
                "|----|-------|----------|--------|----------|\n"
                "| REST-X | probe | rest | optional-fail | `rest-x` |\n"
            )
            path = Path(f.name)
        try:
            self.assertEqual(count_failures(path), 1)
        finally:
            path.unlink(missing_ok=True)

    def test_executive_summary_fail_line_not_counted(self) -> None:
        with tempfile.NamedTemporaryFile("w", suffix=".md", delete=False, encoding="utf-8") as f:
            f.write(
                "| fail | **0** |\n"
                "| REST-OK | GET /health | rest | pass | `rest-ok` |\n"
            )
            path = Path(f.name)
        try:
            self.assertEqual(count_failures(path), 0)
        finally:
            path.unlink(missing_ok=True)

    def test_grpc_comm_fail_id_not_counted_when_pass(self) -> None:
        with tempfile.NamedTemporaryFile("w", suffix=".md", delete=False, encoding="utf-8") as f:
            f.write(
                "| GRPC-COMM-FAIL-001 | Vend failure path | grpc | pass | `grpc-vend-fail` |\n"
            )
            path = Path(f.name)
        try:
            self.assertEqual(count_failures(path), 0)
        finally:
            path.unlink(missing_ok=True)

    def test_missing_file_returns_zero(self) -> None:
        self.assertEqual(count_failures(Path("/nonexistent/RESULT.md")), 0)


if __name__ == "__main__":
    unittest.main()
