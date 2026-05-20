"""Business-domain folder mapping for postman/generated REST collection."""
from __future__ import annotations

FOLDER_ORDER = [
    "00_Health_System",
    "01_Auth",
    "02_Admin_Accounts_RBAC",
    "03_Catalog_Categories_Brands_Tags",
    "04_Product_Media_Offline_Cache",
    "05_Products",
    "06_Sites_Regions",
    "07_Machines_Provisioning",
    "08_Machines_Runtime_Config",
    "09_Machines_Telemetry",
    "10_Inventory",
    "11_Planogram_Assortment",
    "12_Orders",
    "13_Payments",
    "14_Refunds_Disputes",
    "15_Promotions_PriceBooks",
    "16_Finance_Reconciliation",
    "17_Incidents_Diagnostics",
    "18_OTA_Rollout",
    "19_Audit_Logs",
    "20_Webhooks",
    "99_Utilities",
]


def assign_folder_business(path: str, method: str, tags: list) -> str:
    """Map OpenAPI path/tags to business-domain Postman folders."""
    p = path.lower()
    ts = {t.lower() for t in tags}
    tagl = " ".join(tags).lower()

    if any(x in p for x in ("/health/", "/version", "/metrics", "/swagger/")):
        return "00_Health_System"
    if "/v1/auth" in p or "auth" in ts:
        return "01_Auth"
    if (
        "/v1/admin/companies" in p
        or "/v1/admin/users" in p
        or "/v1/admin/roles" in p
        or "/v1/admin/invitations" in p
        or "rbac" in ts
        or "companies" in ts
    ):
        return "02_Admin_Accounts_RBAC"
    if (
        ("/categories" in p or "/brands" in p or "/tags" in p or "catalog" in ts)
        and "/products/" not in p
        and not p.endswith("/products")
    ):
        return "03_Catalog_Categories_Brands_Tags"
    if any(x in p for x in ("/media", "/offline", "/manifest")) or "offline cache" in tagl:
        return "04_Product_Media_Offline_Cache"
    if "/v1/admin/products" in p or ("products" in ts and "/v1/admin/" in p):
        return "05_Products"
    if "/v1/admin/sites" in p or "sites" in ts or "locations" in ts or "/regions" in p:
        return "06_Sites_Regions"
    if (
        "/v1/setup/" in p
        or "activation-codes" in p
        or "/claim" in p
        or "activation" in ts
        or ("/v1/admin/machines" in p and method.upper() == "POST" and "activation" in p)
    ):
        return "07_Machines_Provisioning"
    if (
        "/v1/admin/machines" in p
        or "/v1/machines/" in p
        or "fleet" in ts
        or "machine admin" in ts
    ):
        return "08_Machines_Runtime_Config"
    if "/v1/device/" in p or "telemetry" in ts or "device" in ts or "check-ins" in p:
        return "09_Machines_Telemetry"
    if "inventory" in p or "restock" in p or "fill" in p or "stock-" in p:
        return "10_Inventory"
    if "planogram" in p or "layout" in p or "topology" in p or "cabinet" in ts or "assortment" in p:
        return "11_Planogram_Assortment"
    if "/refunds" in p or "/vend/failure" in p or ("refund" in p and "vend" in tagl):
        return "14_Refunds_Disputes"
    if "/v1/commerce" in p or "commerce" in ts or "checkout" in ts or "/orders" in p or "/vend/" in p:
        return "12_Orders"
    if "payment" in p or "qr" in p or "psp" in tagl or "/cash" in p:
        return "13_Payments"
    if "promotion" in p or "price-book" in p or "pricebook" in p or "price book" in tagl:
        return "15_Promotions_PriceBooks"
    if "reconciliation" in p or "finance" in ts or "/v1/admin/reports" in p or "reporting" in ts:
        return "16_Finance_Reconciliation"
    if "incident" in p or "diagnostic" in p:
        return "17_Incidents_Diagnostics"
    if "ota" in p or "rollout" in p or "firmware" in p:
        return "18_OTA_Rollout"
    if "/v1/admin/audit" in p or "/v1/admin/security" in p or "audit" in ts:
        return "19_Audit_Logs"
    if "webhook" in p or "/v1/partner" in p or "payment provider" in ts:
        return "20_Webhooks"
    if "command" in p or "operator" in p:
        return "99_Utilities"
    return "99_Utilities"


def flow_name(folder: str) -> str:
    """Derive a short flow label from folder name."""
    return folder.split("_", 1)[-1].replace("_", " ") if "_" in folder else folder
