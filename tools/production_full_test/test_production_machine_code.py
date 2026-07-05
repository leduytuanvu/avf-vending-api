#!/usr/bin/env python3
"""Unit tests for production test bootstrap helpers."""

from __future__ import annotations

import re
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

from _common import PRODUCTION_MACHINE_CODE_RE, production_machine_code

CANONICAL = re.compile(r"^AVF[0-9]{6}$")


class ProductionMachineCodeTests(unittest.TestCase):
    def test_regex_matches_canonical_six_digits(self) -> None:
        self.assertEqual(PRODUCTION_MACHINE_CODE_RE.pattern, r"^AVF[0-9]{6}$")

    def test_generated_codes_match_canonical_format(self) -> None:
        for _ in range(100):
            code = production_machine_code()
            self.assertRegex(code, CANONICAL)
            self.assertRegex(code, PRODUCTION_MACHINE_CODE_RE)
            self.assertEqual(len(code), 9)


if __name__ == "__main__":
    raise SystemExit(unittest.main())
