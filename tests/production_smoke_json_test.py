#!/usr/bin/env python3
from __future__ import annotations

import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import threading
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts" / "smoke"))


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

import smoke_evidence_json  # noqa: E402
import validate_public_version_json  # noqa: E402


requires_bash = unittest.skipUnless(BASH is not None, "bash not found (install Git for Windows or add bash to PATH)")


@requires_bash
class SmokeProdScriptJsonModeTests(unittest.TestCase):
    """Blackbox: smoke_prod.sh --json must write JSON-only stdout (deploy evidence)."""

    def test_json_stdout_is_parseable_and_has_overall_status(self) -> None:
        class Handler(BaseHTTPRequestHandler):
            def log_message(self, fmt: str, *args: object) -> None:
                return

            def do_GET(self) -> None:
                path = self.path.split("?", 1)[0]
                if path in ("/health/ready", "/health/live"):
                    self.send_response(200)
                    self.send_header("Content-Type", "text/plain")
                    self.end_headers()
                    self.wfile.write(b"ok\n")
                elif path == "/version":
                    self.send_response(200)
                    self.send_header("Content-Type", "application/json")
                    self.end_headers()
                    self.wfile.write(b'{"version":"9.9.9-test"}\n')
                elif path in ("/", ""):
                    self.send_response(200)
                    self.end_headers()
                    self.wfile.write(b"ok\n")
                else:
                    self.send_response(404)
                    self.end_headers()

        server = HTTPServer(("127.0.0.1", 0), Handler)
        port = server.server_port
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            script = ROOT / "deployments" / "prod" / "scripts" / "smoke_prod.sh"
            env = os.environ.copy()
            env["SMOKE_BASE_URL"] = f"http://127.0.0.1:{port}"
            env["SMOKE_LEVEL"] = "health"
            env["SMOKE_PYTHON"] = sys.executable
            proc = subprocess.run(
                [BASH, str(script), "--json"],
                cwd=str(ROOT),
                env=env,
                capture_output=True,
                text=True,
                timeout=120,
                check=False,
            )
        finally:
            server.shutdown()
            server.server_close()

        self.assertEqual(proc.returncode, 0, proc.stderr)
        out = proc.stdout.strip()
        self.assertFalse(re.search(r"(PASS:|FAIL:|==>)", out), msg=f"human markers leaked to stdout: {out[:500]!r}")
        doc = json.loads(out)
        self.assertEqual(doc.get("overall_status"), "pass")
        self.assertIn("checks", doc)
        self.assertRegex(proc.stderr or "", r"PASS:")
        with tempfile.NamedTemporaryFile(mode="w", delete=False, suffix=".json", encoding="utf-8") as tmp:
            tmp.write(proc.stdout)
            tmp_path = tmp.name
        try:
            jt = subprocess.run(
                [sys.executable, "-m", "json.tool", tmp_path],
                capture_output=True,
                text=True,
                timeout=30,
            )
            self.assertEqual(jt.returncode, 0, jt.stderr)
        finally:
            Path(tmp_path).unlink(missing_ok=True)


class SmokeEvidenceJsonTests(unittest.TestCase):
    def test_empty_file_has_actionable_message(self) -> None:
        with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False) as f:
            path = Path(f.name)
        try:
            path.write_text("", encoding="utf-8")
            with self.assertRaises(smoke_evidence_json.SmokeEvidenceError) as ctx:
                smoke_evidence_json.load_smoke_evidence_dict(path)
            self.assertIn("empty", str(ctx.exception).lower())
        finally:
            path.unlink(missing_ok=True)

    def test_html_body_fails_parse_with_snippet(self) -> None:
        with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False) as f:
            path = Path(f.name)
        try:
            path.write_text("<html><title>x</title></html>", encoding="utf-8")
            with self.assertRaises(smoke_evidence_json.SmokeEvidenceError) as ctx:
                smoke_evidence_json.load_smoke_evidence_dict(path)
            msg = str(ctx.exception)
            self.assertIn("invalid JSON", msg)
            self.assertIn("html", msg.lower())
        finally:
            path.unlink(missing_ok=True)

    def test_valid_evidence_passes(self) -> None:
        with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False) as f:
            path = Path(f.name)
        try:
            path.write_text(json.dumps({"overall_status": "pass", "failed_checks": []}), encoding="utf-8")
            doc = smoke_evidence_json.load_smoke_evidence_dict(path)
            self.assertEqual(doc["overall_status"], "pass")
        finally:
            path.unlink(missing_ok=True)


class ValidatePublicVersionJsonTests(unittest.TestCase):
    def test_empty_body_fails(self) -> None:
        with self.assertRaises(validate_public_version_json.VersionSmokeError) as ctx:
            validate_public_version_json.validate_public_version_response(
                endpoint="https://example.invalid/version",
                http_status="200",
                content_type="application/json",
                body_raw=b"",
                expect_app_env=None,
                expect_public_base_url=None,
            )
        self.assertIn("empty", str(ctx.exception).lower())

    def test_html_body_not_json(self) -> None:
        with self.assertRaises(validate_public_version_json.VersionSmokeError) as ctx:
            validate_public_version_json.validate_public_version_response(
                endpoint="https://example.invalid/version",
                http_status="200",
                content_type="text/html",
                body_raw=b"<!DOCTYPE html><html></html>",
                expect_app_env=None,
                expect_public_base_url=None,
            )
        self.assertIn("not valid JSON", str(ctx.exception))

    def test_plain_ok_not_json(self) -> None:
        """Health endpoints use plain text; /version must be JSON."""
        with self.assertRaises(validate_public_version_json.VersionSmokeError) as ctx:
            validate_public_version_json.validate_public_version_response(
                endpoint="https://example.invalid/version",
                http_status="200",
                content_type="text/plain",
                body_raw=b"ok\n",
                expect_app_env=None,
                expect_public_base_url=None,
            )
        self.assertIn("not valid JSON", str(ctx.exception))

    def test_typical_version_payload_passes(self) -> None:
        payload = {
            "name": "api",
            "version": "1.2.3",
            "app_env": "production",
            "public_base_url": "https://api.example.com",
        }
        validate_public_version_json.validate_public_version_response(
            endpoint="https://api.example.com/version",
            http_status="200",
            content_type="application/json; charset=utf-8",
            body_raw=json.dumps(payload).encode(),
            expect_app_env="production",
            expect_public_base_url="https://api.example.com/",
        )

    def test_app_env_mismatch(self) -> None:
        payload = {"version": "1", "app_env": "staging"}
        with self.assertRaises(validate_public_version_json.VersionSmokeError) as ctx:
            validate_public_version_json.validate_public_version_response(
                endpoint="/version",
                http_status="200",
                content_type="application/json",
                body_raw=json.dumps(payload).encode(),
                expect_app_env="production",
                expect_public_base_url=None,
            )
        self.assertIn("app_env mismatch", str(ctx.exception))


if __name__ == "__main__":
    unittest.main()
