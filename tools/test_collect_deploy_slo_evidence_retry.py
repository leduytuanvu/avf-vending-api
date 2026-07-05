#!/usr/bin/env python3
"""Tests collect_deploy_slo_evidence.sh critical health retry behavior."""

from __future__ import annotations

import json
import os
import shutil
import subprocess
import sys
import threading
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path

REPO = Path(__file__).resolve().parents[1]
SCRIPT = REPO / "scripts" / "deploy" / "monitoring" / "collect_deploy_slo_evidence.sh"


class _ReadyFlapHandler(BaseHTTPRequestHandler):
    ready_hits = 0

    def do_GET(self) -> None:  # noqa: N802
        if self.path == "/health/ready":
            type(self).ready_hits += 1
            code = 503 if type(self).ready_hits < 3 else 200
            self.send_response(code)
            self.end_headers()
            self.wfile.write(b"ready")
        elif self.path == "/health/live":
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b"live")
        elif self.path == "/version":
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b'{"git_sha":"test"}')
        else:
            self.send_response(404)
            self.end_headers()

    def log_message(self, format: str, *args: object) -> None:
        return


class CollectDeploySloRetryTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.bash = shutil.which("bash")
        if cls.bash is None:
            raise unittest.SkipTest("bash not available")
        probe = subprocess.run([cls.bash, "-c", "echo ok"], capture_output=True, text=True, check=False)
        if probe.returncode != 0 or "ok" not in probe.stdout:
            raise unittest.SkipTest("bash not functional on this host")

    def _run_collector(self, base_url: str, *, out_path: Path) -> tuple[int, dict]:
        env = os.environ.copy()
        env["BASE_URL"] = base_url
        env["DEPLOY_SLO_CRITICAL"] = "1"
        env["SLO_CRITICAL_RETRIES"] = "5"
        env["SLO_CRITICAL_RETRY_SLEEP_SEC"] = "0"
        proc = subprocess.run(
            [self.bash, str(SCRIPT), "--json", "--phase", "pre_deploy", "--out", str(out_path)],
            cwd=REPO,
            env=env,
            capture_output=True,
            text=True,
            check=False,
        )
        if not out_path.is_file():
            self.fail(f"collector produced no output (rc={proc.returncode}): {proc.stderr[:800]}")
        data = json.loads(out_path.read_text(encoding="utf-8"))
        return proc.returncode, data

    def test_retries_transient_ready_503_then_passes(self) -> None:
        _ReadyFlapHandler.ready_hits = 0
        server = HTTPServer(("127.0.0.1", 0), _ReadyFlapHandler)
        port = server.server_address[1]
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            out_path = REPO / ".deploy-tmp" / "slo-retry-test-pass.json"
            out_path.parent.mkdir(parents=True, exist_ok=True)
            rc, data = self._run_collector(f"http://127.0.0.1:{port}", out_path=out_path)
            self.assertEqual(rc, 0, data)
            self.assertEqual(data["critical"]["assessment"], "pass")
            self.assertGreaterEqual(data["critical"]["retry_attempts"], 3)
            self.assertEqual(data["critical"]["public_health_ready"]["http_code"], "200")
        finally:
            server.shutdown()

    def test_fails_when_ready_stays_503(self) -> None:
        class Always503Handler(BaseHTTPRequestHandler):
            def do_GET(self) -> None:  # noqa: N802
                if self.path == "/health/live":
                    self.send_response(200)
                else:
                    self.send_response(503)
                self.end_headers()
                self.wfile.write(b"x")

            def log_message(self, format: str, *args: object) -> None:
                return

        server = HTTPServer(("127.0.0.1", 0), Always503Handler)
        port = server.server_address[1]
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            out_path = REPO / ".deploy-tmp" / "slo-retry-test-fail.json"
            out_path.parent.mkdir(parents=True, exist_ok=True)
            rc, data = self._run_collector(f"http://127.0.0.1:{port}", out_path=out_path)
            self.assertEqual(rc, 1)
            self.assertEqual(data["critical"]["assessment"], "fail")
            self.assertEqual(data["critical"]["retry_attempts"], 5)
        finally:
            server.shutdown()


if __name__ == "__main__":
    raise SystemExit(unittest.main())
