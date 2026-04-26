#!/usr/bin/env python3
"""Validate container image refs are digest-pinned (registry/.../name@sha256:...)."""
from __future__ import annotations

import argparse
import re
import sys

_DIGEST_RE = re.compile(r"sha256:[0-9a-f]{64}")


def need_digest_pinned(name: str, ref: str) -> None:
    sref = (ref or "").strip()
    if not sref or "@sha256:" not in sref or not _DIGEST_RE.search(sref):
        print(
            "error: %s must be digest-pinned (registry/.../img@sha256:...), got %r" % (name, ref),
            file=sys.stderr,
        )
        raise SystemExit(1)


def main() -> None:
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--app-ref", default="", help="App image ref")
    p.add_argument("--goose-ref", default="", help="Goose image ref")
    args = p.parse_args()
    need_digest_pinned("app_image_ref", args.app_ref)
    need_digest_pinned("goose_image_ref", args.goose_ref)


if __name__ == "__main__":
    main()
