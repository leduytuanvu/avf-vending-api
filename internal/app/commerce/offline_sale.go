package commerce

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// OfflineSaleWirePayload is the device OFFLINE_SALE outbox JSON (Kotlin CheckoutPricingSnapshot bundle).
type OfflineSaleWirePayload struct {
	OrderID           string          `json:"orderId"`
	MachineID         string          `json:"machineId"`
	TransactionID     string          `json:"transactionId"`
	CashReceivedMinor int64           `json:"cashReceivedMinor"`
	PricingSnapshot   json.RawMessage `json:"pricingSnapshot"`
	SnapshotID        string          `json:"snapshotId"`
	PayableTotalMinor int64           `json:"payableTotalMinor"`
	Currency          string          `json:"currency"`
}

// appCheckoutPricingSnapshot mirrors Kotlin CheckoutPricingSnapshot JSON.
type appCheckoutPricingSnapshot struct {
	SnapshotID           string            `json:"snapshotId"`
	MachineID            string            `json:"machineId"`
	PayableTotalMinor    int64             `json:"payableTotalMinor"`
	Currency             string            `json:"currency"`
	LocalPricingRevision int64             `json:"localPricingRevision"`
	SlotConfigVersion    int               `json:"slotConfigVersion"`
	CapturedAtEpochMs    int64             `json:"capturedAtEpochMs"`
	Lines                []appCheckoutLine `json:"lines"`
}

type appCheckoutLine struct {
	SlotCode       string `json:"slotCode"`
	ProductID      string `json:"productId"`
	UnitPriceMinor int64  `json:"unitPriceMinor"`
	Quantity       int    `json:"quantity"`
}

// OfflineSaleOutboxConfig carries payment-outbox wiring for cash confirm.
type OfflineSaleOutboxConfig struct {
	Topic         string
	EventType     string
	AggregateType string
}

// ProcessOfflineSale replays a bundled offline cash checkout: create order + confirm cash payment.
// Vend close is replayed via separate commerce.start_vend / commerce.confirm_vend_success events.
func (s *Service) ProcessOfflineSale(
	ctx context.Context,
	machineID uuid.UUID,
	idempotencyKey string,
	clientEventID string,
	payload []byte,
	outbox OfflineSaleOutboxConfig,
) error {
	if s == nil {
		return ErrNotConfigured
	}
	if machineID == uuid.Nil {
		return errors.Join(ErrInvalidArgument, errors.New("machine_id required"))
	}
	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		return errors.Join(ErrInvalidArgument, errors.New("idempotency_key required"))
	}
	if strings.TrimSpace(clientEventID) == "" {
		return errors.Join(ErrInvalidArgument, errors.New("client_event_id required"))
	}

	wire, snap, err := parseOfflineSalePayload(payload)
	if err != nil {
		return err
	}
	if mid, err := uuid.Parse(strings.TrimSpace(wire.MachineID)); err == nil && mid != uuid.Nil && mid != machineID {
		return errors.Join(ErrInvalidArgument, errors.New("machine_id mismatch"))
	}
	currency := strings.ToUpper(strings.TrimSpace(wire.Currency))
	if currency == "" {
		currency = strings.ToUpper(strings.TrimSpace(snap.Currency))
	}
	if len(currency) != 3 {
		return errors.Join(ErrInvalidArgument, errors.New("currency must be a 3-letter ISO code"))
	}
	payable := wire.PayableTotalMinor
	if payable <= 0 {
		payable = snap.PayableTotalMinor
	}
	if payable <= 0 {
		return errors.Join(ErrInvalidArgument, errors.New("payable_total_minor must be positive"))
	}
	cashReceived := wire.CashReceivedMinor
	if cashReceived <= 0 {
		cashReceived = payable
	}

	pricingSnap := machinePricingSnapshotFromAppCheckout(snap, payable)
	orderID, err := s.createOfflineOrderFromSnapshot(ctx, machineID, snap, pricingSnap, currency, key)
	if err != nil {
		return err
	}

	cashKey := key + ":offline:cash"
	_, err = s.ConfirmCashPayment(ctx, ConfirmCashPaymentInput{
		OrderID:             orderID,
		MachineID:           machineID,
		IdempotencyKey:      cashKey,
		GrossAcceptedMinor:  cashReceived,
		AllocatedMinor:      payable,
		Currency:            currency,
		ConsentSource:       "implicit_post_order",
		OutboxTopic:         strings.TrimSpace(outbox.Topic),
		OutboxEventType:     strings.TrimSpace(outbox.EventType),
		OutboxAggregateType: strings.TrimSpace(outbox.AggregateType),
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *Service) createOfflineOrderFromSnapshot(
	ctx context.Context,
	machineID uuid.UUID,
	snap appCheckoutPricingSnapshot,
	pricingSnap MachinePricingSnapshotInput,
	currency string,
	key string,
) (uuid.UUID, error) {
	if len(snap.Lines) == 1 {
		line := snap.Lines[0]
		productID, err := uuid.Parse(strings.TrimSpace(line.ProductID))
		if err != nil || productID == uuid.Nil {
			return uuid.Nil, errors.Join(ErrInvalidArgument, errors.New("invalid product_id in pricing_snapshot"))
		}
		slotCode := strings.TrimSpace(line.SlotCode)
		identity, err := s.resolveCheckoutSaleLine(ctx, ResolveSaleLineInput{
			MachineID: machineID,
			ProductID: productID,
			SlotCode:  slotCode,
		}, &pricingSnap, 1)
		if err != nil {
			return uuid.Nil, err
		}
		slotID := identity.SlotConfigID
		slotIdx := identity.SlotIndex
		coKey := key + ":offline:create_order"
		createOut, err := s.CreateOrder(ctx, CreateOrderInput{
			MachineID:       machineID,
			ProductID:       productID,
			SlotID:          &slotID,
			CabinetCode:     identity.CabinetCode,
			SlotCode:        identity.SlotCode,
			SlotIndex:       &slotIdx,
			Currency:        currency,
			IdempotencyKey:  coKey,
			PricingSnapshot: &pricingSnap,
		})
		if err != nil {
			return uuid.Nil, err
		}
		return createOut.Order.ID, nil
	}

	quoteLines := make([]QuoteLineInput, 0, len(snap.Lines))
	for _, line := range snap.Lines {
		productID, err := uuid.Parse(strings.TrimSpace(line.ProductID))
		if err != nil || productID == uuid.Nil {
			return uuid.Nil, errors.Join(ErrInvalidArgument, errors.New("invalid product_id in pricing_snapshot"))
		}
		slotCode := strings.TrimSpace(line.SlotCode)
		qty := int32(line.Quantity)
		if qty <= 0 {
			qty = 1
		}
		identity, err := s.resolveCheckoutSaleLine(ctx, ResolveSaleLineInput{
			MachineID: machineID,
			ProductID: productID,
			SlotCode:  slotCode,
		}, &pricingSnap, qty)
		if err != nil {
			return uuid.Nil, err
		}
		slotID := identity.SlotConfigID
		slotIdx := identity.SlotIndex
		quoteLines = append(quoteLines, QuoteLineInput{
			ProductID:   productID,
			SlotID:      &slotID,
			CabinetCode: identity.CabinetCode,
			SlotCode:    identity.SlotCode,
			SlotIndex:   &slotIdx,
			Quantity:    qty,
		})
	}
	quoteKey := key + ":offline:create_quote"
	quoteOut, err := s.CreateQuote(ctx, CreateQuoteInput{
		MachineID:       machineID,
		Currency:        currency,
		PaymentMethod:   "cash",
		Lines:           quoteLines,
		IdempotencyKey:  quoteKey,
		PricingSnapshot: &pricingSnap,
	})
	if err != nil {
		return uuid.Nil, err
	}
	coKey := key + ":offline:create_order"
	orderOut, err := s.CreateOrderFromQuote(ctx, CreateOrderFromQuoteInput{
		MachineID:       machineID,
		QuoteID:         quoteOut.QuoteID,
		PaymentMethod:   "cash",
		IdempotencyKey:  coKey,
		PricingSnapshot: &pricingSnap,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return orderOut.OrderID, nil
}

func parseOfflineSalePayload(payload []byte) (OfflineSaleWirePayload, appCheckoutPricingSnapshot, error) {
	if len(payload) == 0 {
		return OfflineSaleWirePayload{}, appCheckoutPricingSnapshot{}, errors.Join(ErrInvalidArgument, errors.New("offline_sale payload required"))
	}
	var wire OfflineSaleWirePayload
	if err := json.Unmarshal(payload, &wire); err != nil {
		return OfflineSaleWirePayload{}, appCheckoutPricingSnapshot{}, errors.Join(ErrInvalidArgument, fmt.Errorf("invalid offline_sale payload: %w", err))
	}
	rawSnap := wire.PricingSnapshot
	if len(rawSnap) == 0 {
		return OfflineSaleWirePayload{}, appCheckoutPricingSnapshot{}, errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot required"))
	}
	if rawSnap[0] == '"' {
		var inner string
		if err := json.Unmarshal(rawSnap, &inner); err != nil {
			return OfflineSaleWirePayload{}, appCheckoutPricingSnapshot{}, errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot string invalid"))
		}
		rawSnap = []byte(inner)
	}
	var snap appCheckoutPricingSnapshot
	if err := json.Unmarshal(rawSnap, &snap); err != nil {
		return OfflineSaleWirePayload{}, appCheckoutPricingSnapshot{}, errors.Join(ErrInvalidArgument, fmt.Errorf("invalid pricing_snapshot: %w", err))
	}
	if len(snap.Lines) == 0 {
		return OfflineSaleWirePayload{}, appCheckoutPricingSnapshot{}, errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot lines required"))
	}
	for i, line := range snap.Lines {
		if strings.TrimSpace(line.ProductID) == "" {
			return OfflineSaleWirePayload{}, appCheckoutPricingSnapshot{}, errors.Join(
				ErrInvalidArgument,
				fmt.Errorf("pricing_snapshot product_id required at line %d", i),
			)
		}
	}
	return wire, snap, nil
}

func machinePricingSnapshotFromAppCheckout(snap appCheckoutPricingSnapshot, payable int64) MachinePricingSnapshotInput {
	snapshotID := strings.TrimSpace(snap.SnapshotID)
	if snapshotID == "" && len(snap.Lines) > 0 {
		snapshotID = strings.TrimSpace(snap.Lines[0].SlotCode)
	}
	var captured time.Time
	if snap.CapturedAtEpochMs > 0 {
		captured = time.UnixMilli(snap.CapturedAtEpochMs).UTC()
	}
	lines := make([]MachinePricingSnapshotLineInput, 0, len(snap.Lines))
	var unitPrice int64
	for i, line := range snap.Lines {
		qty := line.Quantity
		if qty <= 0 {
			qty = 1
		}
		unit := line.UnitPriceMinor
		if unit <= 0 && len(snap.Lines) == 1 {
			unit = payable / int64(qty)
		}
		lineSubtotal := unit * int64(qty)
		if unit <= 0 {
			lineSubtotal = payable / int64(len(snap.Lines))
			unit = lineSubtotal / int64(qty)
		}
		if i == 0 {
			unitPrice = unit
		}
		productID, _ := uuid.Parse(strings.TrimSpace(line.ProductID))
		lines = append(lines, MachinePricingSnapshotLineInput{
			LineSequence:      int32(i + 1),
			ProductID:         productID,
			SlotCode:          strings.TrimSpace(line.SlotCode),
			Quantity:          int32(qty),
			UnitPriceMinor:    unit,
			LineSubtotalMinor: lineSubtotal,
		})
	}
	return MachinePricingSnapshotInput{
		SubtotalMinor:        payable,
		TaxMinor:             0,
		TotalMinor:           payable,
		UnitPriceMinor:       unitPrice,
		LocalPricingRevision: snap.LocalPricingRevision,
		PricingFingerprint:   snapshotID,
		CapturedAt:           captured,
		SnapshotID:           snapshotID,
		SlotConfigVersion:    int64(snap.SlotConfigVersion),
		Lines:                lines,
	}
}
