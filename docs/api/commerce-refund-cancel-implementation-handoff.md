# Commerce cancel, refund, and vend-failure compensation — implementation handoff

**Implementation status:** **Shipped** — `internal/httpserver/commerce_http.go` (cancel, refunds, list/get refund routes).

**Goal (original):** Mounted HTTP APIs and state transitions for **unpaid cancel**, **refund** (idempotent), **list/get refund**, and **vend-failure** paths including **cash** vs **online** compensation.

## Existing code to extend

- **Order statuses** already include `cancelled` ([`orders.status` CHECK](../../db/schema/01_platform.sql)).
- **Payments:** `created` | `authorized` | `captured` | `failed` | `refunded`.
- **Refunds table** exists: `requested` | `processing` | `completed` | `failed` ([`refunds.state`](../../db/schema/01_platform.sql)) — **no `idempotency_key` column** today; add via migration + partial unique index `(order_id, idempotency_key)` where key non-empty.
- **Advisory refund rules:** [`EvaluateRefundEligibility`](../../internal/app/commerce/service.go) — promote to enforcement in `CreateRefund` / auto-refund path.
- **Vend failure after capture:** [`FinalizeOrderAfterVend`](../../internal/app/commerce/service.go) sets order `failed`, optionally starts Temporal [`StartVendFailureAfterPaymentSuccess`](../../internal/app/commerce/service.go) when enabled — **HTTP response must** surface `refund_state` / `local_cash_refund_required` per spec below.

## Required routes (Chi + OpenAPI)

| Method | Path | Notes |
|--------|------|--------|
| POST | `/v1/commerce/orders/{orderId}/cancel` | Idempotency header per [`idempotency.go`](../../internal/httpserver/idempotency.go) |
| POST | `/v1/commerce/orders/{orderId}/refunds` | |
| GET | `/v1/commerce/orders/{orderId}/refunds` | List |
| GET | `/v1/commerce/orders/{orderId}/refunds/{refundId}` | Get one |

Add to [`tools/build_openapi.py`](../../tools/build_openapi.py) **`REQUIRED_OPERATIONS`** and `DocOp*` stubs in [`swagger_operations.go`](../../internal/httpserver/swagger_operations.go).

## Auth / tenant

- Reuse commerce group middleware: Bearer + org resolution ([`commerce_http.go`](../../internal/httpserver/commerce_http.go) `RequireOrganizationScope` + `tenantOrgID`).
- **Machine token:** when tenant-bound commerce lands, require order’s `machine_id` ∈ token `machine_ids` (see commerce access handoffs). Until then, document org-scoped JWT only or align with activation machine tokens for kiosk.

## API ↔ persistence mapping

### Client `refund_state` enum

Target product enum:

`not_required` | `required` | `pending` | `succeeded` | `failed_retryable` | `failed_terminal`

**Map from DB `refunds.state` + payment:**

| DB / situation | API `refund_state` |
|----------------|-------------------|
| No refund row; payment not captured | `not_required` |
| No refund row; captured + vend failed + cash path needs local settle | `required` (plus `local_cash_refund_required`) |
| `requested` | `pending` |
| `processing` | `pending` |
| `completed` | `succeeded` |
| `failed` + retry policy TBD | `failed_retryable` or `failed_terminal` (persist distinction in `refunds.metadata` or extend CHECK constraint) |

**Migration option:** widen `refunds.state` CHECK to include `failed_retryable`, `failed_terminal` **or** keep DB minimal and map `failed` + `metadata->>'failure_class'`.

### Cancel unpaid order

**Rules:**

- Allowed only if latest payment is **missing** or state in `created` | `authorized` | `failed` (not `captured` / `refunded`).
- If payment `captured` → **409** or **422** with message to use refund flow (not cancel).
- Transition order → `cancelled`; idempotent replay on same `Idempotency-Key` + same body → `replay: true`; same key different body → **409** (reuse pattern from other commerce writes).

**Persistence:** implement `CancelOrder` in commerce `Service` + `CommerceLifecycleStore` methods (`UpdateOrderStatus` to `cancelled` if valid).

### Create refund

**Rules:**

- Reject if no payment or payment not `captured`.
- Reject if sum(existing refunds `amount_minor` where state not in terminal failed) + new amount > `payment.amount_minor` (or order total — define single source).
- **Idempotency:** unique `(order_id, idempotency_key)` on `refunds` after migration; replay → `replay: true`, same refund id.
- Insert refund `requested` or `processing` if provider call is sync; return **201** or **202** accordingly.

**Provider:** if no PSP refund API wired, persist refund `requested` and enqueue outbox / workflow; document **webhook** can move to `completed` / `failed` (existing `payment_provider_events` patterns).

**P1.2 reconciliation hardening:** failures such as **paid + vend failed**, **paid + no vend result after timeout**, **refund stuck pending**, **duplicate PSP deliveries**, **provider mismatch**, **amount/currency mismatch vs persisted payment**, and **late webhooks after terminal order/payment state** create or refresh rows in **`commerce_reconciliation_cases`** (including optional **`machine_id`** for fleet context). Cases are **never silently auto-refunded** by the reconciler loop alone—operators use **`POST .../commerce/reconciliation/{caseId}/request-refund`** (writes **`refunds`** via **`CreateRefund`** with deterministic idempotency) and **`POST .../resolve`** to close cases with audit.

### List / get refunds

- sqlc queries: `ListRefundsByOrder`, `GetRefundByOrderAndID` scoped by `organization_id` join on `orders`.

## Vend failure (`POST .../vend/failure`)

After [`FinalizeOrderAfterVend`](../../internal/app/commerce/service.go) with `terminalVendState=failed`:

1. **Duplicate vend failure:** idempotent — second call returns same order/vend terminal state without second refund ([`UpdateVendSessionState`](../../internal/app/commerce/service.go) early return when already terminal).
2. **Auto refund:** if `pay.State == "captured"` and `pay.Provider != "cash"` (or non-cash family):
   - Call internal **`CreateRefund`** once with **deterministic idempotency key** e.g. `vend-failure:{orderId}:{slotIndex}` or include `vend_session_id` hash — **must not** create two refunds on duplicate POST.
3. **Cash:** if `provider == "cash"` (or configured cash label):
   - **Do not** mark cloud cash refunded.
   - Response extension: `local_cash_refund_required: true`, `refund_state: required` (or `not_required` for PSP + separate cash flag per product copy).
   - **Audit:** insert row or structured log + optional `financial_ledger_entries` / domain audit table per repo conventions.

4. **HTTP response body** for vend/failure should include the fields the spec requires when automatic refund is deferred:

```json
{
  "order_id": "...",
  "order_status": "failed",
  "vend_state": "failed",
  "refund_state": "required",
  "refund_required": true,
  "local_cash_refund_required": true
}
```

Adjust field names to match existing [`commerceVendFinalizeResponse`](../../internal/httpserver/commerce_http.go) (snake_case vs camelCase — **pick one** and align OpenAPI).

## State machine documentation

Add **`docs/api/commerce-order-lifecycle.md`** (short) or extend [`docs/api/machine-runtime.md`](../../docs/api/machine-runtime.md) / kiosk flow:

- States: order + payment + vend + refund aggregate.
- **Cancel** only pre-capture.
- **Refund** only post-capture, bounded by captured minus prior refunds.
- **Vend failure** → order failed + refund policy branch (online vs cash).

## Tests (packages)

- `internal/app/commerce` — unit tests for cancel/refund validation, idempotency, amount caps, duplicate vend failure single refund.
- `internal/httpserver` — httptest with stub validator + postgres integration when `TEST_DATABASE_URL` set.
- Cover **cross-tenant** `EnsureOrderOrganization` / org mismatch **403**.

## Acceptance

```bash
make sqlc  # if new queries
make swagger && make swagger-check
go test ./...
```

## P0.4 — Reconciliation queue, refund review, order timeline

Shipped artifacts:

- Tables: `order_timelines`, `refund_requests`; view `payment_reconciliation_cases` (see migrations + [`db/schema/01_platform.sql`](../../db/schema/01_platform.sql)).
- Admin routes under `/v1/admin/organizations/{organizationId}/`: reconciliation list/detail/**resolve**/**ignore**, **request-refund**, **orders/{orderId}/timeline**, **refunds** list/get, **orders/{orderId}/refunds** (requires **Idempotency-Key**).
- [`internal/app/commerceadmin/service.go`](../../internal/app/commerceadmin/service.go) — transactional resolve + timeline append; refund orchestration creates/links `refund_requests`.

Runbook: [`docs/runbooks/payment-reconciliation.md`](../runbooks/payment-reconciliation.md).

---

**Plan mode:** implementation requires Agent mode or manual apply.
