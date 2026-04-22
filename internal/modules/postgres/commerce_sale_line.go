package postgres

import (
	"context"
	"errors"
	"strings"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
)

func rowSlotIndex(r db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow) (int32, bool) {
	if !r.SlotIndex.Valid {
		return 0, false
	}
	return r.SlotIndex.Int32, true
}

func rowProductUUID(r db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow) (uuid.UUID, bool) {
	p := pgUUIDToPtr(r.ProductID)
	if p == nil {
		return uuid.Nil, false
	}
	return *p, true
}

func saleLineFromRow(r db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow) (appcommerce.ResolvedSaleLine, error) {
	si, ok := rowSlotIndex(r)
	if !ok {
		return appcommerce.ResolvedSaleLine{}, errors.New("postgres: slot config has no slot_index")
	}
	price := r.PriceMinor
	return appcommerce.ResolvedSaleLine{
		SlotConfigID:  r.ID,
		CabinetCode:   r.CabinetCode,
		SlotCode:      r.SlotCode,
		SlotIndex:     si,
		PriceMinor:    price,
		SubtotalMinor: price,
		TaxMinor:      0,
		TotalMinor:    price,
	}, nil
}

func matchRow(in appcommerce.ResolveSaleLineInput, r db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow) bool {
	pid, ok := rowProductUUID(r)
	if !ok || pid != in.ProductID {
		return false
	}
	switch {
	case in.SlotID != nil && *in.SlotID != uuid.Nil:
		return r.ID == *in.SlotID
	case strings.TrimSpace(in.CabinetCode) != "" && strings.TrimSpace(in.SlotCode) != "":
		return strings.TrimSpace(r.CabinetCode) == strings.TrimSpace(in.CabinetCode) &&
			strings.TrimSpace(r.SlotCode) == strings.TrimSpace(in.SlotCode)
	case in.SlotIndex != nil && *in.SlotIndex >= 0:
		si, ok := rowSlotIndex(r)
		return ok && si == *in.SlotIndex
	default:
		return false
	}
}

// ResolveSaleLine implements appcommerce.SaleLineResolver.
func (s *Store) ResolveSaleLine(ctx context.Context, in appcommerce.ResolveSaleLineInput) (appcommerce.ResolvedSaleLine, error) {
	q := db.New(s.pool)
	ok, err := q.CommerceIsProductInMachinePublishedAssortment(ctx, db.CommerceIsProductInMachinePublishedAssortmentParams{
		ID:             in.MachineID,
		OrganizationID: in.OrganizationID,
		ProductID:      in.ProductID,
	})
	if err != nil {
		return appcommerce.ResolvedSaleLine{}, err
	}
	if !ok {
		return appcommerce.ResolvedSaleLine{}, errors.Join(appcommerce.ErrInvalidArgument, errors.New("product is not in the machine's published assortment"))
	}
	rows, err := q.InventoryAdminListCurrentMachineSlotConfigsByMachine(ctx, in.MachineID)
	if err != nil {
		return appcommerce.ResolvedSaleLine{}, err
	}
	var hits []db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow
	for _, r := range rows {
		if r.OrganizationID != in.OrganizationID {
			continue
		}
		if matchRow(in, r) {
			hits = append(hits, r)
		}
	}
	switch len(hits) {
	case 0:
		return appcommerce.ResolvedSaleLine{}, errors.Join(appcommerce.ErrInvalidArgument, errors.New("no matching current slot config for this product and slot selector"))
	case 1:
		return saleLineFromRow(hits[0])
	default:
		return appcommerce.ResolvedSaleLine{}, errors.Join(appcommerce.ErrInvalidArgument, errors.New("ambiguous slot selector; multiple slot configs matched"))
	}
}

// LookupSlotDisplay returns current slot identity for an order line without re-checking assortment (replay / read enrichment).
func (s *Store) LookupSlotDisplay(ctx context.Context, organizationID, machineID, productID uuid.UUID, slotIndex int32) (appcommerce.ResolvedSaleLine, error) {
	q := db.New(s.pool)
	rows, err := q.InventoryAdminListCurrentMachineSlotConfigsByMachine(ctx, machineID)
	if err != nil {
		return appcommerce.ResolvedSaleLine{}, err
	}
	for _, r := range rows {
		if r.OrganizationID != organizationID {
			continue
		}
		pid, ok := rowProductUUID(r)
		if !ok || pid != productID {
			continue
		}
		si, ok := rowSlotIndex(r)
		if !ok || si != slotIndex {
			continue
		}
		return saleLineFromRow(r)
	}
	return appcommerce.ResolvedSaleLine{}, errors.Join(appcommerce.ErrNotFound, errors.New("slot display row not found"))
}
