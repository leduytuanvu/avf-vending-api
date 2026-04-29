"""Docs/contracts for P2.1 reporting routes (offline)."""

import unittest
from pathlib import Path


class TestReportsDoc(unittest.TestCase):
    def test_fills_route_documented(self) -> None:
        root = Path(__file__).resolve().parents[1]
        text = (root / "docs" / "api" / "reports.md").read_text(encoding="utf-8")
        self.assertIn("/fills", text)
        self.assertIn("technician_fills", text)

    def test_swagger_export_lists_fills(self) -> None:
        root = Path(__file__).resolve().parents[1]
        text = (root / "internal" / "httpserver" / "swagger_operations.go").read_text(encoding="utf-8")
        self.assertIn("reports/fills", text)
        self.assertIn("| fills", text)


if __name__ == "__main__":
    unittest.main()
