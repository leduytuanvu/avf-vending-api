package commerce

import (
	"context"
	"errors"
	"strings"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/google/uuid"
)

// PaymentStatusView is the machine-safe per-payment read model for multi-attempt observation.
type PaymentStatusView struct {
	OrderID           uuid.UUID
	Payment           domaincommerce.Payment
	Outcome           string
	IsWinningPayment  bool
	PaymentDiagnostic string
}

// GetPaymentStatusView returns authoritative state for a single payment attempt on an order.
func (s *Service) GetPaymentStatusView(
	ctx context.Context,
	companyID, orderID, paymentID uuid.UUID,
	machineExternalCode string,
) (PaymentStatusView, error) {
	if s == nil || s.life == nil {
		return PaymentStatusView{}, ErrNotConfigured
	}
	if orderID == uuid.Nil || paymentID == uuid.Nil {
		return PaymentStatusView{}, errors.Join(ErrInvalidArgument, errors.New("order_id and payment_id required"))
	}
	o, err := s.life.GetOrderByID(ctx, orderID)
	if err != nil {
		return PaymentStatusView{}, err
	}
	if companyID != uuid.Nil {
		return PaymentStatusView{}, ErrOrgMismatch
	}
	pay, err := s.life.GetPaymentByID(ctx, paymentID)
	if err != nil {
		return PaymentStatusView{}, err
	}
	if pay.OrderID != orderID {
		return PaymentStatusView{}, errors.Join(ErrInvalidArgument, errors.New("payment order mismatch"))
	}
	out := PaymentStatusView{
		OrderID: orderID,
		Payment: pay,
		Outcome: strings.TrimSpace(pay.Outcome),
	}
	if o.WinningPaymentID != nil && *o.WinningPaymentID == paymentID {
		out.IsWinningPayment = true
	}
	state := strings.ToLower(strings.TrimSpace(pay.State))
	if state == "created" || state == "authorized" || state == "pending" {
		refresh := s.RefreshPendingPaymentFromProviderForPayment(ctx, companyID, orderID, paymentID, machineExternalCode)
		out.PaymentDiagnostic = strings.TrimSpace(refresh.Diagnostic)
		pay, err = s.life.GetPaymentByID(ctx, paymentID)
		if err != nil {
			return PaymentStatusView{}, err
		}
		out.Payment = pay
		out.Outcome = strings.TrimSpace(pay.Outcome)
		if o.WinningPaymentID != nil && *o.WinningPaymentID == paymentID {
			out.IsWinningPayment = true
		}
	}
	return out, nil
}
