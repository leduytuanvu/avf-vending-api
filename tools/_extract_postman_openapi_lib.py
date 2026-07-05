#!/usr/bin/env python3
"""One-off extractor: legacy generate_full_postman_suite.py -> postman_openapi_lib.py"""
from __future__ import annotations

from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
src_path = ROOT / ".tmp-postman-openapi-lib-source.py"
dst_path = ROOT / "scripts" / "postman" / "postman_openapi_lib.py"

lines = src_path.read_text(encoding="utf-8").splitlines()
start = next(i for i, l in enumerate(lines) if l.startswith("def sanitize_full100_text"))
end = next(i for i, l in enumerate(lines) if l.startswith("def write_csv"))

header = '''"""OpenAPI/proto/MQTT Postman builder library (legacy full-production-suite core)."""
from __future__ import annotations

import csv
import hashlib
import json
import os
import re
import shutil
import subprocess
import sys
from collections import defaultdict
from datetime import datetime, timezone
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
SWAGGER = REPO_ROOT / "docs" / "swagger" / "swagger.json"
PROTO_AVF = REPO_ROOT / "proto" / "avf"

HTTP_VERBS = frozenset({"get", "post", "put", "patch", "delete", "options", "head", "trace"})

REGISTERED_SERVICES = {
    ("avf.machine.v1", "MachineActivationService"),
    ("avf.machine.v1", "MachineTokenService"),
    ("avf.machine.v1", "MachineAuthService"),
    ("avf.machine.v1", "MachineBootstrapService"),
    ("avf.machine.v1", "MachineCatalogService"),
    ("avf.machine.v1", "MachineMediaService"),
    ("avf.machine.v1", "MachineInventoryService"),
    ("avf.machine.v1", "MachineTelemetryService"),
    ("avf.machine.v1", "MachineOperatorService"),
    ("avf.machine.v1", "MachineCommerceService"),
    ("avf.machine.v1", "MachineSaleService"),
    ("avf.machine.v1", "MachineOfflineSyncService"),
    ("avf.machine.v1", "MachineCommandService"),
    ("avf.internal.v1", "InternalMachineQueryService"),
    ("avf.internal.v1", "InternalTelemetryQueryService"),
    ("avf.internal.v1", "InternalCommerceQueryService"),
    ("avf.internal.v1", "InternalPaymentQueryService"),
    ("avf.internal.v1", "InternalCatalogQueryService"),
    ("avf.internal.v1", "InternalInventoryQueryService"),
    ("avf.internal.v1", "InternalReportingQueryService"),
}

REST_EXPECTED = 329
GRPC_EXPECTED = 86
MQTT_EXPECTED = 28

'''

body = "\n".join(lines[start:end]) + "\n"
dst_path.parent.mkdir(parents=True, exist_ok=True)
dst_path.write_text(header + body, encoding="utf-8")
print(f"Wrote {dst_path} ({len(header + body)} bytes)")
