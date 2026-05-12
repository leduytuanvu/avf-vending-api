#!/usr/bin/env bash
# shellcheck shell=bash
# gRPC full coverage inventory + safe grpcurl probe wrapper.

set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
REPORT_DIR="${ROOT}/reports/test"
EVIDENCE_DIR="${REPORT_DIR}/grpc-production-evidence"
mkdir -p "${REPORT_DIR}" "${EVIDENCE_DIR}"

GRPC_ADDR="${GRPC_ADDR:-127.0.0.1:9090}"
GRPC_PLAINTEXT="${GRPC_PLAINTEXT:-true}"
PROTO_ROOT="${GRPC_PROTO_ROOT:-${ROOT}/proto}"
OUT_JSON="${REPORT_DIR}/grpc-full-coverage.json"
OUT_MD="${REPORT_DIR}/grpc-full-coverage.md"

grpcurl_base=(grpcurl)
if [[ "${GRPC_PLAINTEXT}" == "true" ]]; then
  grpcurl_base+=(-plaintext)
fi

if [[ -d "${PROTO_ROOT}" ]]; then
  grpcurl_base+=(-import-path "${PROTO_ROOT}")
  while IFS= read -r -d '' proto; do
    rel="${proto#${PROTO_ROOT}/}"
    grpcurl_base+=(-proto "${rel}")
  done < <(find "${PROTO_ROOT}" -name '*.proto' -print0)
fi

tmp_methods="$(mktemp)"
tmp_results="$(mktemp)"
trap 'rm -f "${tmp_methods}" "${tmp_results}"' EXIT

python3 - "${ROOT}" "${tmp_methods}" <<'PY'
import json
import re
import sys
from pathlib import Path

root = Path(sys.argv[1])
out = Path(sys.argv[2])
service_re = re.compile(r"^\s*service\s+(\w+)\s*\{")
rpc_re = re.compile(r"^\s*rpc\s+(\w+)\s*\(([^)]+)\)\s+returns\s+\(([^)]+)\)")
pkg_re = re.compile(r"^\s*package\s+([^;]+);")
rows = []
for path in sorted((root / "proto").rglob("*.proto")):
    package = ""
    service = ""
    for line in path.read_text(encoding="utf-8", errors="replace").splitlines():
        pkg = pkg_re.match(line)
        if pkg:
            package = pkg.group(1)
        svc = service_re.match(line)
        if svc:
            service = svc.group(1)
            continue
        rpc = rpc_re.match(line)
        if rpc and service:
            method = rpc.group(1)
            full = f"{package}.{service}/{method}" if package else f"{service}/{method}"
            low = method.lower()
            cls = "hardware-required" if any(x in low for x in ("vend", "dispense", "command")) else "write" if any(x in low for x in ("create", "update", "ack", "sync", "activate")) else "read-only"
            if "payment" in low or "order" in low:
                cls = "provider-required" if "payment" in low else cls
            rows.append({
                "service": service,
                "rpc": method,
                "full_method": full,
                "request_type": rpc.group(2).strip(),
                "response_type": rpc.group(3).strip(),
                "file": str(path.relative_to(root)).replace("\\", "/"),
                "auth_metadata": "authorization metadata with redacted machine or service token when method is protected",
                "classification": cls,
                "priority": "P0" if cls != "read-only" or any(x in low for x in ("auth", "bootstrap")) else "P1",
                "status": "partial",
                "reason": "not probed",
                "evidence_path": "",
            })
out.write_text(json.dumps(rows, indent=2), encoding="utf-8")
PY

server_status="blocked-tooling"
server_reason=""
if ! command -v grpcurl >/dev/null 2>&1; then
  server_reason="grpcurl not installed"
elif ! "${grpcurl_base[@]}" "${GRPC_ADDR}" list >"${EVIDENCE_DIR}/grpcurl-list.txt" 2>"${EVIDENCE_DIR}/grpcurl-list.stderr.txt"; then
  server_status="blocked-production-url"
  server_reason="gRPC server not reachable or reflection/proto list failed at ${GRPC_ADDR}"
else
  server_status="reachable"
  server_reason="grpcurl list succeeded"
fi

python3 - "${tmp_methods}" "${tmp_results}" "${server_status}" "${server_reason}" "${GRPC_ADDR}" "${EVIDENCE_DIR}" <<'PY'
import json
import sys
from datetime import datetime, timezone
from pathlib import Path

methods = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
server_status = sys.argv[3]
server_reason = sys.argv[4]
addr = sys.argv[5]
evidence = Path(sys.argv[6])

for row in methods:
    if server_status != "reachable":
        row["status"] = server_status
        row["reason"] = server_reason
        continue
    if row["classification"] == "read-only":
        row["status"] = "partial"
        row["reason"] = "server reachable; generic grpcurl call requires method-specific request template"
        row["evidence_path"] = str(evidence / "grpcurl-list.txt")
    elif row["classification"] == "write" and not bool(__import__("os").environ.get("MACHINE_TOKEN")):
        row["status"] = "blocked-missing-seed"
        row["reason"] = "MACHINE_TOKEN not set for authenticated machine write method"
    elif row["classification"] == "hardware-required":
        row["status"] = "blocked-hardware"
        row["reason"] = "requires canary hardware/device ACK evidence"
    elif row["classification"] == "provider-required":
        row["status"] = "blocked-provider"
        row["reason"] = "requires payment/provider sandbox evidence"

summary = {
    "generated_at": datetime.now(timezone.utc).isoformat(),
    "grpc_addr": addr,
    "server_status": server_status,
    "server_reason": server_reason,
    "total_methods": len(methods),
    "passed": sum(1 for r in methods if r["status"] == "pass"),
    "failed": sum(1 for r in methods if r["status"] == "fail"),
    "partial": sum(1 for r in methods if r["status"] == "partial"),
    "blocked": sum(1 for r in methods if str(r["status"]).startswith("blocked")),
}
Path(sys.argv[2]).write_text(json.dumps({"summary": summary, "methods": methods}, indent=2), encoding="utf-8")
PY

cp "${tmp_results}" "${OUT_JSON}"
python3 - "${OUT_JSON}" "${OUT_MD}" <<'PY'
import json
import sys
from pathlib import Path

payload = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
summary = payload["summary"]
methods = payload["methods"]
with Path(sys.argv[2]).open("w", encoding="utf-8") as f:
    f.write("# gRPC Full Coverage\n\n")
    for key in ("generated_at", "grpc_addr", "server_status", "total_methods", "passed", "failed", "partial", "blocked"):
        f.write(f"- {key.replace('_', ' ').title()}: `{summary.get(key)}`\n")
    f.write(f"- Server reason: {summary.get('server_reason')}\n\n")
    f.write("| Service | RPC | Priority | Class | Status | Reason |\n")
    f.write("|---|---|---|---|---|---|\n")
    for row in methods:
        f.write(f"| `{row['service']}` | `{row['rpc']}` | {row['priority']} | {row['classification']} | **{row['status']}** | {row['reason'][:120]} |\n")
PY

echo "Wrote ${OUT_JSON} and ${OUT_MD}"
exit 0
