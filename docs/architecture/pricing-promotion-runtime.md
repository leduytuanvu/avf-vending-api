# Runtime pricing and promotions (single engine)

## Authority

All **vending sale amounts** visible on the runtime catalog, carried into **orders**, sent to **PSP payment sessions**, and validated on **payment webhooks** flow through **`internal/app/pricingengine`**.

1. **Slot list price** — `machine_slot_configs.price_minor` for the selected slot (planogram list price on the machine).
2. **`machine_price_overrides`** — if an override row is active for `(organization_id, machine_id, product_id)` at evaluation time, it **replaces** the slot list price as the register base (currency must match org default currency or evaluation fails).
3. **Promotions** — only rows returned by `PromotionAdminListPromotionsForPreview` apply (`lifecycle_status = active`, `approval_status = approved`, `starts_at <= at < ends_at`). Inactive, draft, or expired windows do not apply.

## Outputs

For each line, the engine produces:

- **Register unit minor** — base after machine override, before promotion discount.
- **Effective unit minor** — amount charged per unit after promotions.
- **Discount unit minor**, **applied promotion IDs**, optional **promotion label** (first applied name).
- **`PricingFingerprint`** per line — stable digest over org, machine, slot config id, slot index, product, list/register/effective amounts, and applied promotion ids. Mirrors into **`salecatalog.Item.PricingFingerprint`** and **`commerce.ResolvedSaleLine.PricingFingerprint`**.

Snapshot-level **`catalog_version`** (`runtime_sale_catalog_v5`) incorporates **`PromotionsSnapshotFingerprint`**, so catalog clients invalidate cache when promotion or override-affected prices change.

## Call sites

| Surface | Usage |
| --- | --- |
| Runtime sale catalog (`salecatalog.Service.BuildSnapshot`) | One `pricingengine.Batch` per snapshot; **PriceLine** per vendable item. |
| Checkout (`postgres.Store.ResolveSaleLine`) | **`Engine.EvaluateSaleLine`** (same batch rules as catalog when called at the same instant). |
| Admin promotion preview with **`machine_id`** | **`Batch.PriceLine`** from slot config (same path as kiosk). |
| Admin promotion preview **without** machine | Price-book preview base from `PreviewPricing`, then shared **`EvaluatePromotionDiscountForProduct`**. |

## Payment invariants

- **CreateMachinePaymentSession** and gRPC **CreatePaymentSession** require client **`amount_minor`** to equal **`orders.total_minor`**; the adapter always uses **persisted order totals** for the PSP.
- **Payment webhooks** require **`payments.amount_minor`** and optional **`provider_amount_minor`** to match **`orders.total_minor`**, and payment currency to match order currency.

## Tests

- **`internal/app/pricingengine`** — promotion math unit tests.
- **`internal/e2e/correctness/pricing_promotion_integration_test.go`** — catalog **`price_minor`** aligns with **`ResolveSaleLine`** when **`TEST_DATABASE_URL`** is set.
- **`internal/modules/postgres/commerce_webhook_p12_integration_test.go`** — webhook amount mismatch rejection.
