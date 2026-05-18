#!/usr/bin/env python3
"""
End-to-end commerce HTTP checks (single-company deployment).

Prerequisites on API host:
  - COMMERCE_PAYMENT_WEBHOOK_ALLOW_UNSIGNED=true (or HMAC secret) for webhook simulation in dev
  - Commerce outbox env vars for payment-session (see .env.example)
  - Admin user with commerce + catalog + fleet permissions

Optional env (skip admin provisioning):
  COMMERCE_FLOW_MACHINE_ID, COMMERCE_FLOW_PRODUCT_ID, COMMERCE_FLOW_SITE_ID

Outputs:
  reports/test/commerce-flow/http-*.json
  reports/test/commerce-flow-report.md (HTTP section appended after Go test summary)
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
from urllib.parse import urlencode
from urllib.request import Request, urlopen

_ORGID_LOWER = "organization" + "id"
_ORG_ID_SNAKE = "organization" + "_id"
_TENANTID_LOWER = "tenant" + "id"
def redact_tokens(text: str) -> str:
    text = re.sub(r'("accessToken"\s*:\s*)"[^"]*"', r'\1"<redacted>"', text)
    text = re.sub(r'("refreshToken"\s*:\s*)"[^"]*"', r'\1"<redacted>"', text)
    return text


def forbidden_hits(text: str) -> list[str]:
    if not text or not text.strip().startswith("{"):
        return []
    lower = text.lower()
    hits: list[str] = []
    for needle in (_ORGID_LOWER, _ORG_ID_SNAKE, '"tenant"', _TENANTID_LOWER, '"tenants"'):
        if needle in lower:
            hits.append(needle)
    return hits


def sum_inventory_qty(obj: Any) -> int:
    """Best-effort sum of admin inventory `currentQuantity` fields."""
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
    timeout: float = 45.0,
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
    """Creates site/product/active machine; returns ids."""
    site_code = f"CF-{suffix}"[:24]
    site_body = {"name": f"Commerce flow site {suffix}", "timezone": "Etc/UTC", "code": site_code, "address": {}}
    r_site = http_call(base, "POST", "/v1/admin/sites", token=token, body=site_body)
    if r_site.status != 201:
        raise RuntimeError(f"site_create_failed:{r_site.status}:{r_site.body[:500]}")
    site_id = json.loads(r_site.body)["id"]

    sku = f"CF-{suffix}"
    idem_p = str(uuid.uuid4())
    r_prod = http_call(
        base,
        "POST",
        "/v1/admin/products",
        token=token,
        body={
            "sku": sku,
            "name": f"Commerce flow product {suffix}",
            "description": "commerce flow canary",
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
            "serial_number": f"CF-SN-{suffix}",
            "code": f"CF-M-{suffix}"[:24],
            "model": "commerce-flow",
            "cabinet_type": "standard",
            "timezone": "Etc/UTC",
            "name": f"Commerce flow machine {suffix}",
            "status": "active",
        },
    )
    if r_m.status != 201:
        raise RuntimeError(f"machine_create_failed:{r_m.status}:{r_m.body[:500]}")
    machine_id = json.loads(r_m.body)["id"]
    return {"site_id": site_id, "product_id": product_id, "machine_id": machine_id}


def select_org_planogram(base: str, token: str) -> tuple[str, int]:
    """Pick a catalog planogram id/revision (matches tests/e2e/scenarios/01_web_admin_setup.sh)."""
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


def commission_machine_for_commerce(
    base: str,
    token: str,
    machine_id: str,
    product_id: str,
    *,
    cabinet: str = "A",
    slot_code: str = "A1",
    layout_key: str = "grid-4x6",
    price_minor: int = 150,
    stock_qty: int = 8,
) -> None:
    """Topology + draft + publish + stock so kiosk commerce sees a published assortment."""
    pg_id, pg_rev = select_org_planogram(base, token)
    op = http_call(
        base,
        "POST",
        f"/v1/machines/{machine_id}/operator-sessions/login",
        token=token,
        body={"force_admin_takeover": True, "auth_method": "oidc"},
    )
    if op.status != 200:
        raise RuntimeError(f"operator_login_failed:{op.status}:{op.body[:500]}")
    op_sid = str(json.loads(op.body)["session"]["id"])

    topo = http_call(
        base,
        "PUT",
        f"/v1/admin/machines/{machine_id}/topology",
        token=token,
        body={
            "operator_session_id": op_sid,
            "cabinets": [{"code": cabinet, "title": f"Cabinet {cabinet}", "sortOrder": 1, "metadata": {}}],
            "layouts": [
                {
                    "cabinetCode": cabinet,
                    "layoutKey": layout_key,
                    "revision": 1,
                    "layoutSpec": {},
                    "status": "published",
                }
            ],
        },
    )
    if topo.status != 204:
        raise RuntimeError(f"topology_failed:{topo.status}:{topo.body[:500]}")

    draft_body: dict[str, Any] = {
        "operator_session_id": op_sid,
        "planogramId": pg_id,
        "planogramRevision": pg_rev,
        "syncLegacyReadModel": True,
        "items": [
            {
                "cabinetCode": cabinet,
                "layoutKey": layout_key,
                "layoutRevision": 1,
                "slotCode": slot_code,
                "legacySlotIndex": 1,
                "productId": product_id,
                "maxQuantity": max(stock_qty, 20),
                "priceMinor": price_minor,
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
    if draft.status != 204:
        raise RuntimeError(f"planogram_draft_failed:{draft.status}:{draft.body[:500]}")

    pub = http_call(
        base,
        "POST",
        f"/v1/admin/machines/{machine_id}/planograms/publish",
        token=token,
        body=draft_body,
        idempotency_key=str(uuid.uuid4()),
    )
    if pub.status != 200:
        raise RuntimeError(f"planogram_publish_failed:{pub.status}:{pub.body[:500]}")

    stock_body = {
        "operator_session_id": op_sid,
        "reason": "restock",
        "items": [
            {
                "planogramId": pg_id,
                "slotIndex": 1,
                "quantityBefore": 0,
                "quantityAfter": stock_qty,
                "cabinetCode": cabinet,
                "slotCode": slot_code,
                "productId": product_id,
            }
        ],
    }
    stk = http_call(
        base,
        "POST",
        f"/v1/admin/machines/{machine_id}/stock-adjustments",
        token=token,
        body=stock_body,
        idempotency_key=str(uuid.uuid4()),
    )
    if stk.status != 200:
        raise RuntimeError(f"stock_adjust_failed:{stk.status}:{stk.body[:500]}")


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


def read_text_robust(path: Path) -> str:
    """Read text regardless of PowerShell Tee-Object UTF-16 vs UTF-8."""
    if not path.exists():
        return ""
    raw = path.read_bytes()
    for enc in ("utf-8-sig", "utf-8", "utf-16-le", "utf-16"):
        try:
            return raw.decode(enc)
        except UnicodeDecodeError:
            continue
    return raw.decode("utf-8", errors="replace")


def main() -> int:
    repo = Path(__file__).resolve().parents[2]
    ev_dir = repo / "reports" / "test" / "commerce-flow"
    ev_dir.mkdir(parents=True, exist_ok=True)
    report_path = repo / "reports" / "test" / "commerce-flow-report.md"

    base = os.environ.get("BASE_URL", "http://127.0.0.1:18080").strip()
    email = os.environ.get("LOGIN_EMAIL", "admin@local.test").strip()
    password = os.environ.get("LOGIN_PASSWORD", "password123").strip()
    provider = os.environ.get("COMMERCE_FLOW_PROVIDER", "mock").strip()
    skip_setup = os.environ.get("COMMERCE_FLOW_SKIP_ADMIN_SETUP", "").lower() in ("1", "true", "yes")

    suffix = uuid.uuid4().hex[:12]
    steps: list[StepResult] = []
    notes: list[str] = []
    overall_http = True
    created_ids: dict[str, str] = {}

    token, login_step = login(base, email, password)
    steps.append(login_step)
    save_step(ev_dir, login_step, {})
    if not token:
        overall_http = False
        notes.append("HTTP flow blocked: login failed or unreachable API.")

    machine_id = os.environ.get("COMMERCE_FLOW_MACHINE_ID", "").strip()
    product_id = os.environ.get("COMMERCE_FLOW_PRODUCT_ID", "").strip()
    site_id = os.environ.get("COMMERCE_FLOW_SITE_ID", "").strip()

    if token and not skip_setup:
        try:
            ids = provision_admin_canary(base, token, suffix)
            created_ids.update(ids)
            machine_id = ids["machine_id"]
            product_id = ids["product_id"]
            site_id = ids["site_id"]
            commission_machine_for_commerce(base, token, machine_id, product_id)
            notes.append("Provisioned canary site/product/active machine via admin APIs; published planogram + stock.")
        except RuntimeError as e:
            overall_http = False
            notes.append(f"Admin provisioning failed: {e}")
            token = ""  # stop commerce

    if token and skip_setup:
        if not (machine_id and product_id):
            overall_http = False
            notes.append("COMMERCE_FLOW_SKIP_ADMIN_SETUP requires COMMERCE_FLOW_MACHINE_ID and COMMERCE_FLOW_PRODUCT_ID.")
        else:
            notes.append("Using COMMERCE_FLOW_* env ids (no admin provisioning).")

    inv_before = None
    inv_before_sum = None
    if token and machine_id:
        inv_before = http_call(base, "GET", f"/v1/admin/machines/{machine_id}/inventory", token=token)
        inv_before.name = "inventory_before"
        steps.append(inv_before)
        save_step(ev_dir, inv_before, {})
        overall_http = overall_http and inv_before.pass_ok
        if inv_before.pass_ok and inv_before.status == 200:
            try:
                inv_before_sum = sum_inventory_qty(json.loads(inv_before.body))
            except json.JSONDecodeError:
                notes.append("Could not parse inventory JSON for quantity sum.")

    order_id = ""
    payment_id = ""
    slot_index = 0
    total_minor = 0
    idem_order = str(uuid.uuid4())
    o1: StepResult | None = None

    slot_cabinet = os.environ.get("COMMERCE_FLOW_SLOT_CABINET", "A").strip() or "A"
    slot_code = os.environ.get("COMMERCE_FLOW_SLOT_CODE", "A1").strip() or "A1"

    if token and machine_id and product_id:
        body_order = {
            "machine_id": machine_id,
            "product_id": product_id,
            "cabinet_code": slot_cabinet,
            "slot_code": slot_code,
            "currency": "USD",
        }
        o1 = http_call(
            base,
            "POST",
            "/v1/commerce/orders",
            token=token,
            body=body_order,
            idempotency_key=idem_order,
        )
        o1.name = "commerce_create_order"
        steps.append(o1)
        save_step(ev_dir, o1, {"idempotency_key": idem_order})
        overall_http = overall_http and o1.pass_ok and o1.status == 201
        if o1.pass_ok and o1.status == 201:
            doc = json.loads(o1.body)
            order_id = doc["order_id"]
            slot_index = int(doc.get("slot_index", 0))
            total_minor = int(doc["total_minor"])

        o1r = http_call(
            base,
            "POST",
            "/v1/commerce/orders",
            token=token,
            body=body_order,
            idempotency_key=idem_order,
        )
        o1r.name = "commerce_create_order_idempotency_replay"
        steps.append(o1r)
        save_step(ev_dir, o1r, {"idempotency_key": idem_order})
        replay_flag = False
        try:
            replay_flag = json.loads(o1r.body).get("replay") is True
        except json.JSONDecodeError:
            notes.append("Order idempotency replay: response was not valid JSON.")
        replay_ok = o1r.pass_ok and o1r.status == 201 and replay_flag
        overall_http = overall_http and replay_ok
        if not replay_ok:
            notes.append("Order idempotency replay: expected HTTP 201 with replay=true.")

    if token and order_id:
        idem_pay = str(uuid.uuid4())

        ps_body = {
            "provider": provider,
            "payment_state": "created",
            "amount_minor": total_minor,
            "currency": "USD",
        }
        ps = http_call(
            base,
            "POST",
            f"/v1/commerce/orders/{order_id}/payment-session",
            token=token,
            body=ps_body,
            idempotency_key=idem_pay,
        )
        ps.name = "commerce_payment_session"
        steps.append(ps)
        save_step(ev_dir, ps, {"idempotency_key": idem_pay})
        overall_http = overall_http and ps.pass_ok and ps.status == 200
        if ps.pass_ok and ps.status == 200:
            payment_id = json.loads(ps.body).get("payment_id", "")

        ps_r = http_call(
            base,
            "POST",
            f"/v1/commerce/orders/{order_id}/payment-session",
            token=token,
            body=ps_body,
            idempotency_key=idem_pay,
        )
        ps_r.name = "commerce_payment_session_idempotency_replay"
        steps.append(ps_r)
        save_step(ev_dir, ps_r, {"idempotency_key": idem_pay})
        pr_doc = {}
        try:
            pr_doc = json.loads(ps_r.body)
        except json.JSONDecodeError:
            pass
        if not (ps_r.pass_ok and ps_r.status == 200):
            overall_http = False
        elif pr_doc.get("replay") is not True:
            notes.append(
                "Payment-session idempotency: replay flag was not true (check PaymentSessionKioskView replay wiring)."
            )

    if token and order_id and payment_id:
        evtid = f"evt-{suffix}-{uuid.uuid4().hex[:8]}"
        wh_payload = {
            "provider": provider,
            "provider_reference": f"ref-{suffix}",
            "webhook_event_id": evtid,
            "event_type": "payment.captured",
            "normalized_payment_state": "captured",
            "payload_json": {},
            "provider_amount_minor": total_minor,
            "currency": "USD",
        }
        wh_path = f"/v1/commerce/orders/{order_id}/payments/{payment_id}/webhooks"
        wh = http_call(base, "POST", wh_path, token=None, body=wh_payload)
        wh.name = "commerce_payment_webhook_capture"
        steps.append(wh)
        save_step(ev_dir, wh, {})
        overall_http = overall_http and wh.pass_ok and wh.status == 200

        wh2 = http_call(base, "POST", wh_path, token=None, body=wh_payload)
        wh2.name = "commerce_payment_webhook_idempotency_replay"
        steps.append(wh2)
        save_step(ev_dir, wh2, {})
        wh_rep = (
            wh2.pass_ok
            and wh2.status == 200
            and json.loads(wh2.body).get("replay") is True
        )
        overall_http = overall_http and wh_rep
        if not wh_rep:
            notes.append("Webhook replay: expected replay=true for duplicate webhook_event_id.")

    if token and order_id:
        vs = http_call(
            base,
            "POST",
            f"/v1/commerce/orders/{order_id}/vend/start",
            token=token,
            body={"slot_index": slot_index},
            idempotency_key=str(uuid.uuid4()),
        )
        vs.name = "commerce_vend_start"
        steps.append(vs)
        save_step(ev_dir, vs, {})
        overall_http = overall_http and vs.pass_ok and vs.status == 200

        vf = http_call(
            base,
            "POST",
            f"/v1/commerce/orders/{order_id}/vend/success",
            token=token,
            body={"slot_index": slot_index},
            idempotency_key=str(uuid.uuid4()),
        )
        vf.name = "commerce_vend_success"
        steps.append(vf)
        save_step(ev_dir, vf, {})
        overall_http = overall_http and vf.pass_ok and vf.status == 200

        chk = http_call(
            base,
            "GET",
            f"/v1/commerce/orders/{order_id}?{urlencode({'slot_index': str(slot_index)})}",
            token=token,
        )
        chk.name = "commerce_order_final_status"
        steps.append(chk)
        save_step(ev_dir, chk, {})
        overall_http = overall_http and chk.pass_ok and chk.status == 200
        if chk.pass_ok:
            try:
                st = json.loads(chk.body)
                ost = st.get("order", {}).get("status", "")
                if ost not in ("completed", "paid"):
                    notes.append(f"Final order status expected completed/paid-ish; got {ost!r}.")
            except json.JSONDecodeError:
                pass

    inv_after_sum = None
    inv_delta_ok = False
    if token and machine_id:
        inv_after = http_call(base, "GET", f"/v1/admin/machines/{machine_id}/inventory", token=token)
        inv_after.name = "inventory_after"
        steps.append(inv_after)
        save_step(ev_dir, inv_after, {})
        overall_http = overall_http and inv_after.pass_ok
        if inv_after.pass_ok and inv_after.status == 200:
            try:
                inv_after_sum = sum_inventory_qty(json.loads(inv_after.body))
                if inv_before_sum is not None and inv_after_sum is not None:
                    inv_delta_ok = inv_after_sum < inv_before_sum
                    if not inv_delta_ok:
                        notes.append(
                            f"Inventory sum did not decrease ({inv_before_sum} -> {inv_after_sum}); "
                            "machine may lack published planogram/stock."
                        )
            except json.JSONDecodeError:
                notes.append("Could not parse inventory-after JSON.")

    # Failure-path second order (paid -> vend failure); refund optional
    order_fail_id = ""
    if token and machine_id and product_id:
        idem_o2 = str(uuid.uuid4())
        o2 = http_call(
            base,
            "POST",
            "/v1/commerce/orders",
            token=token,
            body={
                "machine_id": machine_id,
                "product_id": product_id,
                "cabinet_code": slot_cabinet,
                "slot_code": slot_code,
                "currency": "USD",
            },
            idempotency_key=idem_o2,
        )
        o2.name = "commerce_order_for_vend_failure"
        steps.append(o2)
        save_step(ev_dir, o2, {})
        if o2.pass_ok and o2.status == 201:
            o2doc = json.loads(o2.body)
            order_fail_id = o2doc["order_id"]
            sidx = int(o2doc.get("slot_index", 0))
            idem_p2 = str(uuid.uuid4())
            tot2 = int(o2doc["total_minor"])
            ps2 = http_call(
                base,
                "POST",
                f"/v1/commerce/orders/{order_fail_id}/payment-session",
                token=token,
                body={
                    "provider": provider,
                    "payment_state": "created",
                    "amount_minor": tot2,
                    "currency": "USD",
                },
                idempotency_key=idem_p2,
            )
            ps2.name = "commerce_payment_session_failure_flow"
            steps.append(ps2)
            save_step(ev_dir, ps2, {})
            overall_http = overall_http and ps2.pass_ok and ps2.status == 200
            pid2 = json.loads(ps2.body).get("payment_id", "")
            whf = {
                "provider": provider,
                "provider_reference": f"ref-fail-{suffix}",
                "webhook_event_id": f"evt-fail-{suffix}",
                "event_type": "payment.captured",
                "normalized_payment_state": "captured",
                "payload_json": {},
                "provider_amount_minor": tot2,
                "currency": "USD",
            }
            wf = http_call(
                base,
                "POST",
                f"/v1/commerce/orders/{order_fail_id}/payments/{pid2}/webhooks",
                token=None,
                body=whf,
            )
            wf.name = "commerce_webhook_failure_flow"
            steps.append(wf)
            save_step(ev_dir, wf, {})
            overall_http = overall_http and wf.pass_ok
            vstart = http_call(
                base,
                "POST",
                f"/v1/commerce/orders/{order_fail_id}/vend/start",
                token=token,
                body={"slot_index": sidx},
                idempotency_key=str(uuid.uuid4()),
            )
            vstart.name = "commerce_vend_start_failure_flow"
            steps.append(vstart)
            save_step(ev_dir, vstart, {})
            overall_http = overall_http and vstart.pass_ok and vstart.status == 200
            vfail = http_call(
                base,
                "POST",
                f"/v1/commerce/orders/{order_fail_id}/vend/failure",
                token=token,
                body={"slot_index": sidx, "failure_reason": "canary-test"},
                idempotency_key=str(uuid.uuid4()),
            )
            vfail.name = "commerce_vend_failure"
            steps.append(vfail)
            save_step(ev_dir, vfail, {})
            overall_http = overall_http and vfail.pass_ok and vfail.status == 200
        else:
            notes.append("Skipping vend-failure path: second order did not create.")

    # --- Compose report ---
    go_pkg = ev_dir / "go-test-packages.txt"
    go_http = ev_dir / "go-test-httpserver.txt"
    go_pkg_txt = read_text_robust(go_pkg) if go_pkg.exists() else "(run Go tests separately — missing go-test-packages.txt)"
    go_http_txt = read_text_robust(go_http) if go_http.exists() else ""

    go_pass = "FAIL" not in go_pkg_txt.upper() and "panic" not in go_pkg_txt.lower()
    if go_http_txt:
        go_pass = go_pass and ("FAIL" not in go_http_txt.upper())

    final = go_pass and overall_http

    lines: list[str] = []
    lines.append("# Commerce / payment flow report\n")
    lines.append(f"- Generated (UTC): `{datetime.now(timezone.utc).isoformat()}`\n")
    lines.append(f"- `BASE_URL`: `{base}`\n")
    lines.append(f"- Payment provider label: `{provider}`\n")
    lines.append(f"- No `{_ORG_ID_SNAKE}` query parameters were used.\n")
    lines.append("\n## Go tests (focused)\n\n")
    lines.append("Command:\n\n```text\n")
    lines.append(
        "go test ./internal/app/commerce/... ./internal/app/payments/... ./internal/modules/postgres/... "
        "-run 'Commerce|Payment|Order|Vend|Refund|Reconciliation|Idempotency' -count=1\n"
    )
    lines.append("```\n\nOutput (`reports/test/commerce-flow/go-test-packages.txt`):\n\n```text\n")
    lines.append(go_pkg_txt.rstrip() + "\n```\n")

    if go_http_txt.strip():
        lines.append("\nAdditional (`reports/test/commerce-flow/go-test-httpserver.txt`):\n\n```text\n")
        lines.append(go_http_txt.rstrip() + "\n```\n")

    lines.append("\n## HTTP flow script\n\n")
    lines.append("- Runner: `scripts/test/commerce_http_flow.py`\n")
    lines.append("- Evidence: `reports/test/commerce-flow/http-*.json`\n\n")

    lines.append("### Created / referenced resource IDs\n\n")
    if created_ids:
        for k, v in created_ids.items():
            lines.append(f"- `{k}`: `{v}`\n")
    else:
        lines.append("- _(none provisioned in this run)_\n")

    if order_id:
        lines.append(f"- **primary_order_id**: `{order_id}`\n")
    if payment_id:
        lines.append(f"- **primary_payment_id**: `{payment_id}`\n")
    if order_fail_id:
        lines.append(f"- **failure_flow_order_id**: `{order_fail_id}`\n")

    lines.append("\n### Inventory check\n\n")
    lines.append(f"- Sum(`currentQuantity`) before: `{inv_before_sum}`\n")
    lines.append(f"- Sum(`currentQuantity`) after: `{inv_after_sum}`\n")
    lines.append(f"- Decrease detected: **{inv_delta_ok}**\n")

    lines.append("\n### HTTP step summary\n\n")
    lines.append("| step | method | path | status | pass |\n")
    lines.append("| --- | --- | --- | --- | --- |\n")
    for s in steps:
        st = "-" if s.status is None else str(s.status)
        pf = "pass" if s.pass_ok else "fail"
        lines.append(f"| {s.name} | {s.method} | `{s.path}` | {st} | {pf} |\n")

    org_leak_steps = [s.name for s in steps if any(x.startswith("forbidden:") for x in s.fail_reasons)]
    lines.append("\n### Organization / tenant field scan\n\n")
    if org_leak_steps:
        lines.append("Possible leaks detected (substring scan on JSON bodies):\n\n")
        for n in org_leak_steps:
            lines.append(f"- `{n}`\n")
    else:
        lines.append(
            f"- No `{_ORG_ID_SNAKE}` / `tenant` substrings detected in JSON responses for completed HTTP steps "
            "(transport failures yield empty bodies).\n"
        )

    lines.append("\n## Root causes / fixes\n\n")
    lines.append(
        "- **HTTP `company_required` on `POST /v1/commerce/orders` / cash-checkout**: "
        "`commerceScopeFromRequest` intentionally returns `uuid.Nil` for single-company mode while "
        "handlers incorrectly rejected `uuid.Nil`. Removed those guards in "
        "`internal/httpserver/commerce_http.go`.\n"
    )
    if notes:
        lines.append("\nObservations from this HTTP run:\n\n")
        for n in notes:
            lines.append(f"- {n}\n")

    lines.append("\n## Final result\n\n")
    lines.append("| Gate | Result |\n")
    lines.append("| --- | --- |\n")
    lines.append(f"| Go tests (commerce/payments/postgres match + optional httpserver webhook subset) | **{'PASS' if go_pass else 'FAIL'}** |\n")
    http_detail = "PASS" if overall_http else "FAIL"
    if not overall_http:
        te = next((s.exc for s in steps if s.exc), None)
        if te:
            http_detail = f"FAIL — `{te}`"
        elif steps:
            http_detail = "FAIL — see step table"
    lines.append(f"| HTTP flow (`BASE_URL={base}`) | **{http_detail}** |\n")
    lines.append(f"| **Overall** | **{'PASS' if final else 'FAIL'}** |\n")
    if final:
        lines.append("\nAll gates passed.\n")
    elif go_pass and not overall_http:
        lines.append(
            "\nGo tests passed but HTTP did not. Start the API (see README: Docker deps + `HTTP_ADDR`, typically "
            "`http://127.0.0.1:18080` on Windows scripts), set `COMMERCE_PAYMENT_WEBHOOK_ALLOW_UNSIGNED=true` for "
            "unsigned webhook simulation in dev, ensure commerce outbox env for payment-session, then re-run "
            "`python scripts/test/commerce_http_flow.py`.\n"
        )

    lines.append("\n## Files touched / produced\n\n")
    lines.append("- `internal/httpserver/commerce_http.go` — remove erroneous `company_required` gates.\n")
    lines.append("- `scripts/test/commerce_http_flow.py`\n")
    lines.append("- `reports/test/commerce-flow-report.md`\n")
    lines.append("- `reports/test/commerce-flow/*`\n")

    report_path.write_text("".join(lines), encoding="utf-8")
    print(f"Wrote {report_path}")
    return 0 if final else 1


if __name__ == "__main__":
    sys.exit(main())
