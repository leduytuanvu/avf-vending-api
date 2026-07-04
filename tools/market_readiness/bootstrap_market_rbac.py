#!/usr/bin/env python3
"""Bootstrap technician + multi-machine assignments for market readiness RBAC."""

from __future__ import annotations

import os
import secrets
import sys
import uuid
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "production_full_test"))
sys.path.insert(0, str(Path(__file__).resolve().parent))

from market_common import bundle_dir, setup_market_env, write_json  # noqa: E402
from bootstrap_test_data import admin_patch, admin_post, bootstrap, claim_activation, login  # noqa: E402
from entity_registry import EntityRegistry  # noqa: E402


def create_technician(base_url: str, admin_token: str, prefix: str) -> tuple[str, str, str]:
    """Create fleet technician entity + login user; returns (technician_entity_id, token, email)."""
    email = os.environ.get("PROD_TEST_TECHNICIAN_EMAIL", "").strip()
    password = os.environ.get("PROD_TEST_TECHNICIAN_PASSWORD", "").strip()
    suffix = prefix.replace("AVF-MARKET-READY-", "")[:24]
    if not email:
        email = f"{suffix}.tech.{uuid.uuid4().hex[:6]}@avf-market.test"
    if not password:
        password = secrets.token_urlsafe(16)
    display_name = f"{prefix} Technician"
    st, tech_body = admin_post(
        base_url,
        admin_token,
        "/v1/admin/technicians",
        {"display_name": display_name, "email": email},
    )
    if st not in (200, 201):
        raise RuntimeError(f"technician entity create failed {st}: {tech_body}")
    tech_entity_id = str(tech_body.get("id") or "")
    if not tech_entity_id:
        raise RuntimeError(f"technician entity missing id: {tech_body}")
    st, user_body = admin_post(
        base_url,
        admin_token,
        "/v1/admin/users",
        {"email": email, "password": password, "roles": ["technician"], "status": "active"},
    )
    if st not in (200, 201):
        raise RuntimeError(f"technician user create failed {st}: {user_body}")
    tech_token = login(base_url, email, password)
    return tech_entity_id, tech_token, email


def create_machine(base_url: str, admin_token: str, prefix: str, site_id: str, label: str) -> dict:
    from market_common import production_machine_code  # noqa: E402

    code = production_machine_code()
    serial = f"{prefix}-{label}-SN-{uuid.uuid4().hex[:6]}"
    payload = {
        "name": f"{prefix} Machine {label}",
        "code": code,
        "siteId": site_id,
        "serialNumber": serial,
        "model": "AVF-MARKET-TEST",
        "status": "draft",
        "timezone": "UTC",
        "cabinetType": "ambient",
    }
    st, machine = admin_post(base_url, admin_token, "/v1/admin/machines", payload)
    if st not in (200, 201) or not machine.get("id"):
        raise RuntimeError(f"machine {label} create failed {st}: {machine}")
    mid = machine["id"]
    admin_patch(base_url, admin_token, f"/v1/admin/machines/{mid}", {"status": "active"})
    st, act = admin_post(base_url, admin_token, f"/v1/admin/machines/{mid}/activation-codes", {})
    if st not in (200, 201):
        raise RuntimeError(f"activation code failed for {label}: {st}")
    code_val = act.get("code") or act.get("activationCode") or act.get("plaintextCode")
    claim = claim_activation(base_url, str(code_val), serial)
    return {
        "machineId": mid,
        "machineCode": code,
        "serialNumber": serial,
        "machineToken": claim.get("accessToken") or claim.get("machineToken") or "",
    }


def assign_technician(base_url: str, admin_token: str, machine_id: str, technician_id: str) -> None:
    payload = {"technician_id": technician_id, "role": "field_service"}
    st, body = admin_post(base_url, admin_token, f"/v1/admin/machines/{machine_id}/technicians", payload)
    if st not in (200, 201, 409):
        raise RuntimeError(f"technician assign failed {st}: {body}")


ROLE_MAP = {
    "viewer": ["viewer"],
    "finance": ["finance_admin"],
    "catalog_manager": ["catalog_manager"],
    "disabled": ["viewer"],
}


def bootstrap_security_roles(base_url: str, admin_token: str, prefix: str, reg: EntityRegistry) -> None:
    suffix = prefix.replace("AVF-MARKET-READY-", "")[:24]
    for auth_kind, roles in ROLE_MAP.items():
        email = f"{suffix}.{auth_kind}.{uuid.uuid4().hex[:6]}@avf-market.test"
        password = secrets.token_urlsafe(16)
        st, body = admin_post(
            base_url,
            admin_token,
            "/v1/admin/users",
            {"email": email, "password": password, "roles": roles, "status": "active"},
        )
        if st not in (200, 201):
            raise RuntimeError(f"security role {auth_kind} create failed {st}: {body}")
        uid = str(body.get("accountId") or body.get("id") or "")
        token = login(base_url, email, password)
        if auth_kind == "disabled" and uid:
            admin_post(base_url, admin_token, f"/v1/admin/users/{uid}/disable", {})
            admin_post(base_url, admin_token, f"/v1/admin/users/{uid}/revoke-sessions", {})
        if token:
            reg.set(f"token_{auth_kind}", token, entity_type="token")


def bootstrap_market_rbac(base_url: str, *, skip_extra_machines: bool = False) -> EntityRegistry:
    setup_market_env()
    reg = bootstrap(base_url)
    subst = reg.as_substitution_map()
    admin_token = subst["adminAccessToken"]
    prefix = reg.data.get("prefix", "market")
    site_id = subst["siteId"]

    tech_id, tech_token, tech_email = create_technician(base_url, admin_token, prefix)
    reg.set("technicianId", tech_id, entity_type="technician")
    reg.set("token_technician", tech_token, entity_type="token")
    reg.set("technicianEmail", tech_email, entity_type="config")
    bootstrap_security_roles(base_url, admin_token, prefix, reg)

    machine_a = subst["machineId"]
    assign_technician(base_url, admin_token, machine_a, tech_id)

    if not skip_extra_machines:
        machine_b = create_machine(base_url, admin_token, prefix, site_id, "B")
        assign_technician(base_url, admin_token, machine_b["machineId"], tech_id)
        reg.set("machineIdB", machine_b["machineId"], entity_type="machine")
        reg.set("machineTokenB", machine_b["machineToken"], entity_type="token")

        machine_c = create_machine(base_url, admin_token, prefix, site_id, "C")
        reg.set("machineIdC", machine_c["machineId"], entity_type="machine")
        reg.set("machineTokenC", machine_c["machineToken"], entity_type="token")

    reg.save()
    return reg


def main() -> int:
    import argparse

    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default=os.environ.get("BASE_URL", "https://api.ldtv.dev"))
    parser.add_argument("--pass-only", action="store_true", help="Security roles only after existing bootstrap")
    args = parser.parse_args()
    base = args.base_url
    try:
        if args.pass_only:
            setup_market_env()
            reg = EntityRegistry()
            subst = reg.as_substitution_map()
            if not subst.get("adminAccessToken"):
                reg = bootstrap(base)
                subst = reg.as_substitution_map()
            prefix = reg.data.get("prefix", "market")
            bootstrap_security_roles(base, subst["adminAccessToken"], prefix, reg)
            tech_id = reg.get("technicianId")
            if not tech_id:
                tech_id, tech_token, tech_email = create_technician(base, subst["adminAccessToken"], prefix)
                reg.set("technicianId", tech_id, entity_type="technician")
                reg.set("token_technician", tech_token, entity_type="token")
                assign_technician(base, subst["adminAccessToken"], subst["machineId"], tech_id)
            reg.save()
            print("Market RBAC pass-only bootstrap OK")
            return 0
        reg = bootstrap_market_rbac(base)
        out = bundle_dir()
        write_json(
            out / "MARKET_RBAC_BOOTSTRAP.json",
            {"prefix": reg.data.get("prefix"), "technicianId": reg.get("technicianId")},
        )
        print(f"Market RBAC bootstrap OK technician={reg.get('technicianId')}")
        return 0
    except Exception as exc:
        print(f"bootstrap_market_rbac failed: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
