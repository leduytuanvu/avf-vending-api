// Package commerce orchestrates checkout, payment recording, vend progression, and order completion.
//
// Payment capture, vend success, and order completion are intentionally separate steps: settling
// money does not imply a successful physical vend, and publishing side effects is not handled here.
//
// Persistence uses domaincommerce.OrderVendWorkflow / PaymentOutboxWorkflow plus CommerceLifecycleStore;
// implement the store in internal/modules/postgres (sqlc) for reads/updates not yet in db/queries.
//
// TODO: Add sqlc + Store methods for GetOrderByID, UpdateOrderStatus, UpdateVendSessionState,
// GetLatestPaymentForOrder, InsertPaymentAttempt.
package commerce
