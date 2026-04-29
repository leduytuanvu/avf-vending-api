#!/usr/bin/env python3
"""Run buf breaking for machine proto files when the baseline already has them."""
from __future__ import annotations

import argparse
import os
import shlex
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
PROTO_DIR = ROOT / "proto"
DEFAULT_BUF = "go run github.com/bufbuild/buf/cmd/buf@v1.47.0"


def buf_args() -> list[str]:
    raw = os.environ.get("BUF", DEFAULT_BUF)
    return shlex.split(raw, posix=os.name != "nt")


def run_buf(args: list[str]) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        buf_args() + args,
        cwd=PROTO_DIR,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        check=False,
    )


def normalized(path: str) -> str:
    return path.replace("\\", "/").lstrip("./")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--against", required=True)
    parser.add_argument("--path", default="avf/machine/v1")
    args = parser.parse_args()

    baseline = run_buf(["ls-files", args.against])
    if baseline.returncode != 0:
        print(baseline.stdout, end="")
        return baseline.returncode

    path_prefix = normalized(args.path).rstrip("/") + "/"
    baseline_files = [normalized(line.strip()) for line in baseline.stdout.splitlines() if line.strip()]
    if not any(path_prefix in file for file in baseline_files):
        print(f"OK: no baseline proto files under {args.path}; skipping buf breaking for newly introduced service")
        return 0

    breaking = run_buf(["breaking", "--path", args.path, "--against", args.against])
    print(breaking.stdout, end="")
    return breaking.returncode


if __name__ == "__main__":
    raise SystemExit(main())
