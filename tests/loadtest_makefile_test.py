"""Anchors for P1.5 fleet load harness Makefile targets (offline)."""

import unittest
from pathlib import Path


class TestLoadtestMakefile(unittest.TestCase):
    def test_fleet_storm_targets(self) -> None:
        root = Path(__file__).resolve().parents[1]
        text = (root / "Makefile").read_text(encoding="utf-8")
        for name in (
            "loadtest-small",
            "loadtest-100",
            "loadtest-500",
            "loadtest-1000",
            "run_fleet_storm.sh",
        ):
            with self.subTest(target=name):
                self.assertIn(name, text)


if __name__ == "__main__":
    unittest.main()
