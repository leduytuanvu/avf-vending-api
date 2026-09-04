package commerce

import (
	"context"
	"strings"
	"time"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/observability"
	"github.com/avf/avf-vending-api/internal/platform/observability/productionmetrics"
	platformpayments "github.com/avf/avf-vending-api/internal/platform/payments"
	"github.com/avf/avf-vending-api/internal/platform/payments/psp/ref"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// RefreshPendingPaymentFromProvider optionally queries the live PSP when payment is still created/authorized
// and applies a captured/failed webhook locally when the provider reports a terminal state.
// Failures are logged; poll/IPN remain authoritative — this only accelerates provider query paths.
func (s *Service) RefreshPendingPaymentFromProvider(
	ctx context.Context,
	companyID, orderID uuid.UUID,
	machineExternalCode string,
) {
	log := observability.LoggerFromContext(ctx, zap.NewNop())
	started := time.Now()
	if s == nil || s.paymentSessionReg == nil || s.life == nil || s.webhook == nil {
		return
	}
	log.Info("PAYMENT_QUERY_REFRESH_START",
		zap.String("order_id", orderID.String()),
		zap.String("machine_code", strings.TrimSpace(machineExternalCode)),
	)
	st, err := s.GetCheckoutStatus(ctx, companyID, orderID, 0)
	if err != nil || !st.PaymentPresent {
		if err != nil {
			log.Warn("PAYMENT_QUERY_REFRESH_ERROR",
				zap.String("order_id", orderID.String()),
				zap.String("stage", "checkout_status"),
				zap.Error(err),
				zap.Int64("duration_ms", time.Since(started).Milliseconds()),
			)
			productionmetrics.RecordPaymentQueryRefresh("checkout_status_error")
		} else {
			productionmetrics.RecordPaymentQueryRefresh("no_payment")
		}
		return
	}
	state := strings.ToLower(strings.TrimSpace(st.Payment.State))
	if state != "created" && state != "authorized" && state != "pending" {
		productionmetrics.RecordPaymentQueryRefresh("skipped_terminal_state")
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
		productionmetrics.RecordPaymentQueryRefresh("provider_unsupported")
		return
	}
	providerRef := ""
	if st.Payment.ID != uuid.Nil {
		if getter, ok := s.life.(interface {
			GetLatestPaymentAttemptProviderReference(ctx context.Context, paymentID uuid.UUID) (string, error)
		}); ok {
			stored, refErr := getter.GetLatestPaymentAttemptProviderReference(ctx, st.Payment.ID)
			if refErr != nil {
				log.Warn("PAYMENT_QUERY_REFRESH_ERROR",
					zap.String("order_id", orderID.String()),
					zap.String("payment_id", st.Payment.ID.String()),
					zap.String("stage", "provider_reference_lookup"),
					zap.Error(refErr),
				)
			} else {
				providerRef = strings.TrimSpace(stored)
			}
		}
	}
	if providerRef == "" && st.Payment.ID != uuid.Nil {
		providerRef = ref.GenerateFromUUID(st.Payment.ID)
		log.Info("PAYMENT_PROVIDER_REFERENCE_RESOLVED",
			zap.String("order_id", orderID.String()),
			zap.String("payment_id", st.Payment.ID.String()),
			zap.Bool("provider_reference_present", providerRef != ""),
			zap.String("source", "payment_id_fallback"),
		)
	} else {
		log.Info("PAYMENT_PROVIDER_REFERENCE_RESOLVED",
			zap.String("order_id", orderID.String()),
			zap.String("payment_id", st.Payment.ID.String()),
			zap.Bool("provider_reference_present", providerRef != ""),
			zap.String("source", "latest_attempt"),
		)
	}
	log.Info("PAYMENT_QUERY_PROVIDER_START",
		zap.String("order_id", orderID.String()),
		zap.String("payment_id", st.Payment.ID.String()),
		zap.String("provider", provKey),
		zap.String("machine_code", strings.TrimSpace(machineExternalCode)),
	)
	snap, err := p.QueryPaymentStatus(ctx, domaincommerce.PaymentProviderLookup{
		Provider:            provKey,
		PaymentID:           st.Payment.ID,
		OrderID:             orderID,
		ProviderReference:   providerRef,
		AmountMinor:         st.Payment.AmountMinor,
		MachineExternalCode: strings.TrimSpace(machineExternalCode),
	})
	if err != nil {
		log.Warn("PAYMENT_QUERY_PROVIDER_ERROR",
			zap.String("order_id", orderID.String()),
			zap.String("payment_id", st.Payment.ID.String()),
			zap.String("provider", provKey),
			zap.Error(err),
			zap.Int64("duration_ms", time.Since(started).Milliseconds()),
		)
		productionmetrics.RecordPaymentQueryRefresh("provider_error")
		return
	}
	norm := strings.ToLower(strings.TrimSpace(snap.NormalizedState))
	log.Info("PAYMENT_QUERY_PROVIDER_RESULT",
		zap.String("order_id", orderID.String()),
		zap.String("payment_id", st.Payment.ID.String()),
		zap.String("provider", provKey),
		zap.String("normalized_state", norm),
		zap.Int64("duration_ms", time.Since(started).Milliseconds()),
	)
	if norm != "captured" && norm != "failed" {
		productionmetrics.RecordPaymentQueryRefresh("provider_pending")
		return
	}
	eventID := "query_refresh:" + st.Payment.ID.String() + ":" + norm
	log.Info("PAYMENT_CAPTURE_APPLY_START",
		zap.String("order_id", orderID.String()),
		zap.String("payment_id", st.Payment.ID.String()),
		zap.String("provider", provKey),
		zap.String("normalized_state", norm),
	)
	_, applyErr := s.ApplyPaymentProviderWebhook(ctx, ApplyPaymentProviderWebhookInput{
		OrderID:                 orderID,
		PaymentID:               st.Payment.ID,
		Provider:                provKey,
		ProviderReference:       providerRef,
		WebhookEventID:          eventID,
		EventType:               "provider.query_refresh",
		NormalizedPaymentState:  norm,
		Payload:                 snap.ProviderHint,
		WebhookValidationStatus: "provider_native_verified",
	})
	if applyErr != nil {
		log.Warn("PAYMENT_CAPTURE_APPLY_ERROR",
			zap.String("order_id", orderID.String()),
			zap.String("payment_id", st.Payment.ID.String()),
			zap.String("provider", provKey),
			zap.String("normalized_state", norm),
			zap.Error(applyErr),
			zap.Int64("duration_ms", time.Since(started).Milliseconds()),
		)
		productionmetrics.RecordPaymentQueryRefresh("apply_error")
		return
	}
	log.Info("PAYMENT_CAPTURE_APPLY_SUCCESS",
		zap.String("order_id", orderID.String()),
		zap.String("payment_id", st.Payment.ID.String()),
		zap.String("provider", provKey),
		zap.String("normalized_state", norm),
		zap.Int64("duration_ms", time.Since(started).Milliseconds()),
	)
	productionmetrics.RecordPaymentQueryRefresh(norm)
}
