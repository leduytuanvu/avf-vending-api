package commerce

import (
	"context"
)

func (s *Service) resolveCreateQuoteSaleLines(ctx context.Context, in CreateQuoteInput) ([]ResolvedSaleLine, error) {
	if len(in.Lines) == 0 {
		return nil, ErrInvalidArgument
	}
	if in.PricingSnapshot != nil {
		snap := *in.PricingSnapshot
		if err := validateMachinePricingSnapshotMultiLine(snap, len(in.Lines)); err == nil {
			resolved, snapErr := s.resolveQuoteLinesFromMachineSnapshot(ctx, in, snap)
			if snapErr == nil {
				return resolved, nil
			}
			if !IsAssortmentStaleError(snapErr) {
				return nil, snapErr
			}
		}
	}
	if batch, ok := s.saleLines.(BatchQuoteSaleLineResolver); ok {
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
		resolved, err := batch.ResolveQuoteSaleLines(ctx, in.MachineID, inputs)
		if err == nil {
			return resolved, nil
		}
		if in.PricingSnapshot == nil || !IsAssortmentStaleError(err) {
			return nil, err
		}
	}
	out := make([]ResolvedSaleLine, len(in.Lines))
	for i, line := range in.Lines {
		qty := line.Quantity
		if qty <= 0 {
			qty = 1
		}
		resIn := ResolveSaleLineInput{
			MachineID:   in.MachineID,
			ProductID:   line.ProductID,
			SlotID:      line.SlotID,
			CabinetCode: line.CabinetCode,
			SlotCode:    line.SlotCode,
			SlotIndex:   line.SlotIndex,
		}
		resolved, err := s.resolveCheckoutSaleLine(ctx, resIn, in.PricingSnapshot, qty)
		if err != nil {
			return nil, err
		}
		out[i] = resolved
	}
	return out, nil
}
