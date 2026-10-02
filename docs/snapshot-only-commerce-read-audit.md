# Snapshot-only commerce read audit

When `INVENTORY_AUTHORITATIVE_SERVER=false`, admin slot lists read from `machine_local_layout_mirror`. Commerce paths already consult the mirror for pricing provenance:

| Area | File | Behavior |
|------|------|----------|
| Quote pricing snapshot | `internal/app/commerce/quote_service.go` | Uses `LocalLayoutMirror` when present |
| Sale line checkout | `internal/app/commerce/sale_line_checkout.go` | Classifies pricing source from mirror |
| Pricing snapshot tests | `internal/app/commerce/quote_service_pricing_snapshot_test.go` | Mirror slot JSON fixtures |

Follow-up before full snapshot-only production:

- Verify QR/checkout on devices that no longer receive server planogram publish.
- Confirm vend inventory ledger still decrements on device; server `machine_slot_state` may lag.
- Re-run production readonly smoke (`scripts/e2e/run-phase-c-production-readonly-smoke.sh`) after enabling flags.

Rollout order: device snapshot capture → history UI → `INVENTORY_AUTHORITATIVE_SERVER=false` on API → `NEXT_PUBLIC_INVENTORY_SNAPSHOT_ONLY=true` on web → `technician_snapshot_only_enabled` on app.
