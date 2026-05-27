#!/usr/bin/env python3
"""Generate REST/gRPC/MQTT/worker inventory from repo sources (swagger, proto, mqtt topics, cmd)."""
from __future__ import annotations

import json
import re
from datetime import datetime, timezone
from pathlib import Path

REPO = Path(__file__).resolve().parents[1]
SWAGGER = REPO / "docs" / "swagger" / "swagger.json"
PROTO_MACHINE = REPO / "proto" / "avf" / "machine" / "v1"
PROTO_INTERNAL = REPO / "proto" / "avf" / "internal" / "v1"
TOPICS_GO = REPO / "internal" / "platform" / "mqtt" / "topics.go"
OUT_MD = REPO / "docs" / "audits" / "api-grpc-mqtt-full-inventory.md"
OUT_JSON = REPO / "build" / "reports" / "api-grpc-mqtt-full-inventory.json"

LEGACY_REST_PREFIXES = (
    "/v1/admin/users",
    "/v1/admin/media/uploads",
)
LEGACY_REST_EXACT = {"/v1/admin/media"}
LEGACY_REST_SUFFIX = ("/image", "/commands/dispatch")
LEGACY_REST_CONTAINS = ("/v1/setup/", "/v1/commerce/", "/v1/device/", "/sale-catalog", "/shadow")


def classify_rest_path(path: str) -> str:
    p = path.split("?")[0].rstrip("/") or "/"
    if any(p.startswith(x) for x in LEGACY_REST_PREFIXES):
        return "legacy"
    if p in LEGACY_REST_EXACT:
        return "legacy"
    if any(p.endswith(s) for s in LEGACY_REST_SUFFIX):
        return "legacy"
    if any(x in p for x in LEGACY_REST_CONTAINS):
        return "legacy"
    return "canonical"


def audience_for_path(path: str, op: dict) -> str:
    if path.startswith("/health") or path == "/version":
        return "Public/System"
    if "/webhooks/" in path or "webhook" in path:
        return "Payment Provider"
    if path.startswith("/v1/admin/"):
        return "Admin Web"
    if path.startswith("/v1/operator/"):
        return "Operator"
    if path.startswith("/v1/setup/") or path.startswith("/v1/machines/") or path.startswith("/v1/device/"):
        return "Machine App"
    if path.startswith("/v1/commerce/"):
        return "Machine App"
    if path.startswith("/v1/auth/"):
        tags = op.get("tags") or []
        if "Machine" in str(tags):
            return "Machine App"
        return "Admin Web"
    sec = op.get("security") or []
    if sec:
        return "Authenticated"
    return "Public/System"


def load_rest_from_swagger() -> list[dict]:
    if not SWAGGER.exists():
        raise SystemExit(f"missing {SWAGGER}; run: make swagger")
    spec = json.loads(SWAGGER.read_text(encoding="utf-8"))
    rows: list[dict] = []
    for path, item in sorted((spec.get("paths") or {}).items()):
        for method, op in sorted(item.items()):
            if method.startswith("x-") or not isinstance(op, dict):
                continue
            m = method.upper()
            rows.append(
                {
                    "protocol": "rest",
                    "method": m,
                    "path": path,
                    "operationId": op.get("operationId", ""),
                    "summary": (op.get("summary") or "")[:120],
                    "tags": op.get("tags") or [],
                    "auth": "bearer" if op.get("security") else "none",
                    "audience": audience_for_path(path, op),
                    "surface": classify_rest_path(path),
                    "handler": f"swagger:{op.get('operationId', '')}",
                }
            )
    return rows


def parse_proto_dir(proto_dir: Path, package_hint: str) -> list[dict]:
    rows: list[dict] = []
    if not proto_dir.exists():
        return rows
    svc_re = re.compile(r"^\s*service\s+(\w+)\s*\{", re.M)
    rpc_re = re.compile(r"^\s*rpc\s+(\w+)\s*\([^)]*\)\s*returns", re.M)
    for fp in sorted(proto_dir.glob("*.proto")):
        text = fp.read_text(encoding="utf-8")
        pkg_m = re.search(r"package\s+([\w.]+)\s*;", text)
        pkg = pkg_m.group(1) if pkg_m else package_hint
        for svc_m in svc_re.finditer(text):
            svc = svc_m.group(1)
            start = svc_m.end()
            # naive block: find matching brace
            depth = 1
            i = start
            while i < len(text) and depth:
                if text[i] == "{":
                    depth += 1
                elif text[i] == "}":
                    depth -= 1
                i += 1
            block = text[svc_m.start() : i]
            for rpc_m in rpc_re.finditer(block):
                rpc = rpc_m.group(1)
                legacy = svc in ("MachineAuthService",) or "Skeleton" in svc
                if svc == "MachineSaleService":
                    legacy = "legacy_companion"
                audience = "Internal Backend" if "internal" in pkg else "Machine App"
                rows.append(
                    {
                        "protocol": "grpc",
                        "package": pkg,
                        "service": svc,
                        "method": rpc,
                        "proto_file": str(fp.relative_to(REPO)).replace("\\", "/"),
                        "audience": audience,
                        "surface": "legacy" if legacy is True else ("legacy_companion" if legacy == "legacy_companion" else "canonical"),
                    }
                )
    return rows


def load_mqtt_from_topics_go() -> list[dict]:
    if not TOPICS_GO.exists():
        return []
    text = TOPICS_GO.read_text(encoding="utf-8")
    rows: list[dict] = []
    const_re = re.compile(r'RelTopic(\w+)\s*=\s*"([^"]+)"')
    for name, rel in const_re.findall(text):
        direction = "machine_to_backend"
        if "Command" in name or "ShadowDesired" in name:
            direction = "backend_to_machine"
        layout = "both"
        if "Enterprise" in name or name in ("OutboundEnterpriseCommandTopic",):
            layout = "enterprise"
        surface = "canonical"
        if "Legacy" in name or rel in ("commands/dispatch", "commands/down", "commands/receipt", "telemetry"):
            surface = "legacy"
        if rel == "events":
            surface = "legacy_compatibility"
        rows.append(
            {
                "protocol": "mqtt",
                "channel": rel,
                "constant": f"RelTopic{name}",
                "direction": direction,
                "layout": layout,
                "surface": surface,
                "handler": "mqtt-ingest Dispatch",
            }
        )
    # subscribe patterns
    for fn, layout in (
        ("InboundDeviceTopicPatterns", "legacy"),
        ("InboundEnterpriseDeviceTopicPatterns", "enterprise"),
    ):
        if fn in text:
            rows.append(
                {
                    "protocol": "mqtt",
                    "channel": f"subscription:{fn}",
                    "constant": fn,
                    "direction": "machine_to_backend",
                    "layout": layout,
                    "surface": "canonical" if layout == "enterprise" else "legacy",
                    "handler": "subscriber.go",
                }
            )
    return rows


def load_workers() -> list[dict]:
    workers = [
        ("api", "cmd/api/main.go", "HTTP + optional gRPC", []),
        ("worker", "cmd/worker/main.go", "outbox, telemetry projection, payment timeout, retention", ["outbox", "telemetry", "payment_timeout"]),
        ("reconciler", "cmd/reconciler/main.go", "order/vend/psp/refund reconciliation ticks", ["reconciler"]),
        ("mqtt-ingest", "cmd/mqtt-ingest/main.go", "MQTT subscribe + telemetry pipeline + command ack sweep", ["mqtt_ingest"]),
        ("temporal-worker", "cmd/temporal-worker/main.go", "Temporal workflows/activities", ["temporal"]),
        ("migrate", "cmd/migrate/main.go", "goose migrations one-shot", ["migration"]),
        ("outbox-replay", "cmd/outbox-replay/main.go", "CLI outbox list/requeue", ["outbox"]),
    ]
    return [
        {
            "protocol": "worker",
            "name": name,
            "entry": entry,
            "description": desc,
            "jobs": jobs,
        }
        for name, entry, desc, jobs in workers
    ]


def render_md(data: dict) -> str:
    lines = [
        "# API / gRPC / MQTT full inventory",
        "",
        f"Generated: {data['generated_at']}",
        f"Source commit: {data.get('note', 'local')}",
        "",
        "## Counts",
        "",
        f"| Surface | Count |",
        f"|---------|-------|",
        f"| REST operations | {data['counts']['rest']} |",
        f"| gRPC RPCs | {data['counts']['grpc']} |",
        f"| MQTT channel definitions | {data['counts']['mqtt']} |",
        f"| Worker processes | {data['counts']['workers']} |",
        "",
        "### REST by surface",
        "",
    ]
    for k, v in sorted(data["rest_by_surface"].items()):
        lines.append(f"- **{k}**: {v}")
    lines.extend(["", "### REST by audience", ""])
    for k, v in sorted(data["rest_by_audience"].items()):
        lines.append(f"- **{k}**: {v}")
    lines.extend(
        [
            "",
            "## REST (from OpenAPI)",
            "",
            "| Method | Path | Audience | Auth | Surface | operationId |",
            "|--------|------|----------|------|---------|-------------|",
        ]
    )
    for r in data["rest"][:50]:
        lines.append(
            f"| {r['method']} | `{r['path']}` | {r['audience']} | {r['auth']} | {r['surface']} | {r['operationId']} |"
        )
    if len(data["rest"]) > 50:
        lines.append(f"| … | *{len(data['rest']) - 50} more* | | | | |")
    lines.extend(
        [
            "",
            "Full list: `build/reports/api-grpc-mqtt-full-inventory.json`",
            "",
            "## gRPC",
            "",
            "| Package | Service | RPC | Surface |",
            "|---------|---------|-----|---------|",
        ]
    )
    for g in data["grpc"]:
        lines.append(f"| {g['package']} | {g['service']} | {g['method']} | {g['surface']} |")
    lines.extend(["", "## MQTT", "", "| Channel | Direction | Layout | Surface |", "|---------|-----------|--------|---------|"])
    for m in data["mqtt"]:
        lines.append(f"| `{m['channel']}` | {m['direction']} | {m['layout']} | {m['surface']} |")
    lines.extend(["", "## Workers", ""])
    for w in data["workers"]:
        lines.append(f"- **{w['name']}** (`{w['entry']}`): {w['description']}")
    return "\n".join(lines) + "\n"


def main() -> None:
    rest = load_rest_from_swagger()
    grpc = parse_proto_dir(PROTO_MACHINE, "avf.machine.v1")
    grpc += parse_proto_dir(PROTO_INTERNAL, "avf.internal.v1")
    mqtt = load_mqtt_from_topics_go()
    workers = load_workers()
    rest_by_surface: dict[str, int] = {}
    rest_by_audience: dict[str, int] = {}
    for r in rest:
        rest_by_surface[r["surface"]] = rest_by_surface.get(r["surface"], 0) + 1
        rest_by_audience[r["audience"]] = rest_by_audience.get(r["audience"], 0) + 1
    payload = {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "note": "run from tools/generate_market_readiness_inventory.py",
        "counts": {
            "rest": len(rest),
            "grpc": len(grpc),
            "mqtt": len(mqtt),
            "workers": len(workers),
        },
        "rest_by_surface": rest_by_surface,
        "rest_by_audience": rest_by_audience,
        "rest": rest,
        "grpc": grpc,
        "mqtt": mqtt,
        "workers": workers,
    }
    OUT_JSON.parent.mkdir(parents=True, exist_ok=True)
    OUT_MD.parent.mkdir(parents=True, exist_ok=True)
    OUT_JSON.write_text(json.dumps(payload, indent=2), encoding="utf-8")
    OUT_MD.write_text(render_md(payload), encoding="utf-8")
    print(f"Wrote {OUT_MD}")
    print(f"Wrote {OUT_JSON}")
    print(f"REST={len(rest)} gRPC={len(grpc)} MQTT={len(mqtt)} workers={len(workers)}")


if __name__ == "__main__":
    main()
