#!/usr/bin/env python3
from __future__ import annotations

import os
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "deployments" / "prod" / "shared" / "scripts" / "check_managed_services.sh"


def _resolve_bash() -> str | None:
    candidates: list[Path] = []
    if os.name == "nt":
        for base in (os.environ.get("ProgramFiles"), os.environ.get("ProgramFiles(x86)")):
            if base:
                candidates.append(Path(base) / "Git" / "bin" / "bash.exe")
    w = shutil.which("bash")
    if w:
        candidates.append(Path(w))
    for p in candidates:
        if p.is_file():
            return str(p)
    return None


BASH = _resolve_bash()
requires_bash = unittest.skipUnless(BASH is not None, "bash not found (install Git for Windows or add bash to PATH)")


@requires_bash
class CheckManagedServicesNatsTests(unittest.TestCase):
    def test_production_requires_nats_url(self) -> None:
        with tempfile.TemporaryDirectory() as td:
            env_file = Path(td) / "env"
            env_file.write_text(
                "DATABASE_URL=postgresql://u:p@127.0.0.1:1/db\n"
                "APP_ENV=production\n"
                "API_ARTIFACTS_ENABLED=0\n",
                encoding="utf-8",
            )
            proc = subprocess.run(
                [BASH, str(SCRIPT), str(env_file)],
                cwd=str(ROOT),
                capture_output=True,
                text=True,
                timeout=120,
            )
        self.assertNotEqual(proc.returncode, 0)
        combined = proc.stderr + proc.stdout
        self.assertIn("NATS_URL is required", combined)

    def test_nats_url_unparseable_fails(self) -> None:
        with tempfile.TemporaryDirectory() as td:
            env_file = Path(td) / "env"
            env_file.write_text(
                "DATABASE_URL=postgresql://u:p@127.0.0.1:1/db\n"
                "APP_ENV=staging\n"
                "NATS_URL=%%%not-a-uri\n"
                "API_ARTIFACTS_ENABLED=0\n",
                encoding="utf-8",
            )
            proc = subprocess.run(
                [BASH, str(SCRIPT), str(env_file)],
                cwd=str(ROOT),
                capture_output=True,
                text=True,
                timeout=120,
            )
        self.assertNotEqual(proc.returncode, 0)
        combined = proc.stderr + proc.stdout
        self.assertIn("NATS_URL is not parseable", combined)

    def test_nats_host_port_defaults_4222(self) -> None:
        """Mirrors check_managed_services.sh python_value mode nats_url_host_port."""
        from urllib.parse import urlsplit

        def nats_host_port(value: str) -> str:
            parsed = urlsplit(value)
            host = parsed.hostname or ""
            if not host:
                raise ValueError("host")
            port = parsed.port
            if port is None:
                port = 4222
            return f"{host}:{port}"

        self.assertEqual(nats_host_port("nats://broker.internal"), "broker.internal:4222")
        self.assertEqual(nats_host_port("tls://broker.internal:5222"), "broker.internal:5222")


if __name__ == "__main__":
    unittest.main()
