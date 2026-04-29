package grpcserver

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/salecatalog"
	"github.com/avf/avf-vending-api/internal/app/setupapp"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type machineCatalogServer struct {
	machinev1.UnimplementedMachineCatalogServiceServer
	deps MachineGRPCServicesDeps
}

type machineMediaServer struct {
	machinev1.UnimplementedMachineMediaServiceServer
	deps MachineGRPCServicesDeps
}

func (s *machineMediaServer) GetMediaManifest(ctx context.Context, req *machinev1.MachineMediaServiceGetMediaManifestRequest) (*machinev1.MachineMediaServiceGetMediaManifestResponse, error) {
	inner := &machinev1.GetMediaManifestRequest{}
	if req != nil {
		inner.MachineId = req.GetMachineId()
		inner.IncludeUnavailable = req.GetIncludeUnavailable()
		inner.Meta = req.GetMeta()
	}
	out, err := (&machineCatalogServer{deps: s.deps}).GetMediaManifest(ctx, inner)
	if err != nil {
		return nil, err
	}
	return &machinev1.MachineMediaServiceGetMediaManifestResponse{
		MachineId:        out.GetMachineId(),
		CatalogVersion:   out.GetCatalogVersion(),
		MediaFingerprint: out.GetMediaFingerprint(),
		GeneratedAt:      out.GetGeneratedAt(),
		Entries:          out.GetEntries(),
		Meta:             out.GetMeta(),
	}, nil
}

func (s *machineMediaServer) GetMediaDelta(ctx context.Context, req *machinev1.GetMediaDeltaRequest) (*machinev1.GetMediaDeltaResponse, error) {
	if req == nil {
		req = &machinev1.GetMediaDeltaRequest{}
	}
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
	machineID, err := resolveMachineScope(claims.MachineID, req.GetMachineId())
	if err != nil {
		return nil, err
	}
	if s.deps.SaleCatalog == nil {
		return nil, status.Error(codes.Unavailable, "sale catalog not configured")
	}
	snap, err := s.deps.SaleCatalog.BuildSnapshot(ctx, machineID, salecatalog.Options{
		IncludeUnavailable: req.GetIncludeUnavailable(),
		IncludeImages:      true,
	})
	if err != nil {
		return nil, mapSaleCatalogError(err)
	}
	salecatalog.RefreshPresignedProductMediaURLs(ctx, s.deps.MediaStore, s.deps.MediaPresignTTL, &snap)
	entries, mediaFP := mediaManifestEntriesFromSnapshot(snap)
	rid := ""
	if req.GetMeta() != nil {
		rid = req.GetMeta().GetRequestId()
	}
	basis := strings.TrimSpace(req.GetBasisMediaFingerprint())
	if basis != "" && basis == mediaFP {
		return &machinev1.GetMediaDeltaResponse{
			BasisMatches:      true,
			Meta:              responseMetaCtx(ctx, rid, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_NOT_MODIFIED),
			NextSyncToken:     mediaFP,
			ChangedEntries:    nil,
			DeletedProductIds: nil,
		}, nil
	}
	return &machinev1.GetMediaDeltaResponse{
		BasisMatches:      false,
		ChangedEntries:    entries,
		DeletedProductIds: nil,
		NextSyncToken:     mediaFP,
		Meta:              responseMetaCtx(ctx, rid, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED),
	}, nil
}

func (s *machineMediaServer) AckMediaVersion(ctx context.Context, req *machinev1.AckMediaVersionRequest) (*machinev1.AckMediaVersionResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
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
	rid := ""
	if req.GetMeta() != nil {
		rid = req.GetMeta().GetRequestId()
	}
	if s.deps.EnterpriseAudit != nil {
		fp := strings.TrimSpace(req.GetAcknowledgedMediaFingerprint())
		actorID := claims.MachineID.String()
		meta, _ := json.Marshal(map[string]any{"media_fingerprint": fp})
		if len(meta) == 0 {
			meta = []byte("{}")
		}
		_ = s.deps.EnterpriseAudit.Record(ctx, compliance.EnterpriseAuditRecord{
			OrganizationID: claims.OrganizationID,
			ActorType:      compliance.ActorMachine,
			ActorID:        &actorID,
			Action:         "machine.media.version_acknowledged",
			ResourceType:   "machine",
			ResourceID:     &actorID,
			Metadata:       meta,
		})
	}
	return &machinev1.AckMediaVersionResponse{
		Meta: responseMetaCtx(ctx, rid, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED),
	}, nil
}

func (s *machineCatalogServer) GetSaleCatalog(ctx context.Context, req *machinev1.GetCatalogSnapshotRequest) (*machinev1.GetCatalogSnapshotResponse, error) {
	return s.GetCatalogSnapshot(ctx, req)
}

func (s *machineCatalogServer) SyncSaleCatalog(ctx context.Context, req *machinev1.GetCatalogSnapshotRequest) (*machinev1.GetCatalogSnapshotResponse, error) {
	return s.GetCatalogSnapshot(ctx, req)
}

func (s *machineCatalogServer) GetCatalogSnapshot(ctx context.Context, req *machinev1.GetCatalogSnapshotRequest) (*machinev1.GetCatalogSnapshotResponse, error) {
	if req == nil {
		req = &machinev1.GetCatalogSnapshotRequest{}
	}
	claims, ok := plauth.MachineAccessClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing machine credentials")
	}
	machineID, err := resolveMachineScope(claims.MachineID, req.GetMachineId())
	if err != nil {
		return nil, err
	}
	if s.deps.Pool != nil {
		q := db.New(s.deps.Pool)
		if err := machineCredentialGate(ctx, q, claims); err != nil {
			return nil, err
		}
	}
	if s.deps.SaleCatalog == nil {
		return nil, status.Error(codes.Unavailable, "sale catalog not configured")
	}
	var ifNone *int64
	if req.IfNoneMatchConfigVersion != nil {
		v := req.GetIfNoneMatchConfigVersion()
		ifNone = &v
	}
	includeImages := true
	if req.IncludeImages != nil {
		includeImages = *req.IncludeImages
	}
	snap, err := s.deps.SaleCatalog.BuildSnapshot(ctx, machineID, salecatalog.Options{
		IncludeUnavailable:       req.GetIncludeUnavailable(),
		IncludeImages:            includeImages,
		IfNoneMatchConfigVersion: ifNone,
	})
	if err != nil {
		return nil, mapSaleCatalogError(err)
	}
	rid := ""
	if req.GetMeta() != nil {
		rid = req.GetMeta().GetRequestId()
	}
	if snap.NotModified {
		return &machinev1.GetCatalogSnapshotResponse{
			NotModified: true,
			Meta:        responseMetaCtx(ctx, rid, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_NOT_MODIFIED),
		}, nil
	}
	if includeImages && s.deps.MediaStore != nil && s.deps.MediaPresignTTL > 0 {
		salecatalog.RefreshPresignedProductMediaURLs(ctx, s.deps.MediaStore, s.deps.MediaPresignTTL, &snap)
	}
	return &machinev1.GetCatalogSnapshotResponse{
		Snapshot: snapshotProtoFromSale(snap),
		Meta:     responseMetaCtx(ctx, rid, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED),
	}, nil
}

func (s *machineCatalogServer) GetCatalogDelta(ctx context.Context, req *machinev1.GetCatalogDeltaRequest) (*machinev1.GetCatalogDeltaResponse, error) {
	if req == nil {
		req = &machinev1.GetCatalogDeltaRequest{}
	}
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
	machineID, err := resolveMachineScope(claims.MachineID, req.GetMachineId())
	if err != nil {
		return nil, err
	}
	if s.deps.SaleCatalog == nil {
		return nil, status.Error(codes.Unavailable, "sale catalog not configured")
	}
	snap, err := s.deps.SaleCatalog.BuildSnapshot(ctx, machineID, salecatalog.Options{
		IncludeUnavailable:       true,
		IncludeImages:            true,
		IfNoneMatchConfigVersion: nil,
	})
	if err != nil {
		return nil, mapSaleCatalogError(err)
	}
	if s.deps.MediaStore != nil && s.deps.MediaPresignTTL > 0 {
		salecatalog.RefreshPresignedProductMediaURLs(ctx, s.deps.MediaStore, s.deps.MediaPresignTTL, &snap)
	}
	rid := ""
	if req.GetMeta() != nil {
		rid = req.GetMeta().GetRequestId()
	}
	basis := strings.TrimSpace(req.GetBasisCatalogVersion())
	if basis != "" && basis == snap.CatalogVersion {
		return &machinev1.GetCatalogDeltaResponse{
			BasisMatches: true,
			Meta:         responseMetaCtx(ctx, rid, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_NOT_MODIFIED),
			Snapshot:     nil,
		}, nil
	}
	return &machinev1.GetCatalogDeltaResponse{
		BasisMatches: false,
		Snapshot:     snapshotProtoFromSale(snap),
		Meta:         responseMetaCtx(ctx, rid, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED),
	}, nil
}

func (s *machineCatalogServer) AckCatalogVersion(ctx context.Context, req *machinev1.AckCatalogVersionRequest) (*machinev1.AckCatalogVersionResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	claims, ok := plauth.MachineAccessClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing machine credentials")
	}
	if s.deps.Pool == nil {
		return nil, status.Error(codes.Unavailable, "database_not_configured")
	}
	q := db.New(s.deps.Pool)
	if err := machineCredentialGate(ctx, q, claims); err != nil {
		return nil, err
	}
	rid := ""
	if req.GetMeta() != nil {
		rid = req.GetMeta().GetRequestId()
	}
	if s.deps.EnterpriseAudit != nil {
		cv := strings.TrimSpace(req.GetAcknowledgedCatalogVersion())
		actorID := claims.MachineID.String()
		meta, _ := json.Marshal(map[string]any{"catalog_version": cv})
		if len(meta) == 0 {
			meta = []byte("{}")
		}
		_ = s.deps.EnterpriseAudit.Record(ctx, compliance.EnterpriseAuditRecord{
			OrganizationID: claims.OrganizationID,
			ActorType:      compliance.ActorMachine,
			ActorID:        &actorID,
			Action:         "machine.catalog.version_acknowledged",
			ResourceType:   "machine",
			ResourceID:     &actorID,
			Metadata:       meta,
		})
	}
	return &machinev1.AckCatalogVersionResponse{
		Meta: responseMetaCtx(ctx, rid, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED),
	}, nil
}

func (s *machineCatalogServer) GetMediaManifest(ctx context.Context, req *machinev1.GetMediaManifestRequest) (*machinev1.GetMediaManifestResponse, error) {
	if req == nil {
		req = &machinev1.GetMediaManifestRequest{}
	}
	claims, ok := plauth.MachineAccessClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing machine credentials")
	}
	machineID, err := resolveMachineScope(claims.MachineID, req.GetMachineId())
	if err != nil {
		return nil, err
	}
	if s.deps.Pool != nil {
		q := db.New(s.deps.Pool)
		if err := machineCredentialGate(ctx, q, claims); err != nil {
			return nil, err
		}
	}
	if s.deps.SaleCatalog == nil {
		return nil, status.Error(codes.Unavailable, "sale catalog not configured")
	}
	snap, err := s.deps.SaleCatalog.BuildSnapshot(ctx, machineID, salecatalog.Options{
		IncludeUnavailable: req.GetIncludeUnavailable(),
		IncludeImages:      true,
	})
	if err != nil {
		return nil, mapSaleCatalogError(err)
	}
	salecatalog.RefreshPresignedProductMediaURLs(ctx, s.deps.MediaStore, s.deps.MediaPresignTTL, &snap)
	entries, mediaFP := mediaManifestEntriesFromSnapshot(snap)
	maxEntries := 5000
	if s.deps.Config != nil {
		maxEntries = s.deps.Config.Capacity.MaxMediaManifestEntries
	}
	if len(entries) > maxEntries {
		return nil, status.Errorf(codes.ResourceExhausted, "media manifest too large (%d entries max %d)", len(entries), maxEntries)
	}
	rid := ""
	if req.GetMeta() != nil {
		rid = req.GetMeta().GetRequestId()
	}
	return &machinev1.GetMediaManifestResponse{
		MachineId:        machineID.String(),
		CatalogVersion:   snap.CatalogVersion,
		MediaFingerprint: mediaFP,
		GeneratedAt:      timestamppb.New(snap.GeneratedAt),
		Entries:          entries,
		Meta:             responseMetaCtx(ctx, rid, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED),
	}, nil
}

func mediaManifestEntriesFromSnapshot(snap salecatalog.Snapshot) ([]*machinev1.MediaManifestEntry, string) {
	mediaFP := salecatalog.MediaFingerprint(snap)
	seen := make(map[uuid.UUID]struct{})
	entries := make([]*machinev1.MediaManifestEntry, 0)
	for _, it := range snap.Items {
		if _, dup := seen[it.ProductID]; dup {
			continue
		}
		seen[it.ProductID] = struct{}{}
		if it.Image == nil {
			continue
		}
		pm := productMediaRefProto(it.Image)
		entries = append(entries, &machinev1.MediaManifestEntry{
			ProductId:    it.ProductID.String(),
			Sku:          it.SKU,
			PrimaryMedia: pm,
			MediaId:      pm.GetMediaId(),
		})
	}
	return entries, mediaFP
}

func resolveMachineScope(tokenMachine uuid.UUID, requestMachine string) (uuid.UUID, error) {
	requestMachine = strings.TrimSpace(requestMachine)
	if requestMachine == "" {
		return tokenMachine, nil
	}
	rid, err := uuid.Parse(requestMachine)
	if err != nil || rid == uuid.Nil {
		return uuid.Nil, status.Error(codes.InvalidArgument, "invalid machine_id")
	}
	if rid != tokenMachine {
		return uuid.Nil, status.Error(codes.PermissionDenied, "machine scope mismatch")
	}
	return tokenMachine, nil
}

func mapSaleCatalogError(err error) error {
	switch err {
	case nil:
		return nil
	case setupapp.ErrNotFound:
		return status.Error(codes.NotFound, "machine_not_found")
	case setupapp.ErrMachineNotEligibleForBootstrap:
		return status.Error(codes.PermissionDenied, "machine_not_eligible")
	default:
		return status.Error(codes.Internal, "internal")
	}
}

func snapshotProtoFromSale(s salecatalog.Snapshot) *machinev1.CatalogSnapshot {
	// CatalogSnapshot.catalog_version carries the composite RuntimeSaleCatalogFingerprint (P1 catalog_fingerprint).
	// Clients should key incremental sync off this string with the same IncludeUnavailable/IncludeImages as the request.
	items := make([]*machinev1.CatalogSlotItem, 0, len(s.Items))
	for _, it := range s.Items {
		csi := &machinev1.CatalogSlotItem{
			SlotIndex:         it.SlotIndex,
			SlotCode:          it.SlotCode,
			CabinetCode:       it.CabinetCode,
			ProductId:         it.ProductID.String(),
			Sku:               it.SKU,
			Name:              it.Name,
			ShortName:         it.ShortName,
			PriceMinor:        it.PriceMinor,
			AvailableQuantity: it.AvailableQuantity,
			MaxQuantity:       it.MaxQuantity,
			IsAvailable:       it.IsAvailable,
			UnavailableReason: it.UnavailableReason,
			SortOrder:         it.SortOrder,
		}
		csi.PrimaryMedia = productMediaRefProto(it.Image)
		items = append(items, csi)
	}
	return &machinev1.CatalogSnapshot{
		MachineId:      s.MachineID.String(),
		OrganizationId: s.OrganizationID.String(),
		SiteId:         s.SiteID.String(),
		ConfigVersion:  s.ConfigVersion,
		CatalogVersion: s.CatalogVersion,
		Currency:       s.Currency,
		GeneratedAt:    timestamppb.New(s.GeneratedAt),
		Items:          items,
	}
}

func productMediaRefProto(im *salecatalog.ImageMeta) *machinev1.ProductMediaRef {
	if im == nil {
		return nil
	}
	if im.Deleted {
		return &machinev1.ProductMediaRef{Deleted: true}
	}
	pm := &machinev1.ProductMediaRef{
		ThumbUrl:       im.ThumbURL,
		DisplayUrl:     im.DisplayURL,
		ChecksumSha256: im.ContentHash,
		Etag:           im.Etag,
		UpdatedAt:      timestamppb.New(im.UpdatedAt),
		SizeBytes:      im.SizeBytes,
		ObjectVersion:  im.ObjectVersion,
		MediaVersion:   im.MediaVersion,
		Width:          im.Width,
		Height:         im.Height,
		ContentType:    im.ContentType,
	}
	if im.MediaID != uuid.Nil {
		pm.MediaId = im.MediaID.String()
	}
	if !im.URLExpiresAt.IsZero() {
		pm.ExpiresAt = timestamppb.New(im.URLExpiresAt)
	}
	pm.MediaVariants = mediaVariantsProto(im)
	return pm
}

func mediaVariantsProto(im *salecatalog.ImageMeta) []*machinev1.ProductMediaVariant {
	if im == nil || len(im.Variants) == 0 {
		return nil
	}
	out := make([]*machinev1.ProductMediaVariant, 0, len(im.Variants))
	for _, v := range im.Variants {
		pv := &machinev1.ProductMediaVariant{
			Kind:           machinev1.MediaVariantKind(v.Kind),
			Url:            v.URL,
			ContentType:    v.ContentType,
			ChecksumSha256: v.ChecksumSHA256,
			Etag:           v.Etag,
			SizeBytes:      v.SizeBytes,
			Width:          v.Width,
			Height:         v.Height,
			MediaVersion:   v.MediaVersion,
			UpdatedAt:      timestamppb.New(v.UpdatedAt),
		}
		if v.MediaAssetID != uuid.Nil {
			pv.MediaAssetId = v.MediaAssetID.String()
		}
		if !im.URLExpiresAt.IsZero() {
			pv.ExpiresAt = timestamppb.New(im.URLExpiresAt)
		}
		out = append(out, pv)
	}
	return out
}
