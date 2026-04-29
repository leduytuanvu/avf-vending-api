# Machine Commerce gRPC

`avf.machine.v1.MachineCommerceService` is the vending app sale surface. It is protected by Machine JWT (`typ=machine`, `aud=avf-machine-grpc`) and reuses the existing commerce application service, payment webhook path, and inventory ledger.

## Cash Sale

1. `CreateOrder` creates a pending order and vend session for the authenticated machine.
2. `ConfirmCashPayment` records a captured cash payment and marks the order paid.
3. `StartVend` moves the vend session to `in_progress`.
4. `ConfirmVendSuccess` finalizes the order and applies the inventory sale movement exactly once.
5. `ReportVendFailure` finalizes the vend as failed and reports whether local cash refund or provider refund handling is required.

## QR / PSP Sale

1. `CreateOrder` creates the pending order.
2. `CreatePaymentSession` creates the payment record and transactional outbox event for the PSP integration. It can return an opaque QR payload or HTTPS payment URL, never image bytes.
3. The payment provider still calls the existing REST webhook with HMAC/idempotency validation.
4. The machine polls `GetOrder` or `GetOrderStatus` until payment is captured and the order is paid.
5. `StartVend` is accepted only after the order is paid, then `ConfirmVendSuccess` or `ReportVendFailure` completes the flow.

## Safety Rules

All mutations require `MutationContext.IdempotencyKey` for vending writes. A repeated mutation with the same key returns `Replay=true` when semantics match; mismatched payloads follow the ledger `ABORTED` / `FAILED_PRECONDITION` rules.

`ConfirmVendSuccess` finalizes **`orders`** (to **`completed`**), **`vend_sessions`** (to **`success`**), deduplicated **inventory ledger** rows (`machine_slot_state` decrement), and an **`order_timelines`** audit event in **one Postgres transaction**. Successful dispense inventory suppression uses the literal **`{MutationContext.IdempotencyKey}:vend_sale_inventory`**, matching the `/v1/device/.../vend-results` suffix when both surfaces share the same mutation idempotency semantics. The legacy **`ReportVendSuccess`** RPC remains an alias for **`ConfirmVendSuccess`**.

Failures after captured payment finalize **`orders`** to **`failed`** and **`vend_sessions`** to **`failed`**, enqueue compensation/workflow cues when configured, expose **`RefundRequired`** / **`LocalCashRefundRequired`** booleans at the RPC layer, and append **`commerce_vend_dispense_failed`** timeline metadata (distinct from adding a hypothetical **`orders.status=refund_required`** literal—which the current schema does not model).

State checks block unpaid, expired, canceled, refunded, or otherwise terminal orders from vend start/finalization. Machine scope is always taken from the Machine JWT; request machine echoes and order ownership must match the token.

## Error Mapping

`INVALID_ARGUMENT` is used for malformed requests, `UNAUTHENTICATED` for missing/invalid Machine JWT, `PERMISSION_DENIED` for cross-machine access, `NOT_FOUND` for missing commerce rows, `ABORTED` for idempotency conflicts, `FAILED_PRECONDITION` for illegal state transitions, `RESOURCE_EXHAUSTED` for stock/rate limits, and `INTERNAL` only for unexpected failures.
