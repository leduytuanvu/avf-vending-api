# Commerce / payment flow report
- Generated (UTC): `2026-05-16T10:20:10.598279+00:00`
- `BASE_URL`: `http://127.0.0.1:18080`
- Payment provider label: `mock`
- No `organization_id` query parameters were used.

## Go tests (focused)

Command:

```text
go test ./internal/app/commerce/... ./internal/app/payments/... ./internal/modules/postgres/... -run 'Commerce|Payment|Order|Vend|Refund|Reconciliation|Idempotency' -count=1
```

Output (`reports/test/commerce-flow/go-test-packages.txt`):

```text
ok  	github.com/avf/avf-vending-api/internal/app/commerce	0.180s
ok  	github.com/avf/avf-vending-api/internal/app/payments	0.161s
ok  	github.com/avf/avf-vending-api/internal/modules/postgres	0.169s
```

Additional (`reports/test/commerce-flow/go-test-httpserver.txt`):

```text
ok  	github.com/avf/avf-vending-api/internal/httpserver	0.159s
```

## HTTP flow script

- Runner: `scripts/test/commerce_http_flow.py`
- Evidence: `reports/test/commerce-flow/http-*.json`

### Created / referenced resource IDs

- `site_id`: `cd3effa6-e683-4962-9463-aecd27138017`
- `product_id`: `93088196-f88b-4306-9975-3b1542ee75f4`
- `machine_id`: `b0c2c054-8aaf-4c82-b320-831a9009c1a5`

### Inventory check

- Sum(`currentQuantity`) before: `0`
- Sum(`currentQuantity`) after: `0`
- Decrease detected: **False**

### HTTP step summary

| step | method | path | status | pass |
| --- | --- | --- | --- | --- |
| login | POST | `/v1/auth/login` | 200 | pass |
| inventory_before | GET | `/v1/admin/machines/b0c2c054-8aaf-4c82-b320-831a9009c1a5/inventory` | 200 | pass |
| commerce_create_order | POST | `/v1/commerce/orders` | 400 | pass |
| commerce_create_order_idempotency_replay | POST | `/v1/commerce/orders` | 400 | pass |
| inventory_after | GET | `/v1/admin/machines/b0c2c054-8aaf-4c82-b320-831a9009c1a5/inventory` | 200 | pass |
| commerce_order_for_vend_failure | POST | `/v1/commerce/orders` | 400 | pass |

### Organization / tenant field scan

- No `organization_id` / `tenant` substrings detected in JSON responses for completed HTTP steps (transport failures yield empty bodies).

## Root causes / fixes

- **HTTP `company_required` on `POST /v1/commerce/orders` / cash-checkout**: `commerceScopeFromRequest` intentionally returns `uuid.Nil` for single-company mode while handlers incorrectly rejected `uuid.Nil`. Removed those guards in `internal/httpserver/commerce_http.go`.

Observations from this HTTP run:

- Provisioned canary site/product/active machine via admin APIs.
- Order idempotency replay: expected HTTP 201 with replay=true.
- Inventory sum did not decrease (0 -> 0); machine may lack published planogram/stock.
- Skipping vend-failure path: second order did not create.

## Final result

| Gate | Result |
| --- | --- |
| Go tests (commerce/payments/postgres match + optional httpserver webhook subset) | **PASS** |
| HTTP flow (`BASE_URL=http://127.0.0.1:18080`) | **FAIL — see step table** |
| **Overall** | **FAIL** |

Go tests passed but HTTP did not. Start the API (see README: Docker deps + `HTTP_ADDR`, typically `http://127.0.0.1:18080` on Windows scripts), set `COMMERCE_PAYMENT_WEBHOOK_ALLOW_UNSIGNED=true` for unsigned webhook simulation in dev, ensure commerce outbox env for payment-session, then re-run `python scripts/test/commerce_http_flow.py`.

## Files touched / produced

- `internal/httpserver/commerce_http.go` — remove erroneous `company_required` gates.
- `scripts/test/commerce_http_flow.py`
- `reports/test/commerce-flow-report.md`
- `reports/test/commerce-flow/*`
