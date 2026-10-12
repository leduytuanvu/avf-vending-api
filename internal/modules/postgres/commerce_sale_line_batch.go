package postgres

import (
	"context"
	"errors"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/app/pricingengine"
	"github.com/google/uuid"
)

// ResolveQuoteSaleLines implements appcommerce.BatchQuoteSaleLineResolver.
func (s *Store) ResolveQuoteSaleLines(
	ctx context.Context,
	machineID uuid.UUID,
	lines []appcommerce.QuoteSaleLineResolveInput,
) ([]appcommerce.ResolvedSaleLine, error) {
	if s == nil || s.pool == nil {
		return nil, errors.New("postgres: nil store")
	}
	if machineID == uuid.Nil || len(lines) == 0 {
		return nil, errors.Join(appcommerce.ErrInvalidArgument, errors.New("machine_id and lines required"))
	}
	eng := s.pricing
	if eng == nil {
		eng = pricingengine.New(s.pool)
	}
	at := pricingengine.NowUTC()
	reqs := make([]pricingengine.EvaluateSaleLineRequest, len(lines))
	for i, line := range lines {
		qty := line.Qty
		if qty <= 0 {
			qty = 1
		}
		reqs[i] = pricingengine.EvaluateSaleLineRequest{
			Selector: pricingengine.SaleLineSelector{
				MachineID:   machineID,
				ProductID:   line.In.ProductID,
				SlotID:      line.In.SlotID,
				CabinetCode: line.In.CabinetCode,
				SlotCode:    line.In.SlotCode,
				SlotIndex:   line.In.SlotIndex,
			},
			Qty: qty,
		}
	}
	evaluated, err := eng.EvaluateSaleLines(ctx, reqs, at.UTC())
	if err != nil {
		return nil, err
	}
	out := make([]appcommerce.ResolvedSaleLine, len(evaluated))
	for i, res := range evaluated {
		si, ok := slotIdxFromInventoryRow(res.Row)
		if !ok {
			return nil, errors.New("postgres: slot config has no slot_index")
		}
		out[i] = pricingengine.MapToResolvedSaleLine(res.Row.ID, res.Row.CabinetCode, res.Row.SlotCode, si, res.Line)
	}
	return out, nil
}
