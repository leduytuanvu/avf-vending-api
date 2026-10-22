#!/usr/bin/env python3
"""AVF000195 QR latency probe — machine gRPC sandbox, no TCN hardware.

Kiosk commerce is gRPC (MachineCommerceService), not REST /v1/machine/quotes.
This wrapper either prints identity (AVF_SKIP=1) or delegates to:

  go run ./cmd/qr-latency-avf000195

Default target is local plaintext gRPC. Production live MoMo is refused by the
Go tool unless PAYMENT_ENV is not live.

Env:
  AVF_SKIP=1                 print identity and exit 0 (CI)
  MACHINE_ACCESS_TOKEN       required for a real call
  AVF_PRODUCT_ID             optional; if empty, GetSaleCatalog picks one in-stock line
  AVF_GRPC_TARGET            default localhost:9090
                             staging: grpcs://staging-machine-api.ldtv.dev:443
  AVF_PROVIDERS              default psp_grpc_int,sandbox (never momo live)
  AVF_LATENCY_BUDGET_MS      default 3000; fail sandbox sessions above this
  PAYMENT_ENV                live against api.ldtv.dev/machine-api.ldtv.dev + momo is refused
  AVF_MACHINE_ID             default 01a0a7e5-3c68-7895-b526-bcb6504bccfb (JWT must match)
"""
from __future__ import annotations

import os
import subprocess
import sys
from pathlib import Path

MACHINE_CODE = "AVF000195"
MACHINE_ID = "01a0a7e5-3c68-7895-b526-bcb6504bccfb"


def main() -> int:
    if os.environ.get("AVF_SKIP") == "1":
        print(f"skip live call machine={MACHINE_CODE} id={MACHINE_ID}")
        return 0
    api_root = Path(__file__).resolve().parents[2]
    cmd = ["go", "run", "./cmd/qr-latency-avf000195"]
    print("delegating to:", " ".join(cmd), "cwd=", api_root, flush=True)
    proc = subprocess.run(cmd, cwd=str(api_root))
    return proc.returncode


if __name__ == "__main__":
    raise SystemExit(main())
