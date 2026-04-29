#!/usr/bin/env python3
"""Verify that DR readiness docs and production env examples expose the required contract."""

from __future__ import annotations

import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


CHECKS: dict[str, list[str]] = {
    "docs/runbooks/multi-region-dr-readiness.md": [
        "APP_REGION",
        "APP_NODE_NAME",
        "APP_INSTANCE_ID",
        "PostgreSQL RPO/RTO",
        "Redis is non-authoritative",
        "NATS and JetStream recovery",
        "Object storage recovery",
        "Restore drill validation checklist",
        "Active-active writes are not supported",
    ],
    "docs/runbooks/backup-restore-drill.md": [
        "expected and observed RPO/RTO",
        "Redis state posture",
        "NATS/JetStream posture",
        "object storage bucket/versioning validation",
    ],
    "docs/operations/production-data-migration-safety.md": [
        "Idempotency keys",
        "Redis must not be used as source data",
        "JetStream replay",
        "Database rollback is not automatic",
    ],
    "deployments/prod/app-node/.env.app-node.example": [
        "APP_REGION=",
        "APP_NODE_NAME=",
        "APP_INSTANCE_ID=",
    ],
    "deployments/prod/.env.production.example": [
        "APP_REGION=",
        "APP_NODE_NAME=",
        "APP_INSTANCE_ID=",
    ],
}


def main() -> int:
    missing: list[str] = []
    for rel, needles in CHECKS.items():
        path = ROOT / rel
        if not path.exists():
            missing.append(f"{rel}: file missing")
            continue
        text = path.read_text(encoding="utf-8")
        for needle in needles:
            if needle not in text:
                missing.append(f"{rel}: missing {needle!r}")
    if missing:
        print("FAIL: DR readiness contract incomplete", file=sys.stderr)
        for item in missing:
            print(f"- {item}", file=sys.stderr)
        return 1
    print("PASS: DR readiness docs and env examples cover required contract")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
