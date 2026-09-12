# Multi-Layout E2E Scenario (Plan Section CA)

Staging checklist for fleet canary rollout. Run against a staging machine with feature flags enabled per machine target.

## Prerequisites

1. Apply API migrations `00031`, `00032`, `00033`.
2. Enable flags on staging canary machine:
   - `multi_layout_enabled`
   - `snapshot_history_ingest_enabled`
   - `periodic_snapshot_capture_enabled`
   - `hardware_reconcile_on_activate_enabled`
3. Disable `immediate_planogram_publish_enabled` on canary after local-first verification.

## Scenario

1. **Web:** Create machine → verify `Layout 1` appears via `GET /v1/admin/machines/{id}/layouts`.
2. **Android install:** Bootstrap → `GetMachineLayoutLibrary` returns Layout 1 as active.
3. **Technician edit:** Change slot assignment locally → UI shows "Saved locally", no immediate HTTP publish.
4. **Offline 3h:** Power off network → edit slots → periodic snapshot outbox accumulates PENDING rows.
5. **Reconnect:** WorkManager uploads batch via `ReportLayoutSnapshotBatch` → history rows append on server.
6. **Web history:** Open layout history page → PERIODIC_30M rows paginate.
7. **Second layout:** Web POST new layout → Android shows layout dropdown → select inactive layout → banner + APPLY LAYOUT.
8. **TCN apply:** APPLY runs ALL SINGLE + merge journal → `AckLayoutActivation` sets reported active layout.
9. **Rollback test:** Disable flags → legacy `ReportLocalLayout` + immediate publish paths still function.

## Physical UAT (Plan Section CB)

On TCN test machine capture TX/RX logs for:

- ALL SINGLE command (0xCA family)
- Deterministic merge pairs sorted by physical lane
- Vend busy gate before materialize

## Fleet canary

1. Enable `multi_layout_enabled` at 5% canary target.
2. Monitor `layout_snapshot_*` ingest metrics and pending outbox age.
3. Expand to 100% after 7 days without P0 regressions.
