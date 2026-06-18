#!/usr/bin/env python3
import argparse, json, re, sys
from pathlib import Path

def read_json(p):
    if not p.is_file(): return None
    try: return json.loads(p.read_text(encoding="utf-8"))
    except json.JSONDecodeError: return None

def pick(o, *keys, default=None):
    for k in keys:
        if isinstance(o, dict) and k in o and o[k] is not None: return o[k]
    return default

def parse_range(spec):
    m = re.match(r"^(\d+)-(\d+)$", spec.strip())
    if not m: raise ValueError(spec)
    return int(m.group(1)), int(m.group(2))

def slot_codes(cab, a, b):
    # TCN pilot: cabinet CAB-A uses legacy slot codes A1..An (not CAB-A1).
    prefix = "A" if str(cab).upper().startswith("CAB-") else str(cab)
    return [f"{prefix}{i}" for i in range(a, b+1)]

def media_ok(item, ready):
    pm = (item or {}).get("primaryMedia") or (item or {}).get("primary_media") or {}
    if isinstance(pm, dict):
        for k in ("displayUrl","display_url","thumbUrl","thumb_url","url"):
            if pm.get(k): return True, "primary_media_url"
    if ready: return True, "bootstrap_primary_media_ready"
    return False, "missing"

def bootstrap_slots(b):
    out = {}
    for cab in (b.get("topology") or {}).get("cabinets") or []:
        cc = str(pick(cab,"code",default="") or "")
        sort = pick(cab,"sortOrder","sort_order")
        meta = cab.get("metadata") if isinstance(cab.get("metadata"), dict) else {}
        lk = str((meta or {}).get("layoutKey") or (meta or {}).get("layout_key") or "")
        for i, s in enumerate(cab.get("slots") or []):
            sc = str(pick(s,"slotCode","slot_code",default="") or "")
            if sc: out[sc] = {**s, "cabinet_code": cc, "cabinet_index": i, "cabinet_sort_order": sort, "layout_key": lk}
    return out

def catalog_items(resp):
    if not resp: return []
    return list((resp.get("snapshot") or {}).get("items") or [])

def inv_by_code(doc):
    out = {}
    if not isinstance(doc, dict): return out
    rows = doc.get("items") or doc.get("slots") or doc.get("inventory") or []
    for r in rows:
        sc = str(pick(r,"slotCode","slot_code",default="") or "")
        q = int(pick(r,"totalQuantity","currentQuantity","quantity","qty",default=0) or 0)
        if sc: out[sc] = max(out.get(sc,0), q)
    return out

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--artifact-dir", required=True)
    ap.add_argument("--machine-id", required=True)
    ap.add_argument("--hardware-profile", default="")
    ap.add_argument("--destructive-cabinet", default="A")
    ap.add_argument("--destructive-slot-indexes", default="1-10")
    a = ap.parse_args()
    art = Path(a.artifact_dir); raw = art/"raw"
    failures = []
    bootstrap = read_json(raw/"grpc-bootstrap.response.json") or {}
    cat_all = read_json(raw/"grpc-catalog-all.response.json")
    cat_sel = read_json(raw/"grpc-catalog-sellable.response.json")
    plano = read_json(raw/"grpc-planogram.response.json")
    version = read_json(raw/"version.json") or {}
    inv_adm = read_json(raw/"admin-inventory.body")
    machine = bootstrap.get("machine") or {}
    hw = a.hardware_profile.strip() or str(pick(machine,"hardwareProfileId","hardware_profile_id",default="unknown") or "unknown")
    cash_only = str((version.get("payment_runtime") or {}).get("payment_mode","")) == "cash_only"
    if not cash_only: failures.append("cash_only_runtime_disabled")
    bslots = bootstrap_slots(bootstrap)
    products_ready = {str(pick(p,"productId","product_id",default="")): bool(pick(p,"primaryMediaReady","primary_media_ready",default=False)) for p in (bootstrap.get("catalog") or {}).get("products") or []}
    currency = str(pick((cat_sel or {}).get("snapshot") or {}, "currency", default="") or pick((cat_all or {}).get("snapshot") or {}, "currency", default="") or "")
    inv = inv_by_code(inv_adm or {})
    sell_codes = {str(pick(i,"slotCode","slot_code",default="")) for i in catalog_items(cat_sel) if pick(i,"isAvailable","is_available",default=True)}
    rows = {}
    for item in catalog_items(cat_all) or catalog_items(cat_sel):
        sc = str(pick(item,"slotCode","slot_code",default="") or "")
        if not sc: continue
        bs = bslots.get(sc, {})
        cc = str(pick(item,"cabinetCode","cabinet_code",default="") or bs.get("cabinet_code") or "")
        si = pick(item,"slotIndex","slot_index", default=bs.get("slot_index"))
        pid = str(pick(item,"productId","product_id",default="") or pick(bs,"productId","product_id",default="") or "")
        qty = int(pick(item,"availableQuantity","available_quantity",default=0) or inv.get(sc,0) or pick(bs,"maxQuantity","max_quantity",default=0) or 0)
        mok, ms = media_ok(item, products_ready.get(pid))
        hidden = str(pick(item,"unavailableReason","unavailable_reason",default="") or "")
        avail = bool(pick(item,"isAvailable","is_available",default=False))
        sellable = (sc in sell_codes or avail) and not hidden and pid and qty>0 and int(pick(item,"priceMinor","price_minor",default=0) or 0)>0 and mok
        rows[sc] = dict(machine_id=a.machine_id, cabinet_code=cc, cabinet_index=bs.get("cabinet_index"), cabinet_sort_order=bs.get("cabinet_sort_order"), layout_key=bs.get("layout_key",""), layout_revision=None, slot_code=sc, slot_index=si, motor_index=str(pick(bs,"machineSlotLayoutId","machine_slot_layout_id",default="") or (f"slot_index:{si}" if si is not None else "")), product_id=pid, product_name=str(pick(item,"name",default="") or ""), product_status="active" if avail else "mapped", price_minor=int(pick(item,"priceMinor","price_minor",default=0) or 0), currency=currency, inventory_quantity=qty, media_status=ms, enabled=avail, sellable=sellable, hidden_reason=hidden, hardware_profile=hw, source="grpc_catalog")
    for sc, bs in bslots.items():
        if sc in rows: continue
        cc = str(bs.get("cabinet_code") or "")
        si = pick(bs,"slotIndex","slot_index")
        pid = str(pick(bs,"productId","product_id",default="") or "")
        qty = int(inv.get(sc,0) or pick(bs,"maxQuantity","max_quantity",default=0) or 0)
        mok, ms = media_ok({}, products_ready.get(pid))
        rows[sc] = dict(machine_id=a.machine_id, cabinet_code=cc, cabinet_index=bs.get("cabinet_index"), cabinet_sort_order=bs.get("cabinet_sort_order"), layout_key=bs.get("layout_key",""), layout_revision=None, slot_code=sc, slot_index=si, motor_index=str(pick(bs,"machineSlotLayoutId","machine_slot_layout_id",default="") or f"slot_index:{si}"), product_id=pid, product_name=str(pick(bs,"productName","product_name",default="") or ""), product_status="mapped" if pid else "missing", price_minor=int(pick(bs,"priceMinor","price_minor",default=0) or 0), currency=currency, inventory_quantity=qty, media_status=ms, enabled=bool(pid), sellable=bool(pid and qty>0), hidden_reason="not_in_app_catalog", hardware_profile=hw, source="grpc_bootstrap")
    sellable_rows = [r for r in rows.values() if r["sellable"]]
    app_items = catalog_items(cat_sel)
    s, e = parse_range(a.destructive_slot_indexes)
    pilot = set(slot_codes(a.destructive_cabinet, s, e))
    dest = {sc: rows[sc] for sc in pilot if sc in rows}
    if not app_items and not sellable_rows: failures.append("app_facing_sellable_catalog_empty")
    for r in sellable_rows:
        if not r["cabinet_code"]: failures.append(f"sellable_missing_cabinet_code:{r['slot_code']}")
        if r["slot_index"] is None and not r["motor_index"]: failures.append(f"sellable_missing_slot_index_or_motor:{r['slot_code']}")
        if r["price_minor"] <= 0: failures.append(f"sellable_invalid_price:{r['slot_code']}")
        if r["inventory_quantity"] <= 0: failures.append(f"sellable_inventory_zero:{r['slot_code']}")
        if r["media_status"] == "missing": failures.append(f"sellable_missing_media:{r['slot_code']}")
    for code in sorted(pilot):
        if code not in dest: failures.append(f"pilot_slots_missing_from_topology:{code}"); continue
        d = dest[code]
        if d.get("hidden_reason"): failures.append(f"pilot_hidden_reason:{code}:{d['hidden_reason']}")
        if not d.get("product_id"): failures.append(f"pilot_missing_product:{code}")
        if d.get("price_minor",0) <= 0: failures.append(f"pilot_invalid_price:{code}")
        if d.get("inventory_quantity",0) <= 0: failures.append(f"pilot_inventory_zero:{code}")
        if d.get("media_status") == "missing": failures.append(f"pilot_missing_media:{code}")
    extra = sorted(set(dest) - pilot)
    if extra: failures.append(f"destructive_scope_extra_slots:{','.join(extra)}")
    topo = bootstrap.get("topology") or {}
    cabinets_out = []
    for cab in topo.get("cabinets") or []:
        meta = cab.get("metadata") if isinstance(cab.get("metadata"), dict) else {}
        cabinets_out.append({"code": pick(cab,"code"), "title": pick(cab,"title"), "sort_order": pick(cab,"sortOrder","sort_order"), "layout_key": (meta or {}).get("layoutKey"), "slot_count": len(cab.get("slots") or [])})
    (art/"machine-topology.json").write_text(json.dumps({"machine_id": a.machine_id, "hardware_profile_id": hw, "topology": topo, "bootstrap_machine": machine}, indent=2), encoding="utf-8")
    (art/"cabinets.json").write_text(json.dumps({"cabinets": cabinets_out}, indent=2), encoding="utf-8")
    (art/"published-planogram.json").write_text(json.dumps({"published_planogram_version_id": pick(bootstrap,"publishedPlanogramVersionId","published_planogram_version_id"), "published_planogram_version_no": pick(bootstrap,"publishedPlanogramVersionNo","published_planogram_version_no"), "grpc_planogram_slot_count": len((plano or {}).get("slots") or [])}, indent=2), encoding="utf-8")
    (art/"app-facing-catalog-all.json").write_text(json.dumps({"machine_id": a.machine_id, "item_count": len(app_items), "items": app_items}, indent=2), encoding="utf-8")
    cols = list(next(iter(rows.values())).keys()) if rows else []
    def wtsv(path, rs):
        path.write_text("\t".join(cols)+"\n"+"\n".join("\t".join(str(r.get(c,"")) for c in cols) for r in rs)+"\n", encoding="utf-8")
    if cols:
        wtsv(art/"sellable-slots-all-machine.tsv", sellable_rows)
        wtsv(art/"destructive-test-slots.tsv", [dest[c] for c in sorted(pilot) if c in dest])
        wtsv(art/"hidden-reasons.tsv", [r for r in rows.values() if r.get("hidden_reason")])
    pv_id = pick(bootstrap, "publishedPlanogramVersionId", "published_planogram_version_id")
    pv_no = pick(bootstrap, "publishedPlanogramVersionNo", "published_planogram_version_no")
    grpc_plano_slots = len((plano or {}).get("slots") or [])
    summary = {
        "verdict": "PASS" if not failures else "BLOCKED",
        "machine_id": a.machine_id,
        "hardware_profile": hw,
        "destructive_scope": {"cabinet": a.destructive_cabinet, "slot_indexes": a.destructive_slot_indexes, "slot_codes": sorted(pilot)},
        "sellable_slots_all_machine_count": len(sellable_rows),
        "app_facing_catalog_item_count": len(app_items),
        "destructive_test_slots_count": len(dest),
        "hidden_reason_count": sum(1 for r in rows.values() if r.get("hidden_reason")),
        "cash_only_runtime": cash_only,
        "published_planogram_version_id": pv_id,
        "published_planogram_version_no": pv_no,
        "grpc_planogram_slot_count": grpc_plano_slots,
        "failures": failures,
    }
    (art/"diagnose-summary.json").write_text(json.dumps(summary, indent=2), encoding="utf-8")
    print(json.dumps(summary, indent=2))
    return 0 if summary["verdict"]=="PASS" else 2

if __name__ == "__main__":
    sys.exit(main())
