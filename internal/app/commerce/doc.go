// Package commerce orchestrates checkout, payment recording, vend progression, and order completion.
//
// Settlement (payment captured) stays distinct from a successful mechanical vend: callers explicitly
// advance vend sessions (`pending→in_progress→success|failed`) while orders mirror `paid|vending→completed|failed`.
// Successful terminal vend (`FinalizeOrderAfterVend` with success) delegates to FulfillSuccessfulVendAtomically
// in stores so terminal order+vend rows, idempotent inventory decrement, and `order_timelines` audit inserts
// share one Postgres transaction—or roll back together on insufficient stock / invariant violations.
//
// Persistence uses domaincommerce.OrderVendWorkflow / PaymentOutboxWorkflow plus CommerceLifecycleStore
// implemented in internal/modules/postgres (sqlc).
package commerce
