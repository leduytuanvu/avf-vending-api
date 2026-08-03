package commerce

import (
	"context"
	"strings"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	platformpayments "github.com/avf/avf-vending-api/internal/platform/payments"
	"github.com/google/uuid"
)

// RefreshPendingPaymentFromProvider optionally queries the live PSP when payment is still created/authorized
// and applies a captured/failed webhook locally when the provider reports a terminal state.
// Failures are ignored (poll/IPN remain authoritative); this only accelerates ZaloPay-style query paths.
func (s *Service) RefreshPendingPaymentFromProvider(ctx context.Context, companyID, orderID uuid.UUID) {
	if s == nil || s.paymentSessionReg == nil || s.life == nil || s.webhook == nil {
		return
	}
	st, err := s.GetCheckoutStatus(ctx, companyID, orderID, 0)
	if err != nil || !st.PaymentPresent {
		return
	}
	state := strings.ToLower(strings.TrimSpace(st.Payment.State))
	if state != "created" && state != "authorized" && state != "pending" {
		return
	}
	provKey := strings.ToLower(strings.TrimSpace(st.Payment.Provider))
	type providerGetter interface {
		Get(key string) platformpayments.PaymentProvider
	}
	var p platformpayments.PaymentProvider
	if g, ok := s.paymentSessionReg.(providerGetter); ok {
		p = g.Get(provKey)
	}
	if p == nil || !p.SupportsQueryPaymentStatus() {
		return
	}
	ref := ""
	if st.Payment.ID != uuid.Nil {
		// Best-effort: provider adapters need ProviderReference; try from latest attempt via lifecycle if available.
		if getter, ok := s.life.(interface {
			GetLatestPaymentAttemptProviderReference(ctx context.Context, paymentID uuid.UUID) (string, error)
		}); ok {
			ref, _ = getter.GetLatestPaymentAttemptProviderReference(ctx, st.Payment.ID)
		}
	}
	snap, err := p.QueryPaymentStatus(ctx, domaincommerce.PaymentProviderLookup{
		Provider:          provKey,
		PaymentID:         st.Payment.ID,
		OrderID:           orderID,
		ProviderReference: ref,
		AmountMinor:       st.Payment.AmountMinor,
	})
	if err != nil {
		return
	}
	norm := strings.ToLower(strings.TrimSpace(snap.NormalizedState))
	if norm != "captured" && norm != "failed" {
		return
	}
	eventID := "query_refresh:" + st.Payment.ID.String() + ":" + norm
	_, _ = s.ApplyPaymentProviderWebhook(ctx, ApplyPaymentProviderWebhookInput{
		OrderID:                 orderID,
		PaymentID:               st.Payment.ID,
		Provider:                provKey,
		ProviderReference:       ref,
		WebhookEventID:          eventID,
		EventType:               "provider.query_refresh",
		NormalizedPaymentState:  norm,
		Payload:                 snap.ProviderHint,
		WebhookValidationStatus: "provider_query_verified",
	})
}
