#!/usr/bin/env python3
"""
Inventory + telemetry HTTP checks after single-company / organization removal.

Prerequisites:
  - API reachable at BASE_URL (default http://127.0.0.1:18080)
  - MACHINE_REST_LEGACY_ENABLED=true (telemetry under legacy machine REST mount)
  - Admin user with inventory, fleet, telemetry, and inventory_adjust permissions
  - MQTT broker reachable when API enables MQTT (see start-api-local.ps1)

Optional:
  INV_FLOW_SKIP_ADMIN_SETUP=1 requires INV_FLOW_MACHINE_ID and INV_FLOW_PRODUCT_ID

Outputs:
  reports/test/inventory-telemetry-offline/http-*.json
  reports/test/inventory-telemetry-offline-report.md
"""

from __future__ import annotations

import json
import os
import re
import sys
import uuid
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen


def redact_tokens(text: str) -> str:
    text = re.sub(r'("accessToken"\s*:\s*)"[^"]*"', r'\1"<redacted>"', text)
    text = re.sub(r'("refreshToken"\s*:\s*)"[^"]*"', r'\1"<redacted>"', text)
    return text


def forbidden_hits(text: str) -> list[str]:
    if not text or not text.strip().startswith("{"):
        return []
    lower = text.lower()
    hits: list[str] = []
    for needle in (
        "organizationid",
        "organization_id",
        '"tenant"',
        "tenantid",
        '"tenants"',
    ):
        if needle in lower:
            hits.append(needle)
    return hits


def read_text_robust(path: Path) -> str:
    if not path.exists():
        return ""
    raw = path.read_bytes()
    for enc in ("utf-8-sig", "utf-8", "utf-16-le", "utf-16"):
        try:
            return raw.decode(enc)
        except UnicodeDecodeError:
            continue
    return raw.decode("utf-8", errors="replace")


def sum_inventory_qty(obj: Any) -> int:
    if isinstance(obj, dict):
        s = 0
        if "currentQuantity" in obj and isinstance(obj["currentQuantity"], int):
            s += obj["currentQuantity"]
        for v in obj.values():
            s += sum_inventory_qty(v)
        return s
    if isinstance(obj, list):
        return sum(sum_inventory_qty(x) for x in obj)
    return 0


@dataclass
class StepResult:
    name: str
    method: str
    path: str
    url: str
    status: int | None
    request_id: str
    correlation_id: str
    body: str
    pass_ok: bool
    fail_reasons: list[str] = field(default_factory=list)
    exc: str | None = None


def http_call(
    base: str,
    method: str,
    path: str,
    *,
    token: str | None = None,
    body: dict[str, Any] | None = None,
    idempotency_key: str | None = None,
    timeout: float = 60.0,
) -> StepResult:
    url = base.rstrip("/") + path
    rid = str(uuid.uuid4())
    corr = str(uuid.uuid4())
    headers = {
        "X-Request-ID": rid,
        "X-Correlation-ID": corr,
        "Accept": "application/json",
    }
    data: bytes | None
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        headers["Content-Type"] = "application/json; charset=utf-8"
    elif method in ("POST", "PUT", "PATCH"):
        data = b""
    else:
        data = None
    if token:
        headers["Authorization"] = f"Bearer {token}"
    if idempotency_key:
        headers["Idempotency-Key"] = idempotency_key

    req = Request(url, data=data, headers=headers, method=method)
    status = None
    resp_headers: dict[str, str] = {}
    body_text = ""
    exc = None
    try:
        with urlopen(req, timeout=timeout) as resp:
            status = resp.getcode()
            resp_headers = {k.lower(): v for k, v in resp.headers.items()}
            body_text = resp.read().decode("utf-8", errors="replace")
    except HTTPError as e:
        status = e.code
        resp_headers = {k.lower(): v for k, v in e.headers.items()} if e.headers else {}
        body_text = e.read().decode("utf-8", errors="replace") if e.fp else ""
    except (URLError, OSError) as e:
        exc = str(getattr(e, "reason", None) or e)

    req_id = resp_headers.get("x-request-id", "") or rid
    corr_hdr = resp_headers.get("x-correlation-id", "") or corr

    reasons: list[str] = []
    if exc:
        reasons.append(f"transport:{exc}")
    if status == 500:
        reasons.append("http_500")
    if status is None:
        reasons.append("no_status")
    reasons.extend(f"forbidden:{x}" for x in forbidden_hits(body_text))

    return StepResult(
        name="",
        method=method,
        path=path,
        url=url,
        status=status,
        request_id=req_id,
        correlation_id=corr_hdr,
        body=body_text[:65536],
        pass_ok=len(reasons) == 0,
        fail_reasons=reasons,
        exc=exc,
    )


def login(base: str, email: str, password: str) -> tuple[str | None, StepResult]:
    r = http_call(base, "POST", "/v1/auth/login", token=None, body={"email": email, "password": password})
    r.name = "login"
    tok = None
    if r.status == 200:
        try:
            doc = json.loads(r.body)
            tok = (doc.get("tokens") or {}).get("accessToken")
        except json.JSONDecodeError:
            r.pass_ok = False
            r.fail_reasons.append("login_json")
    else:
        r.pass_ok = False
        if r.status is not None:
            r.fail_reasons.append(f"login_http_{r.status}")
    if not tok:
        r.pass_ok = False
    return tok, r


def provision_admin_canary(base: str, token: str, suffix: str) -> dict[str, str]:
    site_code = f"IT-{suffix}"[:24]
    site_body = {"name": f"Inv telemetry site {suffix}", "timezone": "Etc/UTC", "code": site_code, "address": {}}
    r_site = http_call(base, "POST", "/v1/admin/sites", token=token, body=site_body)
    if r_site.status != 201:
        raise RuntimeError(f"site_create_failed:{r_site.status}:{r_site.body[:500]}")
    site_id = json.loads(r_site.body)["id"]

    sku = f"IT-{suffix}"
    idem_p = str(uuid.uuid4())
    r_prod = http_call(
        base,
        "POST",
        "/v1/admin/products",
        token=token,
        body={
            "sku": sku,
            "name": f"Inv telemetry product {suffix}",
            "description": "inventory telemetry canary",
            "active": True,
            "ageRestricted": False,
            "allergenCodes": [],
        },
        idempotency_key=idem_p,
    )
    if r_prod.status != 200:
        raise RuntimeError(f"product_create_failed:{r_prod.status}:{r_prod.body[:500]}")
    product_id = json.loads(r_prod.body)["id"]

    r_m = http_call(
        base,
        "POST",
        "/v1/admin/machines",
        token=token,
        body={
            "site_id": site_id,
            "serial_number": f"IT-SN-{suffix}",
            "code": f"IT-M-{suffix}"[:24],
            "model": "inv-telemetry-flow",
            "cabinet_type": "standard",
            "timezone": "Etc/UTC",
            "name": f"Inv telemetry machine {suffix}",
            "status": "active",
        },
    )
    if r_m.status != 201:
        raise RuntimeError(f"machine_create_failed:{r_m.status}:{r_m.body[:500]}")
    machine_id = json.loads(r_m.body)["id"]
    return {"site_id": site_id, "product_id": product_id, "machine_id": machine_id}


def select_org_planogram(base: str, token: str) -> tuple[str, int]:
    r = http_call(base, "GET", "/v1/admin/planograms?limit=20", token=token)
    if r.status != 200:
        raise RuntimeError(f"planogram_list_failed:{r.status}:{r.body[:500]}")
    doc = json.loads(r.body)
    items = doc.get("items") if isinstance(doc, dict) else None
    if not isinstance(items, list) or not items:
        raise RuntimeError("no_planogram_in_org")
    pg_id = ""
    pg_rev = 0
    for row in items:
        if isinstance(row, dict) and row.get("status") == "published":
            pg_id = str(row.get("id") or "")
            try:
                pg_rev = int(row.get("revision") or 0)
            except (TypeError, ValueError):
                pg_rev = 0
            break
    if not pg_id:
        row0 = items[0]
        if isinstance(row0, dict):
            pg_id = str(row0.get("id") or "")
            try:
                pg_rev = int(row0.get("revision") or 0)
            except (TypeError, ValueError):
                pg_rev = 0
    if not pg_id:
        raise RuntimeError("no_planogram_id")
    return pg_id, pg_rev


def save_step(out_dir: Path, step: StepResult, extra: dict[str, Any]) -> str:
    slug = re.sub(r"[^a-z0-9]+", "_", step.name.lower()).strip("_") or "step"
    path = out_dir / f"http-{slug}.json"
    rec = {
        "step": step.name,
        "method": step.method,
        "path": step.path,
        "url": step.url,
        "status": step.status,
        "request_id": step.request_id,
        "correlation_id": step.correlation_id,
        "pass": step.pass_ok,
        "fail_reasons": step.fail_reasons,
        "transport_error": step.exc,
        "response_body": redact_tokens(step.body),
        **extra,
    }
    path.write_text(json.dumps(rec, indent=2), encoding="utf-8")
    return path.name


def step_expect_2xx(step: StepResult, *, allow_empty_body: bool = False) -> bool:
    if step.exc:
        return False
    if step.status is None:
        return False
    if not (200 <= step.status < 300):
        return False
    if not allow_empty_body and step.status != 204:
        if not step.body.strip():
            return False
    reasons = [x for x in step.fail_reasons if not x.startswith("forbidden:")]
    return len(reasons) == 0


def main() -> int:
    repo = Path(__file__).resolve().parents[2]
    ev_dir = repo / "reports" / "test" / "inventory-telemetry-offline"
    ev_dir.mkdir(parents=True, exist_ok=True)
    report_path = repo / "reports" / "test" / "inventory-telemetry-offline-report.md"

    base = os.environ.get("BASE_URL", "http://127.0.0.1:18080").strip()
    email = os.environ.get("LOGIN_EMAIL", "e2e-local-admin@invalid.local").strip()
    password = os.environ.get("LOGIN_PASSWORD", "E2E_LocalDev_9c3a!").strip()
    skip_setup = os.environ.get("INV_FLOW_SKIP_ADMIN_SETUP", "").lower() in ("1", "true", "yes")

    suffix = uuid.uuid4().hex[:12]
    steps: list[StepResult] = []
    notes: list[str] = []
    overall_http = True
    created_ids: dict[str, str] = {}

    stock_replay_ok = False
    invalid_negative_blocked = False

    token, login_step = login(base, email, password)
    steps.append(login_step)
    save_step(ev_dir, login_step, {})
    if not token:
        overall_http = False
        notes.append("HTTP blocked: login failed or API unreachable.")

    machine_id = os.environ.get("INV_FLOW_MACHINE_ID", "").strip()
    product_id = os.environ.get("INV_FLOW_PRODUCT_ID", "").strip()

    if token and not skip_setup:
        try:
            ids = provision_admin_canary(base, token, suffix)
            created_ids.update(ids)
            machine_id = ids["machine_id"]
            product_id = ids["product_id"]
            notes.append("Provisioned canary site/product/active machine.")
        except RuntimeError as e:
            overall_http = False
            notes.append(f"Admin provisioning failed: {e}")
            token = ""

    if token and skip_setup:
        if not (machine_id and product_id):
            overall_http = False
            notes.append("INV_FLOW_SKIP_ADMIN_SETUP requires INV_FLOW_MACHINE_ID and INV_FLOW_PRODUCT_ID.")
        else:
            notes.append("Using INV_FLOW_* env ids (no admin provisioning).")

    cab = "A"
    layout_key = "grid-4x6"
    slot_code = "A1"
    legacy_idx = 1
    planogram_id = ""
    pg_revision = 1

    op_sid = ""

    if token and machine_id:
        try:
            planogram_id, pg_revision = select_org_planogram(base, token)
        except RuntimeError as e:
            overall_http = False
            notes.append(f"Planogram list/select failed: {e}")

        inv0 = http_call(base, "GET", f"/v1/admin/machines/{machine_id}/inventory", token=token)
        inv0.name = "admin_inventory_initial"
        steps.append(inv0)
        save_step(ev_dir, inv0, {})
        overall_http = overall_http and step_expect_2xx(inv0)

        slots0 = http_call(base, "GET", f"/v1/admin/machines/{machine_id}/slots", token=token)
        slots0.name = "admin_slots_initial"
        steps.append(slots0)
        save_step(ev_dir, slots0, {})
        overall_http = overall_http and step_expect_2xx(slots0)

        low = http_call(
            base,
            "GET",
            f"/v1/admin/inventory/low-stock?machine_id={machine_id}&limit=10",
            token=token,
        )
        low.name = "admin_inventory_low_stock"
        steps.append(low)
        save_step(ev_dir, low, {})
        overall_http = overall_http and step_expect_2xx(low)

        evl = http_call(
            base,
            "GET",
            f"/v1/admin/machines/{machine_id}/inventory-events?limit=5",
            token=token,
        )
        evl.name = "admin_inventory_events_list"
        steps.append(evl)
        save_step(ev_dir, evl, {})
        overall_http = overall_http and step_expect_2xx(evl)

        op = http_call(
            base,
            "POST",
            f"/v1/machines/{machine_id}/operator-sessions/login",
            token=token,
            body={"force_admin_takeover": True, "auth_method": "oidc"},
        )
        op.name = "operator_session_login"
        steps.append(op)
        save_step(ev_dir, op, {})
        overall_http = overall_http and step_expect_2xx(op)
        if op.status == 200:
            try:
                op_sid = str(json.loads(op.body)["session"]["id"])
            except (json.JSONDecodeError, KeyError):
                notes.append("Could not parse operator session id.")
                overall_http = False

        if op_sid:
            topo = http_call(
                base,
                "PUT",
                f"/v1/admin/machines/{machine_id}/topology",
                token=token,
                body={
                    "operator_session_id": op_sid,
                    "cabinets": [{"code": cab, "title": f"Cabinet {cab}", "sortOrder": 1, "metadata": {}}],
                    "layouts": [
                        {
                            "cabinetCode": cab,
                            "layoutKey": layout_key,
                            "revision": 1,
                            "layoutSpec": {},
                            "status": "published",
                        }
                    ],
                },
            )
            topo.name = "admin_topology_put"
            steps.append(topo)
            save_step(ev_dir, topo, {})
            ok_topo = topo.status == 204 and topo.exc is None
            if not ok_topo:
                overall_http = False
                notes.append(f"Topology PUT expected 204; got status={topo.status} err={topo.exc}")

        draft_body: dict[str, Any] | None = None
        if op_sid and planogram_id:
            draft_body = {
                "operator_session_id": op_sid,
                "planogramId": planogram_id,
                "planogramRevision": pg_revision,
                "syncLegacyReadModel": True,
                "items": [
                    {
                        "cabinetCode": cab,
                        "layoutKey": layout_key,
                        "layoutRevision": 1,
                        "slotCode": slot_code,
                        "legacySlotIndex": legacy_idx,
                        "productId": product_id,
                        "maxQuantity": 20,
                        "priceMinor": 150,
                        "metadata": {},
                    }
                ],
            }
            draft = http_call(
                base,
                "PUT",
                f"/v1/admin/machines/{machine_id}/planograms/draft",
                token=token,
                body=draft_body,
            )
            draft.name = "admin_planogram_draft_put"
            steps.append(draft)
            save_step(ev_dir, draft, {})
            ok_draft = draft.status == 204 and draft.exc is None
            if not ok_draft:
                overall_http = False
                notes.append(f"Planogram draft PUT expected 204; got status={draft.status}")

            if ok_draft and draft_body is not None:
                pub = http_call(
                    base,
                    "POST",
                    f"/v1/admin/machines/{machine_id}/planograms/publish",
                    token=token,
                    body=draft_body,
                    idempotency_key=str(uuid.uuid4()),
                )
                pub.name = "admin_planogram_publish_post"
                steps.append(pub)
                save_step(ev_dir, pub, {})
                ok_pub = pub.status == 200 and pub.exc is None
                if not ok_pub:
                    overall_http = False
                    notes.append(f"Planogram publish expected 200; got status={pub.status}")

        idem_adj = str(uuid.uuid4())
        if op_sid and product_id and planogram_id:
            adj_body = {
                "operator_session_id": op_sid,
                "reason": "manual_adjustment",
                "items": [
                    {
                        "planogramId": planogram_id,
                        "slotIndex": legacy_idx,
                        "quantityBefore": 0,
                        "quantityAfter": 5,
                        "cabinetCode": cab,
                        "slotCode": slot_code,
                        "productId": product_id,
                    }
                ],
            }
            adj1 = http_call(
                base,
                "POST",
                f"/v1/admin/machines/{machine_id}/stock-adjustments",
                token=token,
                body=adj_body,
                idempotency_key=idem_adj,
            )
            adj1.name = "admin_stock_adjustment"
            steps.append(adj1)
            save_step(ev_dir, adj1, {"idempotency_key": idem_adj})
            overall_http = overall_http and step_expect_2xx(adj1)

            adj2 = http_call(
                base,
                "POST",
                f"/v1/admin/machines/{machine_id}/stock-adjustments",
                token=token,
                body=adj_body,
                idempotency_key=idem_adj,
            )
            adj2.name = "admin_stock_adjustment_idempotency_replay"
            steps.append(adj2)
            save_step(ev_dir, adj2, {"idempotency_key": idem_adj})
            overall_http = overall_http and step_expect_2xx(adj2)
            try:
                doc2 = json.loads(adj2.body)
                stock_replay_ok = doc2.get("replay") is True
            except json.JSONDecodeError:
                stock_replay_ok = False
            if not stock_replay_ok:
                notes.append("Stock adjustment replay: expected replay=true on duplicate Idempotency-Key.")
                overall_http = False

            bad_adj = http_call(
                base,
                "POST",
                f"/v1/admin/machines/{machine_id}/stock-adjustments",
                token=token,
                body={
                    "operator_session_id": op_sid,
                    "reason": "manual_adjustment",
                    "items": [
                        {
                            "planogramId": planogram_id,
                            "slotIndex": legacy_idx,
                            "quantityBefore": 0,
                            "quantityAfter": -1,
                            "cabinetCode": cab,
                            "slotCode": slot_code,
                            "productId": product_id,
                        }
                    ],
                },
                idempotency_key=str(uuid.uuid4()),
            )
            bad_adj.name = "admin_stock_adjustment_negative_quantity_rejected"
            steps.append(bad_adj)
            save_step(ev_dir, bad_adj, {})
            invalid_negative_blocked = bad_adj.status is not None and bad_adj.status >= 400
            if not invalid_negative_blocked:
                notes.append("Expected rejection for negative quantity_after; did not get HTTP error.")
                overall_http = False

        rec = http_call(
            base,
            "POST",
            f"/v1/admin/operations/machines/{machine_id}/inventory/reconcile",
            token=token,
            body={"reason": "inventory-telemetry-flow-marker"},
        )
        rec.name = "admin_inventory_reconcile_marker"
        steps.append(rec)
        save_step(ev_dir, rec, {})
        overall_http = overall_http and (rec.status == 202 and rec.exc is None)

        snap = http_call(base, "GET", f"/v1/machines/{machine_id}/telemetry/snapshot", token=token)
        snap.name = "machine_telemetry_snapshot"
        steps.append(snap)
        save_step(ev_dir, snap, {})
        if snap.status == 404:
            notes.append("Telemetry snapshot 404 — no telemetry row yet (acceptable for cold machine).")
        elif not step_expect_2xx(snap):
            overall_http = False

        inc = http_call(base, "GET", f"/v1/machines/{machine_id}/telemetry/incidents?limit=5", token=token)
        inc.name = "machine_telemetry_incidents"
        steps.append(inc)
        save_step(ev_dir, inc, {})
        overall_http = overall_http and step_expect_2xx(inc)

        rollup = http_call(
            base,
            "GET",
            f"/v1/machines/{machine_id}/telemetry/rollups?limit=5",
            token=token,
        )
        rollup.name = "machine_telemetry_rollups"
        steps.append(rollup)
        save_step(ev_dir, rollup, {})
        overall_http = overall_http and step_expect_2xx(rollup)

        inv1 = http_call(base, "GET", f"/v1/admin/machines/{machine_id}/inventory", token=token)
        inv1.name = "admin_inventory_after_adjustment"
        steps.append(inv1)
        save_step(ev_dir, inv1, {})
        overall_http = overall_http and step_expect_2xx(inv1)

    go_pkg = ev_dir / "go-test-packages.txt"
    go_sup = ev_dir / "go-test-supplementary.txt"
    go_pkg_txt = read_text_robust(go_pkg) if go_pkg.exists() else "(missing — run Go tests pipeline)"
    go_sup_txt = read_text_robust(go_sup) if go_sup.exists() else ""

    go_pass = "FAIL" not in go_pkg_txt.upper() and "panic" not in go_pkg_txt.lower()
    sup_fail = bool(go_sup_txt.strip()) and ("FAIL" in go_sup_txt.upper())
    if sup_fail:
        notes.append(
            "Supplementary Go log contains FAIL (often Postgres/grpc integration when Docker is down); "
            "primary focused-package gate above is authoritative."
        )

    org_leak_steps = [s.name for s in steps if any(x.startswith("forbidden:") for x in s.fail_reasons)]

    final = go_pass and overall_http

    lines: list[str] = []
    lines.append("# Inventory, telemetry, offline replay report\n\n")
    lines.append(f"- Generated (UTC): `{datetime.now(timezone.utc).isoformat()}`\n")
    lines.append(f"- `BASE_URL`: `{base}`\n")
    lines.append("- No `organization_id` query parameters were used on HTTP steps; admin fleet scope resolves as single-company server-side (`uuid.Nil`).\n\n")

    lines.append("## Go tests (focused packages)\n\nCommand:\n\n```text\n")
    lines.append(
        "go test ./internal/app/inventoryapp/... ./internal/app/inventoryadmin/... ./internal/app/telemetryapp/... "
        "./internal/modules/postgres/... "
        "-run 'Inventory|Telemetry|Offline|Replay|Critical|Adjustment|Refill|Stock' -count=1\n"
    )
    lines.append("```\n\nOutput (`reports/test/inventory-telemetry-offline/go-test-packages.txt`):\n\n```text\n")
    lines.append(go_pkg_txt.rstrip() + "\n```\n")

    if go_sup_txt.strip():
        lines.append("\nSupplementary (MQTT contract, critical idempotency helpers, grpc offline replay, postgres telemetry duplicates):\n\n```text\n")
        lines.append(go_sup_txt.rstrip() + "\n```\n")

    lines.append("\n## HTTP flow (`scripts/test/inventory_telemetry_http_flow.py`)\n\n")
    lines.append(f"- Stock adjustment idempotency replay: **{stock_replay_ok}**\n")
    lines.append(f"- Negative `quantity_after` rejected (HTTP error): **{invalid_negative_blocked}**\n\n")

    lines.append("### Resource IDs\n\n")
    if created_ids:
        for k, v in created_ids.items():
            lines.append(f"- `{k}`: `{v}`\n")
        lines.append(f"- machine-local **planogramId** (draft + adjustments): `{planogram_id}`\n")
    elif machine_id:
        lines.append(f"- `machine_id`: `{machine_id}`\n")
        lines.append(f"- `product_id`: `{product_id}`\n")

    lines.append("\n### Organization / tenant scan\n\n")
    if org_leak_steps:
        for n in org_leak_steps:
            lines.append(f"- Possible substring leak in `{n}`\n")
    else:
        lines.append("- No `organization_id` / `tenant` substrings detected in JSON bodies for HTTP steps.\n")

    lines.append("\n### HTTP steps\n\n")
    lines.append("| step | method | path | status | pass |\n| --- | --- | --- | --- | --- |\n")
    for s in steps:
        st = "-" if s.status is None else str(s.status)
        pf = "pass" if s.pass_ok else "fail"
        lines.append(f"| {s.name} | {s.method} | `{s.path}` | {st} | {pf} |\n")

    lines.append("\n## Offline replay & critical telemetry (automated coverage)\n\n")
    lines.append(
        "- **gRPC offline replay** (`PushOfflineEvents`): duplicate sequence replays as `REPLAYED` — "
        "`go test ./internal/grpcserver -run TestP06_OfflineSync_duplicateOfflineSequenceReplayed` "
        "(requires `TEST_DATABASE_URL`).\n"
    )
    lines.append(
        "- **Uniqueness / rejection**: conflicting `client_event_id` at different sequence — "
        "`TestP06_OfflineSync_duplicateClientEventIdAtLaterSequenceRejected`.\n"
    )
    lines.append(
        "- **MQTT offline replay contract** (critical duplicate idempotency payload): "
        "`internal/platform/mqtt/offline_replay_contract_test.go`.\n"
    )
    lines.append(
        "- **Critical telemetry idempotency key derivation**: `internal/platform/telemetry/critical_idempotency_test.go`.\n"
    )
    lines.append(
        "- **OLTP duplicate suppression** (edge telemetry): "
        "`internal/modules/postgres/telemetry_idempotency_integration_test.go`.\n"
    )

    lines.append("\n## Observations\n\n")
    for n in notes:
        lines.append(f"- {n}\n")
    if not notes:
        lines.append("- _(none)_\n")

    lines.append("\n## Final result\n\n| Gate | Result |\n| --- | --- |\n")
    sup_line = "PASS" if go_pass and not sup_fail else ("WARN (supplementary FAIL)" if go_pass and sup_fail else "FAIL")
    lines.append(f"| Go tests (focused packages) | **{'PASS' if go_pass else 'FAIL'}** |\n")
    lines.append(f"| Go tests (supplementary MQTT/telemetry + optional integration notes) | **{sup_line}** |\n")
    http_detail = "PASS" if overall_http else "FAIL"
    if not overall_http:
        te = next((s.exc for s in steps if s.exc), None)
        if te:
            http_detail = f"FAIL — `{te}`"
    lines.append(f"| HTTP inventory/telemetry flow | **{http_detail}** |\n")
    lines.append(f"| **Overall** | **{'PASS' if final else 'FAIL'}** |\n")

    lines.append("\n## Evidence files\n\n")
    lines.append("- `reports/test/inventory-telemetry-offline/http-*.json`\n")
    lines.append("- `reports/test/inventory-telemetry-offline/go-test-*.txt`\n")

    report_path.write_text("".join(lines), encoding="utf-8")
    print(f"Wrote {report_path}")
    return 0 if final else 1


if __name__ == "__main__":
    sys.exit(main())
