package commerce

import (
	"context"
	"errors"
	"strings"
	"time"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/google/uuid"
)

// ConfirmCashPayment records cash evidence, creates payment, and claims winner atomically.
func (s *Service) ConfirmCashPayment(ctx context.Context, in ConfirmCashPaymentInput) (ConfirmCashPaymentResult, error) {
	if s.payments == nil || s.life == nil {
		return ConfirmCashPaymentResult{}, ErrNotConfigured
	}
	if in.OrderID == uuid.Nil || in.MachineID == uuid.Nil {
		return ConfirmCashPaymentResult{}, errors.Join(ErrInvalidArgument, errors.New("order_id and machine_id required"))
	}
	key := strings.TrimSpace(in.IdempotencyKey)
	if key == "" {
		return ConfirmCashPaymentResult{}, errors.Join(ErrInvalidArgument, errors.New("idempotency_key required"))
	}

	o, err := s.life.GetOrderByID(ctx, in.OrderID)
	if err != nil {
		return ConfirmCashPaymentResult{}, err
	}
	if o.MachineID != in.MachineID {
		return ConfirmCashPaymentResult{}, errors.Join(ErrInvalidArgument, errors.New("order machine mismatch"))
	}
	if orderStatusTerminal(o.Status) {
		return ConfirmCashPaymentResult{}, errors.Join(ErrIllegalTransition, errors.New("order is terminal"))
	}

	consent := normalizeConsentSource(in.ConsentSource)
	allocated := in.AllocatedMinor
	legacyThin := in.GrossAcceptedMinor == 0 && in.AllocatedMinor == 0
	if legacyThin {
		allocated = o.TotalMinor
		consent = "unknown"
	} else if allocated <= 0 {
		return ConfirmCashPaymentResult{}, errors.Join(ErrInvalidArgument, errors.New("allocated_minor must be positive"))
	} else if allocated != o.TotalMinor {
		return ConfirmCashPaymentResult{}, errors.Join(ErrInvalidArgument, errors.New("allocated_minor must match order total"))
	}
	if in.PreOrderCreditMinor > 0 && consent != "explicit_confirm" && consent != "unknown" && consent != "operator" {
		if consent == "implicit_post_order" && in.PreOrderCreditMinor > 0 && in.PostOrderInsertedMinor == 0 {
			// post-order only path ok
		} else if in.PreOrderCreditMinor > 0 && consent != "explicit_confirm" {
			return ConfirmCashPaymentResult{}, errors.Join(ErrInvalidArgument, errors.New("pre_order_credit requires explicit_confirm consent"))
		}
	}
	if in.PreOrderCreditMinor > 0 && consent != "explicit_confirm" && consent != "unknown" && in.PostOrderInsertedMinor == 0 {
		return ConfirmCashPaymentResult{}, errors.Join(ErrInvalidArgument, errors.New("pre_order_credit requires explicit_confirm consent"))
	}

	currency := strings.ToUpper(strings.TrimSpace(in.Currency))
	if currency == "" {
		currency = strings.ToUpper(strings.TrimSpace(o.Currency))
	}

	payKey := key + ":cash:payment"
	outboxIdem := key + ":cash:payment:outbox:" + in.OrderID.String()
	payRes, err := s.StartPaymentWithOutbox(ctx, StartPaymentInput{
		OrderID:              in.OrderID,
		Provider:             "cash",
		PaymentState:         "captured",
		AmountMinor:          allocated,
		Currency:             currency,
		IdempotencyKey:       payKey,
		OutboxTopic:          in.OutboxTopic,
		OutboxEventType:      in.OutboxEventType,
		OutboxPayload:        []byte(`{"source":"machine_grpc_cash"}`),
		OutboxAggregateType:  in.OutboxAggregateType,
		OutboxAggregateID:    in.OrderID,
		OutboxIdempotencyKey: outboxIdem,
		Simulated:            in.Simulated,
		SimulationRunID:      in.SimulationRunID,
		SimulationScenario:   in.SimulationScenario,
		FakeBill:             in.FakeBill,
		FakeBoard:            in.FakeBoard,
	})
	if err != nil {
		return ConfirmCashPaymentResult{}, err
	}
	if payRes.Replay {
		if payRes.Payment.AmountMinor != allocated ||
			strings.ToUpper(strings.TrimSpace(payRes.Payment.Currency)) != currency ||
			payRes.Payment.State != "captured" {
			return ConfirmCashPaymentResult{}, ErrIdempotencyPayloadConflict
		}
	}

	var alloc CashAllocationView
	var changeEv *CashChangeEventView
	if s.financial != nil {
		if len(in.AcceptanceEvents) > 0 {
			if err := s.financial.RecordCashAcceptanceEvents(ctx, RecordCashAcceptanceEventsInput{
				MachineID: in.MachineID,
				OrderID:   in.OrderID,
				Currency:  currency,
				Events:    in.AcceptanceEvents,
			}); err != nil {
				return ConfirmCashPaymentResult{}, err
			}
		}
		alloc, err = s.financial.RecordCashAllocation(ctx, RecordCashAllocationInput{
			OrderID:                in.OrderID,
			PaymentID:              payRes.Payment.ID,
			MachineID:              in.MachineID,
			AmountMinor:            allocated,
			PreOrderCreditMinor:    in.PreOrderCreditMinor,
			PostOrderInsertedMinor: in.PostOrderInsertedMinor,
			ConsentSource:          consent,
			ConsentedAt:            in.ConsentedAt,
			Currency:               currency,
			IdempotencyKey:         key + ":cash:allocation",
		})
		if err != nil {
			return ConfirmCashPaymentResult{}, err
		}
		changeOutcome := normalizeChangeOutcome(in.ChangeOutcome)
		if in.ChangeDueMinor > 0 || in.ChangeDispensedMinor > 0 || changeOutcome != "none" {
			liability := int64(0)
			if changeOutcome == "not_delivered" || changeOutcome == "ambiguous" {
				liability = in.ChangeDueMinor - in.ChangeDispensedMinor
				if liability < 0 {
					liability = in.ChangeDueMinor
				}
			}
			ce, cerr := s.financial.RecordCashChangeEvent(ctx, RecordCashChangeEventInput{
				OrderID:              in.OrderID,
				PaymentID:            payRes.Payment.ID,
				MachineID:            in.MachineID,
				ChangeDueMinor:       in.ChangeDueMinor,
				ChangeDispensedMinor: in.ChangeDispensedMinor,
				Outcome:              changeOutcome,
				LiabilityMinor:       liability,
				Currency:             currency,
				IdempotencyKey:       key + ":cash:change",
			})
			if cerr != nil {
				return ConfirmCashPaymentResult{}, cerr
			}
			changeEv = &ce
		}
		_ = s.financial.InsertLedgerEntry(ctx, LedgerEntryInput{
			MachineID:         &in.MachineID,
			OrderID:           &in.OrderID,
			PaymentID:         &payRes.Payment.ID,
			EntryType:         "cash_allocated",
			SignedAmountMinor: allocated,
			Currency:          currency,
			OccurredAt:        time.Now().UTC(),
		})
		if consent == "unknown" {
			orderIDCopy := in.OrderID
			paymentIDCopy := payRes.Payment.ID
			_, _ = s.financial.UpsertReconciliationCase(ctx, domaincommerce.ReconciliationCaseInput{
				CaseType:       "legacy_cash_confirm_unknown_consent",
				Severity:       "info",
				OrderID:        &orderIDCopy,
				PaymentID:      &paymentIDCopy,
				MachineID:      &in.MachineID,
				Reason:         "Legacy thin cash confirm without consent or amount evidence",
				CorrelationKey: "financial_correctness:legacy_cash:" + in.OrderID.String(),
			})
		}
	}

	var paid domaincommerce.Order
	if s.winnerArbitrationEnabled && s.financial != nil {
		arb, aerr := s.AttemptWinningPaymentClaim(ctx, payRes.Payment.ID, in.OrderID)
		if aerr != nil {
			return ConfirmCashPaymentResult{}, aerr
		}
		paid = arb.Order
		if !arb.Won {
			return ConfirmCashPaymentResult{}, errors.Join(ErrIllegalTransition, errors.New("payment lost winner arbitration"))
		}
	} else {
		paid, err = s.MarkOrderPaidAfterPaymentCapture(ctx, uuid.Nil, in.OrderID)
		if err != nil {
			return ConfirmCashPaymentResult{}, err
		}
	}

	return ConfirmCashPaymentResult{
		Replay:      payRes.Replay,
		Payment:     payRes.Payment,
		Order:       paid,
		Allocation:  alloc,
		ChangeEvent: changeEv,
	}, nil
}

// CancelPaymentSession cancels the latest non-captured payment while keeping the order payable.
func (s *Service) CancelPaymentSession(ctx context.Context, in CancelPaymentSessionInput) (CancelPaymentSessionResult, error) {
	if s.life == nil {
		return CancelPaymentSessionResult{}, ErrNotConfigured
	}
	if in.OrderID == uuid.Nil {
		return CancelPaymentSessionResult{}, errors.Join(ErrInvalidArgument, errors.New("order_id required"))
	}
	o, err := s.life.GetOrderByID(ctx, in.OrderID)
	if err != nil {
		return CancelPaymentSessionResult{}, err
	}
	if o.MachineID != in.MachineID {
		return CancelPaymentSessionResult{}, errors.Join(ErrInvalidArgument, errors.New("order machine mismatch"))
	}
	if orderStatusTerminal(o.Status) {
		return CancelPaymentSessionResult{}, errors.Join(ErrIllegalTransition, errors.New("order is terminal"))
	}
	if s.financial == nil {
		return CancelPaymentSessionResult{}, ErrNotConfigured
	}
	pay, err := s.financial.GetLatestNonCapturedPaymentForOrder(ctx, in.OrderID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return CancelPaymentSessionResult{Order: o, PaymentFound: false}, nil
		}
		return CancelPaymentSessionResult{}, err
	}
	canceled, err := s.financial.CancelPaymentByID(ctx, pay.ID)
	if err != nil {
		return CancelPaymentSessionResult{}, err
	}
	return CancelPaymentSessionResult{
		Order:        o,
		Payment:      canceled,
		PaymentFound: true,
	}, nil
}

// GetOrderMoneyView returns the admin money read model for an order.
func (s *Service) GetOrderMoneyView(ctx context.Context, orderID uuid.UUID) (OrderMoneyView, error) {
	if s.financial == nil {
		return OrderMoneyView{}, ErrNotConfigured
	}
	return s.financial.GetOrderMoneyView(ctx, orderID)
}
