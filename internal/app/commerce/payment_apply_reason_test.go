package commerce

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClassifyApplyRejectReason(t *testing.T) {
	require.Equal(t, "", ClassifyApplyRejectReason(nil))
	require.Equal(t, "amount_currency_mismatch", ClassifyApplyRejectReason(ErrWebhookAmountCurrencyMismatch))
	require.Equal(t, "illegal_transition", ClassifyApplyRejectReason(ErrIllegalTransition))
	require.Equal(t, "provider_mismatch", ClassifyApplyRejectReason(ErrWebhookProviderMismatch))
	require.Equal(t, "after_terminal_order", ClassifyApplyRejectReason(ErrWebhookAfterTerminalOrder))
	require.Equal(t, "apply_error", ClassifyApplyRejectReason(errors.New("other")))
}
