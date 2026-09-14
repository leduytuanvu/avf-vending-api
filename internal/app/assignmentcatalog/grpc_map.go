package assignmentcatalog

import (
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func MapSnapshotProto(snap *Snapshot) *machinev1.AssignmentCatalogSnapshot {
	if snap == nil {
		return nil
	}
	out := &machinev1.AssignmentCatalogSnapshot{
		MachineId:      snap.MachineID.String(),
		CatalogVersion: snap.CatalogVersion,
		GeneratedAt:    timestamppb.New(snap.GeneratedAt.UTC()),
		ProductCount:   int32(len(snap.Products)),
		Products:       make([]*machinev1.AssignmentCatalogProduct, 0, len(snap.Products)),
	}
	for _, p := range snap.Products {
		out.Products = append(out.Products, mapProductProto(p))
	}
	return out
}

func mapProductProto(p Product) *machinev1.AssignmentCatalogProduct {
	return &machinev1.AssignmentCatalogProduct{
		ProductId:            p.ID.String(),
		Sku:                  p.SKU,
		Barcode:              p.Barcode,
		Name:                 p.Name,
		ShortName:            p.ShortName,
		Description:          p.Description,
		IsActive:             p.Active,
		BasePriceMinor:       p.BasePriceMinor,
		Currency:             p.Currency,
		ImageKey:             p.ImageKey,
		ImageHash:            p.ImageHash,
		ThumbUrl:             p.ThumbURL,
		DisplayUrl:           p.DisplayURL,
		ImageContentRevision: p.ImageContentRevision,
	}
}

func MapDeltaProductsProto(products []Product) []*machinev1.AssignmentCatalogProduct {
	out := make([]*machinev1.AssignmentCatalogProduct, 0, len(products))
	for _, p := range products {
		out = append(out, mapProductProto(p))
	}
	return out
}

func MapDeletedProductIDsProto(ids []uuid.UUID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.String())
	}
	return out
}
