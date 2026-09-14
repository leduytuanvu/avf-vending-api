package grpcserver

import (
	"context"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/productmastercatalog"
	"github.com/avf/avf-vending-api/internal/gen/db"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type machineProductMasterCatalogServer struct {
	machinev1.UnimplementedMachineProductMasterCatalogServiceServer
	deps MachineGRPCServicesDeps
}

func (s *machineProductMasterCatalogServer) GetProductMasterCatalogSnapshot(
	ctx context.Context,
	req *machinev1.GetProductMasterCatalogSnapshotRequest,
) (*machinev1.GetProductMasterCatalogSnapshotResponse, error) {
	claims, ok := plauth.MachineAccessClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing machine credentials")
	}
	if s.deps.Pool != nil {
		q := db.New(s.deps.Pool)
		if err := machineCredentialGate(ctx, q, claims); err != nil {
			return nil, err
		}
	}
	if _, err := resolveMachineScope(claims.MachineID, req.GetMachineId()); err != nil {
		return nil, err
	}
	if s.deps.ProductMasterCatalog == nil {
		return nil, status.Error(codes.Unavailable, "product master catalog not configured")
	}
	snap, nextToken, err := s.deps.ProductMasterCatalog.BuildSnapshot(ctx, req.GetPageSize(), req.GetPageToken())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	rid := ""
	if req.GetMeta() != nil {
		rid = req.GetMeta().GetRequestId()
	}
	return &machinev1.GetProductMasterCatalogSnapshotResponse{
		Snapshot:      productMasterSnapshotProto(snap),
		NextPageToken: nextToken,
		Meta:          responseMetaCtx(ctx, rid, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED),
	}, nil
}

func (s *machineProductMasterCatalogServer) GetProductMasterCatalogDelta(
	ctx context.Context,
	req *machinev1.GetProductMasterCatalogDeltaRequest,
) (*machinev1.GetProductMasterCatalogDeltaResponse, error) {
	claims, ok := plauth.MachineAccessClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing machine credentials")
	}
	if s.deps.Pool != nil {
		q := db.New(s.deps.Pool)
		if err := machineCredentialGate(ctx, q, claims); err != nil {
			return nil, err
		}
	}
	if _, err := resolveMachineScope(claims.MachineID, req.GetMachineId()); err != nil {
		return nil, err
	}
	if s.deps.ProductMasterCatalog == nil {
		return nil, status.Error(codes.Unavailable, "product master catalog not configured")
	}
	basisIDs := make([]uuid.UUID, 0, len(req.GetBasisProductIds()))
	for _, raw := range req.GetBasisProductIds() {
		id, perr := uuid.Parse(strings.TrimSpace(raw))
		if perr != nil || id == uuid.Nil {
			continue
		}
		basisIDs = append(basisIDs, id)
	}
	delta, err := s.deps.ProductMasterCatalog.BuildDelta(ctx, req.GetBasisCatalogVersion(), basisIDs)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	rid := ""
	if req.GetMeta() != nil {
		rid = req.GetMeta().GetRequestId()
	}
	statusCode := machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED
	if delta.BasisMatches {
		statusCode = machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_NOT_MODIFIED
	}
	return &machinev1.GetProductMasterCatalogDeltaResponse{
		BasisMatches:                   delta.BasisMatches,
		BasisCatalogVersion:            delta.BasisCatalogVersion,
		ToCatalogVersion:               delta.ToCatalogVersion,
		TotalActiveCount:               delta.TotalActiveCount,
		Upserts:                        productMasterRecordsProto(delta.Upserts),
		DeletedOrDeactivatedProductIds: uuidStrings(delta.DeletedOrDeactivatedProductIDs),
		GeneratedAt:                    timestamppb.New(delta.GeneratedAt),
		ResetRequired:                  delta.ResetRequired,
		Meta:                           responseMetaCtx(ctx, rid, statusCode),
	}, nil
}

func productMasterSnapshotProto(snap productmastercatalog.Snapshot) *machinev1.ProductMasterCatalogSnapshot {
	if snap.CatalogVersion == "" && len(snap.Products) == 0 {
		return nil
	}
	return &machinev1.ProductMasterCatalogSnapshot{
		CatalogVersion:   snap.CatalogVersion,
		GeneratedAt:      timestamppb.New(snap.GeneratedAt),
		TotalActiveCount: snap.TotalActiveCount,
		Products:         productMasterRecordsProto(snap.Products),
	}
}

func productMasterRecordsProto(records []productmastercatalog.Record) []*machinev1.ProductMasterRecord {
	out := make([]*machinev1.ProductMasterRecord, 0, len(records))
	for _, r := range records {
		pm := &machinev1.ProductMasterRecord{
			ProductId:      r.ProductID.String(),
			Sku:            r.SKU,
			Barcode:        r.Barcode,
			Name:           r.Name,
			ShortName:      r.ShortName,
			Description:    r.Description,
			BasePriceMinor: r.BasePriceMinor,
			Currency:       r.Currency,
			Active:         r.Active,
		}
		if r.CategoryID != nil {
			pm.CategoryId = r.CategoryID.String()
		}
		if r.BrandID != nil {
			pm.BrandId = r.BrandID.String()
		}
		if r.Image != nil {
			pm.PrimaryMedia = &machinev1.ProductMediaRef{
				ThumbUrl:       r.Image.ThumbURL,
				DisplayUrl:     r.Image.DisplayURL,
				ChecksumSha256: r.Image.ContentHash,
				Etag:           r.Image.Etag,
				MediaVersion:   r.Image.MediaVersion,
				CacheKey:       r.Image.CacheKey,
			}
			if r.Image.MediaID != uuid.Nil {
				pm.PrimaryMedia.MediaId = r.Image.MediaID.String()
			}
		}
		out = append(out, pm)
	}
	return out
}

func uuidStrings(ids []uuid.UUID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			continue
		}
		out = append(out, id.String())
	}
	return out
}
