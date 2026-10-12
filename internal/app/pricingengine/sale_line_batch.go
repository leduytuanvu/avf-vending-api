package pricingengine

import (
	"context"
	"errors"
	"time"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
)

// EvaluateSaleLineRequest is one cart line to price in a batched quote evaluation.
type EvaluateSaleLineRequest struct {
	Selector SaleLineSelector
	Qty      int32
}

// EvaluateSaleLineResult is pricing output plus the matched slot config row.
type EvaluateSaleLineResult struct {
	Line LinePriceResult
	Row  db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow
}

// EvaluateSaleLines loads machine slot configs once and reuses one pricing [Batch] for every line.
func (e *Engine) EvaluateSaleLines(ctx context.Context, reqs []EvaluateSaleLineRequest, at time.Time) ([]EvaluateSaleLineResult, error) {
	if e == nil || e.pool == nil {
		return nil, errors.New("pricingengine: nil engine")
	}
	if len(reqs) == 0 {
		return nil, errors.Join(appcommerce.ErrInvalidArgument, errors.New("at least one sale line required"))
	}
	machineID := reqs[0].Selector.MachineID
	if machineID == uuid.Nil {
		return nil, errors.Join(appcommerce.ErrInvalidArgument, errors.New("machine_id required"))
	}
	for i := 1; i < len(reqs); i++ {
		if reqs[i].Selector.MachineID != machineID {
			return nil, errors.Join(appcommerce.ErrInvalidArgument, errors.New("all sale lines must share machine_id"))
		}
	}
	q := db.New(e.pool)
	rows, err := q.InventoryAdminListCurrentMachineSlotConfigsByMachine(ctx, machineID)
	if err != nil {
		return nil, err
	}
	batch, err := e.newBatch(ctx, uuid.Nil, machineID, at)
	if err != nil {
		return nil, err
	}
	productIDs := make([]uuid.UUID, 0, len(reqs))
	seenProducts := make(map[uuid.UUID]struct{}, len(reqs))
	for _, req := range reqs {
		pid := req.Selector.ProductID
		if pid == uuid.Nil {
			continue
		}
		if _, ok := seenProducts[pid]; ok {
			continue
		}
		seenProducts[pid] = struct{}{}
		productIDs = append(productIDs, pid)
	}
	if len(productIDs) > 0 {
		if err := batch.PrefetchProductMeta(ctx, productIDs); err != nil {
			return nil, err
		}
	}
	out := make([]EvaluateSaleLineResult, len(reqs))
	for i, req := range reqs {
		in := req.Selector
		qty := req.Qty
		if qty <= 0 {
			qty = 1
		}
		ok, err := q.CommerceIsProductInMachinePublishedAssortment(ctx, db.CommerceIsProductInMachinePublishedAssortmentParams{
			ProductID: in.ProductID,
			ID:        machineID,
		})
		if err != nil {
			return nil, err
		}
		if !ok {
			repaired, repairErr := e.repairPublishedAssortmentFromSlotConfigs(ctx, machineID, in.ProductID)
			if repairErr != nil {
				return nil, repairErr
			}
			if repaired {
				ok, err = q.CommerceIsProductInMachinePublishedAssortment(ctx, db.CommerceIsProductInMachinePublishedAssortmentParams{
					ProductID: in.ProductID,
					ID:        machineID,
				})
				if err != nil {
					return nil, err
				}
			}
		}
		if !ok {
			return nil, errors.Join(appcommerce.ErrInvalidArgument, errors.New("product is not in the machine's published assortment"))
		}
		row, err := pickSlotRow(rows, uuid.Nil, in)
		if err != nil {
			return nil, err
		}
		si, ok := rowSlotIndex(row)
		if !ok {
			return nil, errors.New("pricingengine: slot config has no slot_index")
		}
		res, err := batch.PriceLine(ctx, PriceLineInput{
			MachineID:         machineID,
			ProductID:         in.ProductID,
			SlotListUnitMinor: row.PriceMinor,
			SlotConfigID:      row.ID,
			CabinetCode:       row.CabinetCode,
			SlotCode:          row.SlotCode,
			SlotIndex:         si,
			Quantity:          qty,
		})
		if err != nil {
			return nil, err
		}
		out[i] = EvaluateSaleLineResult{Line: res, Row: row}
	}
	return out, nil
}
