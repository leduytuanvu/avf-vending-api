# Stuck `vending` repair — machine AVF000195 (`01a0a7e5-3c68-7895-b526-bcb6504bccfb`)

Field log **2026-10-18** (device `com.avf.vending.tcn`): physical vend succeeded but
`GRPC_CONFIRM_VEND_ERROR` / `idempotency_payload_mismatch` left orders in `vending` on the portal.

## App fix (deploy first)

Install a build containing:

- `PaymentBillCashRailPolicy` + unified vend confirm evidence (`VendLineCommerceEvidenceStore`)
- `PaymentViewModel.cashPaymentReady` wired to `observeCashRailReady()`
- Vend idempotency recovery on `idempotency_payload_mismatch` + outbox rekey on `vend_close_persistence_failed`

Deploy API build that binds `vend_hardware_evidence.raw` as UTF-8 jsonb text (fixes SQLSTATE 22P02 on confirm/report vend with evidence).

After deploy, prefer **outbox replay** (device online) before manual SQL when idempotency keys are not poisoned.

## Orders seen with confirm / idempotency errors in the captured log

Verify each row in Admin + logcat before repair (dispensed vs failed line).

```
01a117ed-f4aa-760d-b73b-654cad65c758
01a13e0a-481c-74a9-9b4f-a5151985435f
04f3a303-536d-4040-9f7d-b6af16305fdc
0972bb2a-2d73-3da5-9529-fb492f1893ab
0b3e95e8-6c88-3748-9e0e-e20df705b556
10f0a516-d342-33e6-88df-556759b29a23
23d53117-2ca2-4caf-b8ac-9327b5506abd
263b52b9-a10d-3741-a2d3-0918672b2cbf
27801fc2-fdc5-45e4-be50-cb5eacd0acef
4e7e62ef-03f8-4d50-b332-a92c0874c25b
518498a3-8661-4723-a6f4-34a5d4a4b2fb
776fb38f-11c5-3f59-8d80-988f226df4c2
983bfb66-b8d3-46c7-8baa-50ec92c6b928
a19f0423-865e-3e68-b615-f4bbae9cca50
adcf22ab-81f3-4a22-9417-2358719a90fa
b48e7bf3-0e97-4ca8-926d-70c6c658214b
e7f474b2-b93e-3d20-bad8-f15107d71b18
fd9b900c-01b9-44bd-972e-34206c237382
```

## Per-order SQL (physical success confirmed)

Use the generic template with dry-run first:

```bash
psql "$DATABASE_URL" -v dry_run=1 -v order_id='0972bb2a-2d73-3da5-9529-fb492f1893ab' \
  -f scripts/ops/repair_stuck_paid_vending_template.sql
```

Repeat for each order after evidence review. See also [`repair_stuck_vend_close_01a0f8b7.sql`](repair_stuck_vend_close_01a0f8b7.sql) for a single-order example.

## Idempotency poison

If outbox keeps failing with `idempotency_payload_mismatch` for the same correlation key after the app fix,
follow the cash-movement idempotency repair pattern in
[`repair_cash_movement_idempotency_poison_01a11014.md`](repair_cash_movement_idempotency_poison_01a11014.md)
(read-only audit first, then apply section 5 per runbook).

**Note:** This repo environment has no production DB access; run from an operator workstation.
