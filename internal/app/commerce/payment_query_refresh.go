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

const (
	paymentQueryMinIntervalDefault  = 5 * time.Second
	paymentQueryMinIntervalActiveQR = 3 * time.Second
	paymentQueryAcceleratedWindow   = 2 * time.Minute
)

// RefreshPendingPaymentFromProvider optionally queries the live PSP when payment is still created/authorized
// and applies a captured webhook locally when the provider reports capture.
// Query refresh never applies failed — terminal failure remains the job of authenticated callbacks and expiry.
func (s *Service) RefreshPendingPaymentFromProvider(
	ctx context.Context,
	companyID, orderID uuid.UUID,
	machineExternalCode string,
) PaymentQueryRefreshOutcome {
	log := observability.LoggerFromContext(ctx, zap.NewNop())
	started := time.Now()
	out := PaymentQueryRefreshOutcome{Diagnostic: "awaiting_callback"}
	if s == nil || s.paymentSessionReg == nil || s.life == nil || s.webhook == nil {
		out.Diagnostic = "not_configured"
		return out
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
			out.Diagnostic = "checkout_status_error"
		} else {
			productionmetrics.RecordPaymentQueryRefresh("no_payment")
			out.Diagnostic = "no_payment"
		}
		logOutcome(log, orderID, out, started)
		return out
	}
	state := strings.ToLower(strings.TrimSpace(st.Payment.State))
	if state != "created" && state != "authorized" && state != "pending" {
		productionmetrics.RecordPaymentQueryRefresh("skipped_terminal_state")
		out.Diagnostic = "skipped_terminal_state"
		out.Skipped = true
		logOutcome(log, orderID, out, started)
		return out
	}
	if st.Payment.ID != uuid.Nil && !s.paymentQueryThrottleAllows(st.Payment.ID, st.Payment.CreatedAt, started) {
		productionmetrics.RecordPaymentQueryRefresh("throttled")
		out.Diagnostic = "provider_throttled"
		out.Skipped = true
		logOutcome(log, orderID, out, started)
		return out
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
		out.Diagnostic = "provider_unsupported"
		out.Skipped = true
		logOutcome(log, orderID, out, started)
		return out
	}
	providerRef := ""
	attemptPayload := []byte(nil)
	if st.Payment.ID != uuid.Nil {
		if getter, ok := s.life.(interface {
			GetLatestPaymentAttemptProviderReference(ctx context.Context, paymentID uuid.UUID) (string, error)
			GetLatestPaymentAttemptPayload(ctx context.Context, paymentID uuid.UUID) ([]byte, error)
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
			payload, payloadErr := getter.GetLatestPaymentAttemptPayload(ctx, st.Payment.ID)
			if payloadErr != nil {
				log.Warn("PAYMENT_QUERY_REFRESH_ERROR",
					zap.String("order_id", orderID.String()),
					zap.String("payment_id", st.Payment.ID.String()),
					zap.String("stage", "attempt_payload_lookup"),
					zap.Error(payloadErr),
				)
			} else {
				attemptPayload = payload
			}
		} else if getter, ok := s.life.(interface {
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
	if st.Payment.ID != uuid.Nil {
		s.paymentQueryThrottle.Store(st.Payment.ID.String(), started)
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
		AttemptPayloadJSON:  attemptPayload,
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
		out.Diagnostic = "provider_error"
		logOutcome(log, orderID, out, started)
		return out
	}
	norm := strings.ToLower(strings.TrimSpace(snap.NormalizedState))
	log.Info("PAYMENT_QUERY_PROVIDER_RESULT",
		zap.String("order_id", orderID.String()),
		zap.String("payment_id", st.Payment.ID.String()),
		zap.String("provider", provKey),
		zap.String("normalized_state", norm),
		zap.Int64("duration_ms", time.Since(started).Milliseconds()),
	)
	if norm != "captured" {
		if norm == "failed" {
			log.Warn("PAYMENT_QUERY_PROVIDER_REPORTED_FAILURE",
				zap.String("order_id", orderID.String()),
				zap.String("payment_id", st.Payment.ID.String()),
				zap.String("provider", provKey),
				zap.String("normalized_state", norm),
			)
			productionmetrics.RecordPaymentQueryRefresh("provider_reported_failure")
			out.Diagnostic = "provider_reported_failure"
		} else {
			productionmetrics.RecordPaymentQueryRefresh("provider_pending")
			out.Diagnostic = "provider_pending"
		}
		logOutcome(log, orderID, out, started)
		return out
	}
	eventID := "query_refresh:" + st.Payment.ID.String() + ":captured"
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
		reason := ClassifyApplyRejectReason(applyErr)
		log.Warn("PAYMENT_CAPTURE_APPLY_ERROR",
			zap.String("order_id", orderID.String()),
			zap.String("payment_id", st.Payment.ID.String()),
			zap.String("provider", provKey),
			zap.String("normalized_state", norm),
			zap.String("reject_reason", reason),
			zap.Error(applyErr),
			zap.Int64("duration_ms", time.Since(started).Milliseconds()),
		)
		productionmetrics.RecordPaymentQueryRefresh("apply_error")
		out.Diagnostic = "apply_rejected"
		logOutcome(log, orderID, out, started)
		return out
	}
	log.Info("PAYMENT_CAPTURE_APPLY_SUCCESS",
		zap.String("order_id", orderID.String()),
		zap.String("payment_id", st.Payment.ID.String()),
		zap.String("provider", provKey),
		zap.String("normalized_state", norm),
		zap.Int64("duration_ms", time.Since(started).Milliseconds()),
	)
	productionmetrics.RecordPaymentQueryRefresh(norm)
	out.Diagnostic = "captured"
	logOutcome(log, orderID, out, started)
	return out
}

func paymentQueryMinInterval(paymentCreatedAt, now time.Time) time.Duration {
	if paymentCreatedAt.IsZero() {
		return paymentQueryMinIntervalDefault
	}
	if now.Sub(paymentCreatedAt) <= paymentQueryAcceleratedWindow {
		return paymentQueryMinIntervalActiveQR
	}
	return paymentQueryMinIntervalDefault
}

func (s *Service) paymentQueryThrottleAllows(paymentID uuid.UUID, paymentCreatedAt, now time.Time) bool {
	if s == nil || paymentID == uuid.Nil {
		return true
	}
	minInterval := paymentQueryMinInterval(paymentCreatedAt, now)
	key := paymentID.String()
	if prev, ok := s.paymentQueryThrottle.Load(key); ok {
		if last, ok := prev.(time.Time); ok && now.Sub(last) < minInterval {
			return false
		}
	}
	return true
}

func logOutcome(log *zap.Logger, orderID uuid.UUID, out PaymentQueryRefreshOutcome, started time.Time) {
	if log == nil {
		return
	}
	log.Info("PAYMENT_QUERY_REFRESH_OUTCOME",
		zap.String("order_id", orderID.String()),
		zap.String("diagnostic", out.Diagnostic),
		zap.Bool("skipped", out.Skipped),
		zap.Int64("duration_ms", time.Since(started).Milliseconds()),
	)
}
