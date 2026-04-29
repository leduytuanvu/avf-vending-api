package commerce

import "errors"

var (
	ErrInvalidArgument   = errors.New("commerce: invalid argument")
	ErrNotConfigured     = errors.New("commerce: dependency not configured")
	ErrNotFound          = errors.New("commerce: not found")
	ErrOrgMismatch       = errors.New("commerce: organization mismatch")
	ErrIllegalTransition = errors.New("commerce: illegal state transition")
	ErrPaymentNotSettled = errors.New("commerce: payment not in a settled captured state for this operation")
	ErrRefundNotAllowed  = errors.New("commerce: refund not allowed for current payment or order state")
	ErrCancelNotAllowed  = errors.New("commerce: cancel not allowed for current order state")
	// ErrWebhookIdempotencyConflict means a replay used a different provider_reference or webhook_event_id than stored.
	ErrWebhookIdempotencyConflict = errors.New("commerce: webhook idempotency conflict")
	// ErrWebhookProviderMismatch means the webhook body provider does not match the payment's provider.
	ErrWebhookProviderMismatch = errors.New("commerce: webhook provider does not match payment provider")
	// ErrWebhookAmountCurrencyMismatch means provider-reported amounts or currency disagree with persisted payment and order totals.
	ErrWebhookAmountCurrencyMismatch = errors.New("commerce: webhook amount or currency does not match payment")
	// ErrWebhookAfterTerminalOrder means the PSP notified after order/payment reached a terminal lifecycle state.
	ErrWebhookAfterTerminalOrder = errors.New("commerce: webhook rejected because order or payment is already terminal")
	// ErrIdempotencyPayloadConflict means an idempotency key was reused with different semantic payload than the original request.
	ErrIdempotencyPayloadConflict = errors.New("commerce: idempotency key conflict")
)
