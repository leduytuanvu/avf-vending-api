package grpcserver

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/activation"
	"github.com/avf/avf-vending-api/internal/app/featureflags"
	"github.com/avf/avf-vending-api/internal/app/setupapp"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const actionMachineBootstrapRequested = "machine.bootstrap_requested"

// RegisterMachineGRPCServices registers machine lifecycle gRPC services.
func RegisterMachineGRPCServices(deps MachineGRPCServicesDeps) ServiceRegistrar {
	return func(s *grpc.Server) error {
		if deps.Activation == nil || deps.MachineQueries == nil || deps.Pool == nil || deps.InventoryLedger == nil || deps.Commerce == nil || deps.TelemetryStore == nil {
			return fmt.Errorf("grpcserver: incomplete machine gRPC deps")
		}
		machinev1.RegisterMachineActivationServiceServer(s, &machineActivationServer{deps: deps})
		machinev1.RegisterMachineTokenServiceServer(s, &machineTokenServer{deps: deps})
		machinev1.RegisterMachineAuthServiceServer(s, &machineAuthServer{deps: deps})
		machinev1.RegisterMachineBootstrapServiceServer(s, &machineBootstrapServer{deps: deps})
		machinev1.RegisterMachineCatalogServiceServer(s, &machineCatalogServer{deps: deps})
		machinev1.RegisterMachineMediaServiceServer(s, &machineMediaServer{deps: deps})
		machinev1.RegisterMachineInventoryServiceServer(s, &machineInventoryServer{deps: deps})
		machinev1.RegisterMachineTelemetryServiceServer(s, &machineTelemetryServer{deps: deps})
		machinev1.RegisterMachineOperatorServiceServer(s, &machineOperatorServer{deps: deps})
		machinev1.RegisterMachineCommerceServiceServer(s, &machineCommerceServer{deps: deps})
		machinev1.RegisterMachineSaleServiceServer(s, &machineSaleServer{deps: deps})
		machinev1.RegisterMachineOfflineSyncServiceServer(s, &machineOfflineSyncServer{deps: deps})
		machinev1.RegisterMachineCommandServiceServer(s, &machineCommandServer{deps: deps})
		return nil
	}
}

type machineAuthServer struct {
	machinev1.UnimplementedMachineAuthServiceServer
	deps MachineGRPCServicesDeps
}

func (s *machineAuthServer) ActivateMachine(ctx context.Context, req *machinev1.ActivateMachineRequest) (*machinev1.ActivateMachineResponse, error) {
	var inner *machinev1.ClaimActivationRequest
	if req != nil {
		inner = req.GetClaim()
	}
	out, err := (&machineActivationServer{deps: s.deps}).ClaimActivation(ctx, inner)
	if err != nil {
		return nil, err
	}
	return &machinev1.ActivateMachineResponse{Claim: out}, nil
}

func (s *machineAuthServer) ClaimActivation(ctx context.Context, req *machinev1.MachineAuthServiceClaimActivationRequest) (*machinev1.MachineAuthServiceClaimActivationResponse, error) {
	var inner *machinev1.ClaimActivationRequest
	if req != nil {
		inner = req.GetClaim()
	}
	out, err := (&machineActivationServer{deps: s.deps}).ClaimActivation(ctx, inner)
	if err != nil {
		return nil, err
	}
	return &machinev1.MachineAuthServiceClaimActivationResponse{Claim: out}, nil
}

func (s *machineAuthServer) RefreshMachineToken(ctx context.Context, req *machinev1.MachineAuthServiceRefreshMachineTokenRequest) (*machinev1.MachineAuthServiceRefreshMachineTokenResponse, error) {
	var inner *machinev1.RefreshMachineTokenRequest
	if req != nil {
		inner = req.GetRefresh()
	}
	out, err := (&machineTokenServer{deps: s.deps}).RefreshMachineToken(ctx, inner)
	if err != nil {
		return nil, err
	}
	return &machinev1.MachineAuthServiceRefreshMachineTokenResponse{Refresh: out}, nil
}

type machineActivationServer struct {
	machinev1.UnimplementedMachineActivationServiceServer
	deps MachineGRPCServicesDeps
}

func grpcActivationClaimTransport(ctx context.Context) (ip, ua string) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if v := md.Get("grpc-user-agent"); len(v) > 0 {
			ua = strings.Join(v, ",")
		} else if v := md.Get("user-agent"); len(v) > 0 {
			ua = strings.Join(v, ",")
		}
		if v := md.Get("x-forwarded-for"); len(v) > 0 {
			ip = strings.TrimSpace(strings.Split(v[0], ",")[0])
		}
	}
	if ip == "" {
		if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
			addr := p.Addr.String()
			host, _, err := net.SplitHostPort(addr)
			if err == nil {
				ip = host
			} else {
				ip = strings.TrimSpace(addr)
			}
		}
	}
	return strings.TrimSpace(ip), strings.TrimSpace(ua)
}

func (s *machineActivationServer) ClaimActivation(ctx context.Context, req *machinev1.ClaimActivationRequest) (*machinev1.ClaimActivationResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	fp := req.GetDeviceFingerprint()
	cip, ua := grpcActivationClaimTransport(ctx)
	out, err := s.deps.Activation.Claim(ctx, activation.ClaimInput{
		ActivationCode: req.GetActivationCode(),
		DeviceFingerprint: activation.DeviceFingerprint{
			AndroidID:    fp.GetAndroidId(),
			SerialNumber: fp.GetSerialNumber(),
			Manufacturer: fp.GetManufacturer(),
			Model:        fp.GetModel(),
			PackageName:  fp.GetPackageName(),
			VersionName:  fp.GetVersionName(),
			VersionCode:  int(fp.GetVersionCode()),
		},
		ClientIP:  cip,
		UserAgent: ua,
	}, s.deps.MQTTBrokerURL, s.deps.MQTTTopicPrefix)
	if err != nil {
		return nil, mapActivationError(err)
	}
	resp := &machinev1.ClaimActivationResponse{
		MachineId:            out.MachineID.String(),
		OrganizationId:       out.OrganizationID.String(),
		SiteId:               out.SiteID.String(),
		MachineName:          out.MachineName,
		AccessToken:          out.MachineToken,
		AccessTokenExpiresAt: timestamppb.New(out.TokenExpiresAt),
		MqttBrokerUrl:        out.MQTTBrokerURL,
		MqttTopicPrefix:      out.MQTTTopicPrefix,
		BootstrapHttpPath:    out.BootstrapPath,
		BootstrapRequired:    out.BootstrapRequired,
	}
	if out.RefreshToken != "" {
		resp.RefreshToken = out.RefreshToken
		resp.RefreshTokenExpiresAt = timestamppb.New(out.RefreshExpiresAt)
	}
	return resp, nil
}

func mapActivationError(err error) error {
	switch {
	case err == nil:
		return nil
	case err == activation.ErrInvalid:
		return status.Error(codes.InvalidArgument, "activation_invalid")
	case err == activation.ErrMachineNotEligible:
		return status.Error(codes.PermissionDenied, "machine_not_eligible")
	default:
		return status.Error(codes.Internal, "internal")
	}
}

type machineTokenServer struct {
	machinev1.UnimplementedMachineTokenServiceServer
	deps MachineGRPCServicesDeps
}

func (s *machineTokenServer) RefreshMachineToken(ctx context.Context, req *machinev1.RefreshMachineTokenRequest) (*machinev1.RefreshMachineTokenResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	out, err := s.deps.Activation.RefreshMachineSession(ctx, activation.RefreshInput{
		RefreshToken: req.GetRefreshToken(),
	}, s.deps.MQTTBrokerURL, s.deps.MQTTTopicPrefix)
	if err != nil {
		switch err {
		case activation.ErrRefreshInvalid:
			return nil, status.Error(codes.Unauthenticated, "invalid_refresh_token")
		case activation.ErrMachineNotEligible:
			return nil, status.Error(codes.PermissionDenied, "machine_not_eligible")
		default:
			return nil, status.Error(codes.Internal, "internal")
		}
	}
	return &machinev1.RefreshMachineTokenResponse{
		MachineId:             out.MachineID.String(),
		OrganizationId:        out.OrganizationID.String(),
		SiteId:                out.SiteID.String(),
		MachineName:           out.MachineName,
		AccessToken:           out.MachineToken,
		AccessTokenExpiresAt:  timestamppb.New(out.TokenExpiresAt),
		RefreshToken:          out.RefreshToken,
		RefreshTokenExpiresAt: timestamppb.New(out.RefreshExpiresAt),
		MqttBrokerUrl:         out.MQTTBrokerURL,
		MqttTopicPrefix:       out.MQTTTopicPrefix,
		BootstrapHttpPath:     out.BootstrapPath,
	}, nil
}

type machineBootstrapServer struct {
	machinev1.UnimplementedMachineBootstrapServiceServer
	deps MachineGRPCServicesDeps
}

func (s *machineBootstrapServer) GetBootstrap(ctx context.Context, _ *machinev1.GetBootstrapRequest) (*machinev1.GetBootstrapResponse, error) {
	claims, ok := plauth.MachineAccessClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing machine credentials")
	}
	q := db.New(s.deps.Pool)
	if err := machineCredentialGate(ctx, q, claims); err != nil {
		return nil, err
	}
	b, err := s.deps.MachineQueries.GetMachineBootstrap(ctx, claims.MachineID)
	if err != nil {
		return nil, mapBootstrapError(err)
	}
	recordMachineBootstrapAudit(ctx, s.deps, claims)
	return mapBootstrapToProto(ctx, s.deps, claims.MachineID, b)
}

func (s *machineBootstrapServer) CheckForUpdates(ctx context.Context, req *machinev1.CheckForUpdatesRequest) (*machinev1.CheckForUpdatesResponse, error) {
	claims, ok := plauth.MachineAccessClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing machine credentials")
	}
	q := db.New(s.deps.Pool)
	if err := machineCredentialGate(ctx, q, claims); err != nil {
		return nil, err
	}
	b, err := s.deps.MachineQueries.GetMachineBootstrap(ctx, claims.MachineID)
	if err != nil {
		return nil, mapBootstrapError(err)
	}
	cat := setupapp.CatalogFingerprint(b)
	pr := setupapp.PricingFingerprint(b)
	pl := setupapp.PlanogramFingerprint(b)
	med := setupapp.MediaFingerprint(b)
	var ota bool
	if s.deps.FeatureFlags != nil {
		if rh, err := s.deps.FeatureFlags.RuntimeHintsForMachine(ctx, claims.MachineID); err == nil && rh != nil {
			ota = len(rh.PendingMachineConfigRollouts) > 0
		}
	}
	if req == nil {
		req = &machinev1.CheckForUpdatesRequest{}
	}
	return &machinev1.CheckForUpdatesResponse{
		CatalogChanged:               req.GetCatalogFingerprint() != cat,
		PricingChanged:               req.GetPricingFingerprint() != pr,
		PlanogramChanged:             req.GetPlanogramFingerprint() != pl,
		MediaChanged:                 req.GetMediaFingerprint() != med,
		FirmwareOrAppUpdateAvailable: ota,
		ServerCatalogFingerprint:     cat,
		ServerPricingFingerprint:     pr,
		ServerPlanogramFingerprint:   pl,
		ServerMediaFingerprint:       med,
	}, nil
}

func (s *machineBootstrapServer) CheckIn(ctx context.Context, req *machinev1.MachineBootstrapServiceCheckInRequest) (*machinev1.MachineBootstrapServiceCheckInResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	meta := req.GetMeta()
	if meta == nil {
		return nil, status.Error(codes.InvalidArgument, "meta required")
	}
	if strings.TrimSpace(meta.GetIdempotencyKey()) == "" {
		return nil, status.Error(codes.InvalidArgument, "idempotency_key required")
	}
	ce := strings.TrimSpace(meta.GetClientEventId())
	if ce == "" {
		bid := strings.TrimSpace(req.GetBootId())
		if bid == "" {
			return nil, status.Error(codes.InvalidArgument, "client_event_id or boot_id required")
		}
		ce = "bootstrap_boot:" + bid
	}
	ts := meta.GetOccurredAt()
	if ts == nil || !ts.IsValid() {
		return nil, status.Error(codes.InvalidArgument, "occurred_at required")
	}
	md := map[string]string{}
	for k, v := range req.GetAttributes() {
		md[k] = v
	}
	if bid := strings.TrimSpace(req.GetBootId()); bid != "" {
		md["boot_id"] = bid
	}
	if ns := strings.TrimSpace(req.GetNetworkState()); ns != "" {
		md["network_state"] = ns
	}
	if av := strings.TrimSpace(meta.GetAppVersion()); av != "" {
		md["app_version_meta"] = av
	}
	tel := &machineTelemetryServer{deps: s.deps}
	out, err := tel.CheckIn(ctx, &machinev1.CheckInRequest{
		Context: &machinev1.IdempotencyContext{
			IdempotencyKey:  strings.TrimSpace(meta.GetIdempotencyKey()),
			ClientEventId:   ce,
			ClientCreatedAt: ts,
		},
		BootId:       strings.TrimSpace(req.GetBootId()),
		NetworkState: strings.TrimSpace(req.GetNetworkState()),
		Metadata:     md,
	})
	if err != nil {
		return nil, err
	}
	rid := ""
	if meta != nil {
		rid = meta.GetRequestId()
	}
	st := machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED
	if out.GetReplay() {
		st = machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_REPLAYED
	}
	return &machinev1.MachineBootstrapServiceCheckInResponse{
		Meta: responseMetaCtx(ctx, rid, st),
	}, nil
}

func mapBootstrapError(err error) error {
	switch {
	case err == nil:
		return nil
	case err == setupapp.ErrNotFound:
		return status.Error(codes.NotFound, "machine_not_found")
	case err == setupapp.ErrMachineNotEligibleForBootstrap:
		return status.Error(codes.PermissionDenied, "machine_not_eligible")
	default:
		return status.Error(codes.Internal, "internal")
	}
}

func recordMachineBootstrapAudit(ctx context.Context, deps MachineGRPCServicesDeps, claims plauth.MachineAccessClaims) {
	if deps.EnterpriseAudit == nil {
		return
	}
	mid := claims.MachineID.String()
	_ = deps.EnterpriseAudit.Record(ctx, compliance.EnterpriseAuditRecord{
		OrganizationID: claims.OrganizationID,
		ActorType:      compliance.ActorMachine,
		ActorID:        &mid,
		Action:         actionMachineBootstrapRequested,
		ResourceType:   "machine",
		ResourceID:     &mid,
		Metadata:       []byte("{}"),
	})
}

func mapBootstrapToProto(ctx context.Context, deps MachineGRPCServicesDeps, machineID uuid.UUID, b setupapp.MachineBootstrap) (*machinev1.GetBootstrapResponse, error) {
	m := b.Machine
	var hw string
	if m.HardwareProfileID != nil && *m.HardwareProfileID != uuid.Nil {
		hw = m.HardwareProfileID.String()
	}
	byCab := make(map[string][]setupapp.CabinetSlotConfigView)
	for _, s := range b.CurrentCabinetSlots {
		byCab[s.CabinetCode] = append(byCab[s.CabinetCode], s)
	}
	cabinets := make([]*machinev1.BootstrapCabinet, 0, len(b.Cabinets))
	for _, c := range b.Cabinets {
		slots := byCab[c.Code]
		if slots == nil {
			slots = []setupapp.CabinetSlotConfigView{}
		}
		sk := make([]*machinev1.BootstrapSlot, 0, len(slots))
		for _, sl := range slots {
			bs := &machinev1.BootstrapSlot{
				ConfigId:             sl.ConfigID.String(),
				SlotCode:             sl.SlotCode,
				ProductSku:           sl.ProductSKU,
				ProductName:          sl.ProductName,
				MaxQuantity:          sl.MaxQuantity,
				PriceMinor:           sl.PriceMinor,
				EffectiveFromRfc3339: sl.EffectiveFrom.UTC().Format(time.RFC3339Nano),
				IsCurrent:            sl.IsCurrent,
				MachineSlotLayoutId:  sl.MachineSlotLayout.String(),
			}
			if sl.SlotIndex != nil {
				bs.SlotIndex = *sl.SlotIndex
			}
			if sl.ProductID != nil {
				bs.ProductId = sl.ProductID.String()
			}
			sk = append(sk, bs)
		}
		meta, _ := structpb.NewStruct(map[string]any{})
		if len(c.Metadata) > 0 {
			if s := structFromJSON(c.Metadata); s != nil {
				meta = s
			}
		}
		cabinets = append(cabinets, &machinev1.BootstrapCabinet{
			Id:        c.ID.String(),
			Code:      c.Code,
			Title:     c.Title,
			SortOrder: c.SortOrder,
			Metadata:  meta,
			Slots:     sk,
		})
	}
	products := make([]*machinev1.BootstrapCatalogProduct, 0, len(b.AssortmentProducts))
	for _, p := range b.AssortmentProducts {
		products = append(products, &machinev1.BootstrapCatalogProduct{
			ProductId:      p.ProductID.String(),
			Sku:            p.SKU,
			Name:           p.Name,
			SortOrder:      p.SortOrder,
			AssortmentId:   p.AssortmentID.String(),
			AssortmentName: p.AssortmentName,
		})
	}
	prefix := deps.MQTTTopicPrefix
	if prefix == "" {
		prefix = "avf/devices"
	}
	resp := &machinev1.GetBootstrapResponse{
		Machine: &machinev1.BootstrapMachine{
			MachineId:         m.ID.String(),
			OrganizationId:    m.OrganizationID.String(),
			SiteId:            m.SiteID.String(),
			HardwareProfileId: hw,
			SerialNumber:      m.SerialNumber,
			Name:              m.Name,
			Status:            m.Status,
			CommandSequence:   m.CommandSequence,
			CreatedAt:         timestamppb.New(m.CreatedAt.UTC()),
			UpdatedAt:         timestamppb.New(m.UpdatedAt.UTC()),
		},
		Topology:             &machinev1.BootstrapTopology{Cabinets: cabinets},
		Catalog:              &machinev1.BootstrapCatalog{Products: products},
		CatalogFingerprint:   setupapp.CatalogFingerprint(b),
		PricingFingerprint:   setupapp.PricingFingerprint(b),
		PlanogramFingerprint: setupapp.PlanogramFingerprint(b),
		MediaFingerprint:     setupapp.MediaFingerprint(b),
		ServerTime:           timestamppb.New(time.Now().UTC()),
		Mqtt: &machinev1.MqttConfigMetadata{
			BrokerUrl:   deps.MQTTBrokerURL,
			TopicPrefix: prefix,
		},
		PublishedPlanogramVersionNo: b.PublishedPlanogramVersionNo,
	}
	if b.PublishedPlanogramVersionID != nil && *b.PublishedPlanogramVersionID != uuid.Nil {
		resp.PublishedPlanogramVersionId = b.PublishedPlanogramVersionID.String()
	}
	if deps.FeatureFlags != nil {
		if rh, err := deps.FeatureFlags.RuntimeHintsForMachine(ctx, machineID); err == nil && rh != nil {
			resp.RuntimeHints = mapRuntimeHintsProto(rh)
		}
	}
	return resp, nil
}

func mapRuntimeHintsProto(h *featureflags.RuntimeHints) *machinev1.RuntimeHints {
	if h == nil {
		return nil
	}
	out := &machinev1.RuntimeHints{
		FeatureFlags:                 h.FeatureFlags,
		AppliedMachineConfigRevision: h.AppliedMachineConfigRevision,
	}
	for _, x := range h.PendingMachineConfigRollouts {
		out.PendingMachineConfigRollouts = append(out.PendingMachineConfigRollouts, &machinev1.PendingRolloutHint{
			RolloutId:          x.RolloutID,
			TargetVersionId:    x.TargetVersionID,
			TargetVersionLabel: x.TargetVersionLabel,
			Status:             x.Status,
		})
	}
	return out
}
