package commerce

import (
	"context"
)

func (s *Service) resolveQuoteLinesFromMachineSnapshot(
	ctx context.Context,
	in CreateQuoteInput,
	snap MachinePricingSnapshotInput,
) ([]ResolvedSaleLine, error) {
	if s == nil || s.saleLines == nil {
		return nil, ErrNotConfigured
	}
	resolver, ok := s.saleLines.(SnapshotQuoteTopologyResolver)
	if !ok {
		return nil, ErrNotConfigured
	}
	inputs := make([]QuoteSaleLineResolveInput, len(in.Lines))
	for i, line := range in.Lines {
		qty := line.Quantity
		if qty <= 0 {
			qty = 1
		}
		inputs[i] = QuoteSaleLineResolveInput{
			In: ResolveSaleLineInput{
				MachineID:   in.MachineID,
				ProductID:   line.ProductID,
				SlotID:      line.SlotID,
				CabinetCode: line.CabinetCode,
				SlotCode:    line.SlotCode,
				SlotIndex:   line.SlotIndex,
			},
			Qty: qty,
		}
	}
	return resolver.ResolveQuoteLinesFromMachineSnapshot(ctx, in.MachineID, inputs, snap)
}
