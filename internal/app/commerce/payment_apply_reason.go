package commerce

import (
	"errors"
	"strings"
)

// ClassifyApplyRejectReason maps webhook apply errors to a stable diagnostic reason code.
func ClassifyApplyRejectReason(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, ErrWebhookAmountCurrencyMismatch):
		return "amount_currency_mismatch"
	case errors.Is(err, ErrIllegalTransition):
		return "illegal_transition"
	case errors.Is(err, ErrWebhookProviderMismatch):
		return "provider_mismatch"
	case errors.Is(err, ErrWebhookAfterTerminalOrder):
		return "after_terminal_order"
	case errors.Is(err, ErrWebhookIdempotencyConflict):
		return "idempotency_conflict"
	case errors.Is(err, ErrOrgMismatch):
		return "org_mismatch"
	default:
		msg := strings.ToLower(strings.TrimSpace(err.Error()))
		if strings.Contains(msg, "terminal") {
			return "after_terminal_order"
		}
		return "apply_error"
	}
}
