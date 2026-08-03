package payments

import (
	"context"
	"fmt"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/workfloworch"
	"github.com/google/uuid"
)

// RegistryRefundBridge adapts the payment Registry to workfloworch.RefundProvider.
type RegistryRefundBridge struct {
	Reg            *Registry
	ResolveAttempt func(ctx context.Context, paymentID uuid.UUID) (provider string, providerReference string, err error)
}

// RefundPayment looks up the payment's provider adapter and calls RefundPayment.
func (b RegistryRefundBridge) RefundPayment(ctx context.Context, in workfloworch.ProviderRefundRequest) error {
	if b.Reg == nil {
		return fmt.Errorf("payments: nil registry for refund")
	}
	if b.ResolveAttempt == nil {
		return fmt.Errorf("payments: ResolveAttempt required for refund")
	}
	provider, ref, err := b.ResolveAttempt(ctx, in.PaymentID)
	if err != nil {
		return err
	}
	p := b.Reg.Get(provider)
	if p == nil {
		return fmt.Errorf("%w: %q", ErrUnknownProvider, provider)
	}
	return p.RefundPayment(ctx, RefundPaymentInput{
		PaymentID:         in.PaymentID,
		ProviderReference: ref,
		AmountMinor:       in.AmountMinor,
		Currency:          in.Currency,
		IdempotencyKey:    strings.TrimSpace(in.IdempotencyKey),
	})
}
