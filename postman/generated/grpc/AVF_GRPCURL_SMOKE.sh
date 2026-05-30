#!/usr/bin/env bash
# AVF gRPC smoke — grpcurl coverage for all inventory methods.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXAMPLES="${SCRIPT_DIR}/AVF_GRPC_EXAMPLES.json"
MODE="${1:-run}"
GRPC_HOST="${GRPC_HOST:-localhost}"
GRPC_PORT="${GRPC_PORT:-50051}"
GRPC_TLS="${GRPC_TLS:-false}"
ACCESS_TOKEN="${ACCESS_TOKEN:-}"
MACHINE_TOKEN="${MACHINE_TOKEN:-}"
REQUEST_ID="${REQUEST_ID:-$(uuidgen 2>/dev/null || cat /proc/sys/kernel/random/uuid 2>/dev/null || echo test-req-id)}"
ADDR="${GRPC_HOST}:${GRPC_PORT}"

need() { command -v "$1" >/dev/null 2>&1 || { echo "MISSING: $1"; exit 1; }; }
PY=""
for cand in python python3; do
  if command -v "$cand" >/dev/null 2>&1; then PY="$cand"; break; fi
done
[ -n "$PY" ] || { echo "MISSING: python3 or python"; exit 1; }

if [ "$MODE" = "list" ] || [ "$MODE" = "--list" ]; then
  export EXAMPLES
  "$PY" - <<'PY'
import json, os, pathlib
for row in json.loads(pathlib.Path(os.environ["EXAMPLES"]).read_text(encoding="utf-8")):
    print(" ", row["fullMethod"])
PY
  exit 0
fi

if [ "$MODE" = "dry-run" ] || [ "$MODE" = "--dry-run" ]; then
  echo "DRY-RUN grpcurl against ${ADDR} (TLS=${GRPC_TLS})"
  export EXAMPLES REQUEST_ID
  "$PY" - <<'PY'
import json, os, pathlib
examples = json.loads(pathlib.Path(os.environ["EXAMPLES"]).read_text(encoding="utf-8"))
rid = os.environ.get("REQUEST_ID", "")
for row in examples:
    fm = row["fullMethod"]
    machine = ".machine." in fm
    tok = "MACHINE_TOKEN" if machine else "ACCESS_TOKEN"
    print(f"grpcurl -H authorization:Bearer${{{tok}}} -H x-request-id:{rid} -d @ {fm}")
PY
  exit 0
fi

need grpcurl
export EXAMPLES ADDR REQUEST_ID ACCESS_TOKEN MACHINE_TOKEN GRPC_TLS
"$PY" - <<'PY'
import json, os, subprocess, pathlib, sys
examples = json.loads(pathlib.Path(os.environ["EXAMPLES"]).read_text(encoding="utf-8"))
addr = os.environ["ADDR"]
request_id = os.environ.get("REQUEST_ID", "test-req-id")
access = os.environ.get("ACCESS_TOKEN", "")
machine = os.environ.get("MACHINE_TOKEN", "")
tls = os.environ.get("GRPC_TLS", "false")
tls_args = [] if tls in ("true", "1") else ["-plaintext"]
pass_count = fail_count = 0
for row in examples:
    fm = row["fullMethod"]
    use_machine = ".machine." in fm
    token = machine if use_machine else access
    var = "MACHINE_TOKEN" if use_machine else "ACCESS_TOKEN"
    if not token:
        print(f"FAIL {fm} missing {var}")
        fail_count += 1
        continue
    payload = json.dumps(row.get("requestExample") or {})
    cmd = ["grpcurl", *tls_args, "-H", f"authorization: Bearer {token}", "-H", f"x-request-id: {request_id}", "-d", payload, addr, fm]
    try:
        subprocess.run(cmd, check=True, capture_output=True, timeout=30)
        print(f"PASS {fm}")
        pass_count += 1
    except Exception:
        print(f"FAIL {fm}")
        fail_count += 1
print(f"SUMMARY pass={pass_count} fail={fail_count}")
sys.exit(1 if fail_count else 0)
PY
