#!/usr/bin/env python3
"""Verify machine gRPC production contract doc covers all machine v1 services and key flows."""
from __future__ import annotations

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
PROTO_DIR = ROOT / "proto" / "avf" / "machine" / "v1"
DOC = ROOT / "docs" / "api" / "machine-grpc-production-contract.md"
SYNC_DOC = ROOT / "docs" / "api" / "android-proto-sync.md"

REQUIRED_FLOW_ANCHORS = [
    "ClaimActivation",
    "RefreshMachineToken",
    "GetBootstrap",
    "CheckIn",
    "AckConfigVersion",
    "CheckForUpdates",
    "GetCatalogSnapshot",
    "SyncCatalogBundle",
    "GetCatalogDelta",
    "AckCatalogVersion",
    "GetMediaManifest",
    "GetMediaDelta",
    "AckMediaVersion",
    "GetInventorySnapshot",
    "GetPlanogram",
    "CreateOrder",
    "CreatePaymentSession",
    "ConfirmCashPayment",
    "CreateCashCheckout",
    "GetOrderStatus",
    "StartVend",
    "ReportVendSuccess",
    "ReportVendFailure",
    "CancelOrder",
    "SubmitTelemetryBatch",
    "PushCriticalEvent",
    "PushOfflineEvents",
    "GetSyncCursor",
    "SubmitFillReport",
    "SubmitStockAdjustment",
    "GetAssignedUpdate",
    "ReportDiagnosticBundleResult",
]

REQUIRED_ANDROID_FLOW_INDEX = [
    "Android runtime flow index",
    "check for updates",
    "get catalog delta",
    "ack catalog version",
    "ack media version",
    "get order status",
    "create cash checkout",
    "cancel order",
    "get sync cursor",
]

SERVICE_RE = re.compile(r"^\s*service\s+([A-Za-z0-9_]+)\s*\{", re.MULTILINE)


def main() -> int:
    if not DOC.is_file():
        print(f"ERROR: missing {DOC}", file=sys.stderr)
        return 1
    if not SYNC_DOC.is_file():
        print(f"ERROR: missing {SYNC_DOC} — run generate_android_proto_sync_doc.py", file=sys.stderr)
        return 1

    services: set[str] = set()
    for path in sorted(PROTO_DIR.glob("*.proto")):
        if path.name == "machine_runtime.proto":
            continue
        services.update(SERVICE_RE.findall(path.read_text(encoding="utf-8")))

    doc = DOC.read_text(encoding="utf-8")
    missing_svc = sorted(s for s in services if s not in doc)
    if missing_svc:
        print("ERROR: production contract doc missing services:", file=sys.stderr)
        for s in missing_svc:
            print(f"  {s}", file=sys.stderr)
        return 1

    missing_flow = [a for a in REQUIRED_FLOW_ANCHORS if a not in doc]
    if missing_flow:
        print("ERROR: production contract doc missing flow RPC anchors:", file=sys.stderr)
        for a in missing_flow:
            print(f"  {a}", file=sys.stderr)
        return 1

    missing_index = [s for s in REQUIRED_ANDROID_FLOW_INDEX if s not in doc]
    if missing_index:
        print("ERROR: production contract doc missing Android flow index sections:", file=sys.stderr)
        for s in missing_index:
            print(f"  {s}", file=sys.stderr)
        return 1

    required_sections = [
        "Machine JWT",
        "Idempotency",
        "Legacy REST",
        "MQTT",
        "Persistence",
        "RPC reference",
    ]
    missing_sec = [s for s in required_sections if s not in doc]
    if missing_sec:
        print("ERROR: production contract doc missing sections:", file=sys.stderr)
        for s in missing_sec:
            print(f"  {s}", file=sys.stderr)
        return 1

    print(f"OK: production contract doc covers {len(services)} services and {len(REQUIRED_FLOW_ANCHORS)} flow anchors")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
