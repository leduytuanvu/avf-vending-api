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

	wire, snap, line, err := parseOfflineSalePayload(payload)
	if err != nil {
		return err
	}
	if mid, err := uuid.Parse(strings.TrimSpace(wire.MachineID)); err == nil && mid != uuid.Nil && mid != machineID {
		return errors.Join(ErrInvalidArgument, errors.New("machine_id mismatch"))
	}
	productID, err := uuid.Parse(strings.TrimSpace(line.ProductID))
	if err != nil || productID == uuid.Nil {
		return errors.Join(ErrInvalidArgument, errors.New("invalid product_id in pricing_snapshot"))
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

	pricingSnap := machinePricingSnapshotFromAppCheckout(snap, line, payable)
	slotCode := strings.TrimSpace(line.SlotCode)
	identity, err := s.saleLines.ResolveSaleLine(ctx, ResolveSaleLineInput{
		MachineID: machineID,
		ProductID: productID,
		SlotCode:  slotCode,
	})
	if err != nil {
		return err
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
		return err
	}
	orderID := createOut.Order.ID

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

func parseOfflineSalePayload(payload []byte) (OfflineSaleWirePayload, appCheckoutPricingSnapshot, appCheckoutLine, error) {
	if len(payload) == 0 {
		return OfflineSaleWirePayload{}, appCheckoutPricingSnapshot{}, appCheckoutLine{}, errors.Join(ErrInvalidArgument, errors.New("offline_sale payload required"))
	}
	var wire OfflineSaleWirePayload
	if err := json.Unmarshal(payload, &wire); err != nil {
		return OfflineSaleWirePayload{}, appCheckoutPricingSnapshot{}, appCheckoutLine{}, errors.Join(ErrInvalidArgument, fmt.Errorf("invalid offline_sale payload: %w", err))
	}
	rawSnap := wire.PricingSnapshot
	if len(rawSnap) == 0 {
		return OfflineSaleWirePayload{}, appCheckoutPricingSnapshot{}, appCheckoutLine{}, errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot required"))
	}
	if rawSnap[0] == '"' {
		var inner string
		if err := json.Unmarshal(rawSnap, &inner); err != nil {
			return OfflineSaleWirePayload{}, appCheckoutPricingSnapshot{}, appCheckoutLine{}, errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot string invalid"))
		}
		rawSnap = []byte(inner)
	}
	var snap appCheckoutPricingSnapshot
	if err := json.Unmarshal(rawSnap, &snap); err != nil {
		return OfflineSaleWirePayload{}, appCheckoutPricingSnapshot{}, appCheckoutLine{}, errors.Join(ErrInvalidArgument, fmt.Errorf("invalid pricing_snapshot: %w", err))
	}
	if len(snap.Lines) == 0 {
		return OfflineSaleWirePayload{}, appCheckoutPricingSnapshot{}, appCheckoutLine{}, errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot lines required"))
	}
	line := snap.Lines[0]
	if strings.TrimSpace(line.ProductID) == "" {
		return OfflineSaleWirePayload{}, appCheckoutPricingSnapshot{}, appCheckoutLine{}, errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot product_id required"))
	}
	return wire, snap, line, nil
}

func machinePricingSnapshotFromAppCheckout(snap appCheckoutPricingSnapshot, line appCheckoutLine, payable int64) MachinePricingSnapshotInput {
	snapshotID := strings.TrimSpace(snap.SnapshotID)
	if snapshotID == "" {
		snapshotID = strings.TrimSpace(line.SlotCode)
	}
	unit := line.UnitPriceMinor
	if unit <= 0 {
		unit = payable
	}
	qty := line.Quantity
	if qty <= 0 {
		qty = 1
	}
	lineSubtotal := unit * int64(qty)
	var captured time.Time
	if snap.CapturedAtEpochMs > 0 {
		captured = time.UnixMilli(snap.CapturedAtEpochMs).UTC()
	}
	productID, _ := uuid.Parse(strings.TrimSpace(line.ProductID))
	return MachinePricingSnapshotInput{
		SubtotalMinor:        payable,
		TaxMinor:             0,
		TotalMinor:           payable,
		UnitPriceMinor:       unit,
		LocalPricingRevision: snap.LocalPricingRevision,
		PricingFingerprint:   snapshotID,
		CapturedAt:           captured,
		SnapshotID:           snapshotID,
		SlotConfigVersion:    int64(snap.SlotConfigVersion),
		Lines: []MachinePricingSnapshotLineInput{
			{
				LineSequence:      1,
				ProductID:         productID,
				SlotCode:          strings.TrimSpace(line.SlotCode),
				Quantity:          int32(qty),
				UnitPriceMinor:    unit,
				LineSubtotalMinor: lineSubtotal,
			},
		},
	}
}
