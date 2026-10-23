package commerce

import (
	"context"
	"sort"
	"strings"
	"time"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/observability"
	"github.com/avf/avf-vending-api/internal/platform/observability/productionmetrics"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	paymentQueryMinIntervalDefault  = 5 * time.Second
	paymentQueryMinIntervalActiveQR = 1500 * time.Millisecond
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
	if pay, err := s.resolveAuthoritativePayment(ctx, orderID); err == nil {
		state := strings.ToLower(strings.TrimSpace(pay.State))
		if state == "captured" {
			productionmetrics.RecordPaymentQueryRefresh("skipped_terminal_state")
			out.Diagnostic = "skipped_terminal_state"
			out.Skipped = true
			logOutcome(log, orderID, out, started)
			return out
		}
	}
	pending, err := s.pendingPaymentsForQueryRefresh(ctx, orderID)
	if err != nil {
		log.Warn("PAYMENT_QUERY_REFRESH_ERROR",
			zap.String("order_id", orderID.String()),
			zap.String("stage", "list_pending_payments"),
			zap.Error(err),
			zap.Int64("duration_ms", time.Since(started).Milliseconds()),
		)
		productionmetrics.RecordPaymentQueryRefresh("checkout_status_error")
		out.Diagnostic = "checkout_status_error"
		logOutcome(log, orderID, out, started)
		return out
	}
	if len(pending) == 0 {
		productionmetrics.RecordPaymentQueryRefresh("no_payment")
		out.Diagnostic = "no_payment"
		logOutcome(log, orderID, out, started)
		return out
	}
	sort.Slice(pending, func(i, j int) bool {
		return pending[i].CreatedAt.Before(pending[j].CreatedAt)
	})
	lastDiagnostic := "awaiting_callback"
	for _, pay := range pending {
		if err := ctx.Err(); err != nil {
			out.Diagnostic = lastDiagnostic
			logOutcome(log, orderID, out, started)
			return out
		}
		refresh := s.RefreshPendingPaymentFromProviderForPayment(ctx, companyID, orderID, pay.ID, machineExternalCode)
		lastDiagnostic = strings.TrimSpace(refresh.Diagnostic)
		if lastDiagnostic == "" {
			lastDiagnostic = "awaiting_callback"
		}
		if lastDiagnostic == "captured" {
			productionmetrics.RecordPaymentQueryRefresh("captured")
			out.Diagnostic = "captured"
			logOutcome(log, orderID, out, started)
			return out
		}
	}
	out.Diagnostic = lastDiagnostic
	logOutcome(log, orderID, out, started)
	return out
}

func (s *Service) pendingPaymentsForQueryRefresh(ctx context.Context, orderID uuid.UUID) ([]domaincommerce.Payment, error) {
	var all []domaincommerce.Payment
	var err error
	if s.financial != nil {
		all, err = s.financial.ListPaymentsForOrder(ctx, orderID)
		if err != nil {
			return nil, err
		}
	} else if s.life != nil {
		pay, err := s.life.GetLatestPaymentForOrder(ctx, orderID)
		if err != nil {
			return nil, err
		}
		all = []domaincommerce.Payment{pay}
	} else {
		return nil, ErrNotConfigured
	}
	out := make([]domaincommerce.Payment, 0, len(all))
	for _, pay := range all {
		if isPendingPaymentQueryState(pay.State) {
			out = append(out, pay)
		}
	}
	return out, nil
}

func isPendingPaymentQueryState(state string) bool {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "created", "authorized", "pending":
		return true
	default:
		return false
	}
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
