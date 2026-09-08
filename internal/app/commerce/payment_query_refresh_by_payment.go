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

// RefreshPendingPaymentFromProviderForPayment queries the PSP for a specific payment attempt.
func (s *Service) RefreshPendingPaymentFromProviderForPayment(
	ctx context.Context,
	companyID, orderID, paymentID uuid.UUID,
	machineExternalCode string,
) PaymentQueryRefreshOutcome {
	log := observability.LoggerFromContext(ctx, zap.NewNop())
	started := time.Now()
	out := PaymentQueryRefreshOutcome{Diagnostic: "awaiting_callback"}
	if s == nil || s.paymentSessionReg == nil || s.life == nil || s.webhook == nil {
		out.Diagnostic = "not_configured"
		return out
	}
	if orderID == uuid.Nil || paymentID == uuid.Nil {
		out.Diagnostic = "invalid_ids"
		return out
	}
	log.Info("PAYMENT_QUERY_REFRESH_START",
		zap.String("order_id", orderID.String()),
		zap.String("payment_id", paymentID.String()),
		zap.String("machine_code", strings.TrimSpace(machineExternalCode)),
	)
	pay, err := s.life.GetPaymentByID(ctx, paymentID)
	if err != nil {
		log.Warn("PAYMENT_QUERY_REFRESH_ERROR",
			zap.String("order_id", orderID.String()),
			zap.String("payment_id", paymentID.String()),
			zap.String("stage", "payment_lookup"),
			zap.Error(err),
		)
		out.Diagnostic = "payment_lookup_error"
		return out
	}
	if pay.OrderID != orderID {
		out.Diagnostic = "payment_order_mismatch"
		return out
	}
	state := strings.ToLower(strings.TrimSpace(pay.State))
	if state != "created" && state != "authorized" && state != "pending" {
		productionmetrics.RecordPaymentQueryRefresh("skipped_terminal_state")
		out.Diagnostic = "skipped_terminal_state"
		out.Skipped = true
		return out
	}
	if !s.paymentQueryThrottleAllows(pay.ID, pay.CreatedAt, started) {
		productionmetrics.RecordPaymentQueryRefresh("throttled")
		out.Diagnostic = "provider_throttled"
		out.Skipped = true
		return out
	}
	return s.refreshPaymentFromProvider(ctx, log, orderID, pay, machineExternalCode, started)
}

func (s *Service) refreshPaymentFromProvider(
	ctx context.Context,
	log *zap.Logger,
	orderID uuid.UUID,
	pay domaincommerce.Payment,
	machineExternalCode string,
	started time.Time,
) PaymentQueryRefreshOutcome {
	out := PaymentQueryRefreshOutcome{Diagnostic: "awaiting_callback"}
	provKey := strings.ToLower(strings.TrimSpace(pay.Provider))
	type providerGetter interface {
		Get(key string) platformpayments.PaymentProvider
	}
	var p platformpayments.PaymentProvider
	if g, ok := s.paymentSessionReg.(providerGetter); ok {
		p = g.Get(provKey)
	}
	if p == nil || !p.SupportsQueryPaymentStatus() {
		productionmetrics.RecordPaymentQueryRefresh("provider_unsupported")
		out.Diagnostic = "provider_unsupported"
		out.Skipped = true
		logOutcome(log, orderID, out, started)
		return out
	}
	providerRef := ""
	attemptPayload := []byte(nil)
	if pay.ID != uuid.Nil {
		if getter, ok := s.life.(interface {
			GetLatestPaymentAttemptProviderReference(ctx context.Context, paymentID uuid.UUID) (string, error)
			GetLatestPaymentAttemptPayload(ctx context.Context, paymentID uuid.UUID) ([]byte, error)
		}); ok {
			stored, refErr := getter.GetLatestPaymentAttemptProviderReference(ctx, pay.ID)
			if refErr == nil {
				providerRef = strings.TrimSpace(stored)
			}
			payload, payloadErr := getter.GetLatestPaymentAttemptPayload(ctx, pay.ID)
			if payloadErr == nil {
				attemptPayload = payload
			}
		}
	}
	if providerRef == "" && pay.ID != uuid.Nil {
		providerRef = ref.GenerateFromUUID(pay.ID)
	}
	s.paymentQueryThrottle.Store(pay.ID.String(), started)
	snap, err := p.QueryPaymentStatus(ctx, domaincommerce.PaymentProviderLookup{
		Provider:            provKey,
		PaymentID:           pay.ID,
		OrderID:             orderID,
		ProviderReference:   providerRef,
		AmountMinor:         pay.AmountMinor,
		MachineExternalCode: strings.TrimSpace(machineExternalCode),
		AttemptPayloadJSON:  attemptPayload,
	})
	if err != nil {
		productionmetrics.RecordPaymentQueryRefresh("provider_error")
		out.Diagnostic = "provider_error"
		logOutcome(log, orderID, out, started)
		return out
	}
	norm := strings.ToLower(strings.TrimSpace(snap.NormalizedState))
	if norm != "captured" {
		if norm == "failed" {
			out.Diagnostic = "provider_reported_failure"
		} else {
			out.Diagnostic = "provider_pending"
		}
		logOutcome(log, orderID, out, started)
		return out
	}
	eventID := "query_refresh:" + pay.ID.String() + ":captured"
	_, applyErr := s.ApplyPaymentProviderWebhook(ctx, ApplyPaymentProviderWebhookInput{
		OrderID:                 orderID,
		PaymentID:               pay.ID,
		Provider:                provKey,
		ProviderReference:       providerRef,
		WebhookEventID:          eventID,
		EventType:               "provider.query_refresh",
		NormalizedPaymentState:  norm,
		Payload:                 snap.ProviderHint,
		WebhookValidationStatus: "provider_native_verified",
	})
	if applyErr != nil {
		out.Diagnostic = "apply_rejected"
		logOutcome(log, orderID, out, started)
		return out
	}
	productionmetrics.RecordPaymentQueryRefresh(norm)
	out.Diagnostic = "captured"
	logOutcome(log, orderID, out, started)
	return out
}
