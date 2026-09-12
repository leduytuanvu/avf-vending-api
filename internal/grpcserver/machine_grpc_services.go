package grpcserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/activation"
	"github.com/avf/avf-vending-api/internal/app/featureflags"
	"github.com/avf/avf-vending-api/internal/app/layoutassignment"
	"github.com/avf/avf-vending-api/internal/app/sellreadiness"
	"github.com/avf/avf-vending-api/internal/app/setupapp"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	platformmqtt "github.com/avf/avf-vending-api/internal/platform/mqtt"
	platformpayments "github.com/avf/avf-vending-api/internal/platform/payments"
	"github.com/avf/avf-vending-api/internal/platform/pgxutil"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const actionMachineBootstrapRequested = "machine.bootstrap_requested"

func resolveMQTTTopicLayout(deps MachineGRPCServicesDeps) string {
	if deps.Config != nil {
		return platformmqtt.LayoutString(platformmqtt.NormalizeTopicLayout(deps.Config.MQTT.TopicLayout))
	}
	return string(platformmqtt.TopicLayoutLegacy)
}

func mapMqttConfigMetadataProto(deps MachineGRPCServicesDeps) *machinev1.MqttConfigMetadata {
	prefix := strings.TrimSuffix(strings.TrimSpace(deps.MQTTTopicPrefix), "/")
	layout := resolveMQTTTopicLayout(deps)
	tlsRequired := false
	if deps.Config != nil {
		tlsRequired = platformmqtt.BootstrapTLSRequired(deps.MQTTBrokerURL, deps.Config.MQTT.TLSEnabled, string(deps.Config.AppEnv))
	}
	return &machinev1.MqttConfigMetadata{
		BrokerUrl:      deps.MQTTBrokerURL,
		TopicPrefix:    prefix,
		TopicLayout:    layout,
		TlsRequired:    tlsRequired,
		ClientIdPolicy: platformmqtt.MachineClientIDPolicyTemplate,
	}
}

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
		machinev1.RegisterMachineRuntimeSessionServiceServer(s, &machineRuntimeSessionServer{deps: deps})
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
	started := time.Now()
	fp := req.GetDeviceFingerprint()
	cip, ua := grpcActivationClaimTransport(ctx)
	out, err := s.deps.Activation.Claim(ctx, activation.ClaimInput{
		ActivationCode:    req.GetActivationCode(),
		DeviceFingerprint: activation.DeviceFingerprintFromProto(fp),
		ClientIP:          cip,
		UserAgent:         ua,
	}, s.deps.MQTTBrokerURL, s.deps.MQTTTopicPrefix, resolveMQTTTopicLayout(s.deps))
	if err != nil {
		logUnmappedClaimActivationError(ctx, err, started)
		return nil, mapActivationError(err)
	}
	resp := &machinev1.ClaimActivationResponse{
		MachineId:            out.MachineID.String(),
		MachineCode:          strings.TrimSpace(out.MachineCode),
		SiteId:               out.SiteID.String(),
		MachineName:          out.MachineName,
		AccessToken:          out.MachineToken,
		AccessTokenExpiresAt: timestamppb.New(out.TokenExpiresAt),
		MqttBrokerUrl:        out.MQTTBrokerURL,
		MqttTopicPrefix:      out.MQTTTopicPrefix,
		MqttTopicLayout:      out.MQTTTopicLayout,
		MqttUsername:         out.MQTTUsername,
		MqttPassword:         out.MQTTPassword,
		BootstrapHttpPath:    out.BootstrapPath,
		BootstrapRequired:    out.BootstrapRequired,
	}
	if out.RefreshToken != "" {
		resp.RefreshToken = out.RefreshToken
		resp.RefreshTokenExpiresAt = timestamppb.New(out.RefreshExpiresAt)
	}
	if out.DeviceAttachmentID != nil && *out.DeviceAttachmentID != uuid.Nil {
		resp.DeviceAttachmentId = out.DeviceAttachmentID.String()
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
	case errors.Is(err, activation.ErrMQTTProvisioning):
		slog.Warn("activation_claim_mqtt_provisioning_failed", "err", err)
		return status.Error(codes.Unavailable, "mqtt_provisioning_failed")
	default:
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr != nil && pgErr.Code == "22P02" {
			return status.Error(codes.FailedPrecondition, "activation_storage_json_invalid")
		}
		return status.Error(codes.Internal, "internal")
	}
}

// Standalone 6-digit tokens matching activation.ActivationCodeLength / ^[0-9]{6}$.
// Word boundaries avoid redacting SQLSTATE (5 digits), AVF000001, UUIDs, and duration_ms.
var claimActivationCodePlaintextRE = regexp.MustCompile(`\b[0-9]{6}\b`)

func claimActivationLogger() *slog.Logger {
	if claimActivationTestLogger != nil {
		return claimActivationTestLogger
	}
	return slog.Default()
}

// claimActivationTestLogger is test-only; production always uses slog.Default().
var claimActivationTestLogger *slog.Logger

func logUnmappedClaimActivationError(ctx context.Context, err error, started time.Time) {
	if err == nil || err == activation.ErrInvalid || err == activation.ErrMachineNotEligible || errors.Is(err, activation.ErrMQTTProvisioning) {
		return
	}
	meta, _ := GRPCRequestMetaFromContext(ctx)
	md, _ := metadata.FromIncomingContext(ctx)
	attrs := []any{
		"grpc_method", machinev1.MachineActivationService_ClaimActivation_FullMethodName,
		"request_id", meta.RequestID,
		"correlation_id", meta.CorrelationID,
		"trace_id", grpcAccessTraceID(md, meta),
		"error_type", claimActivationInnermostErrorType(err),
		"error", claimActivationLogErrorText(err),
	}
	if !started.IsZero() {
		attrs = append(attrs, "duration_ms", time.Since(started).Milliseconds())
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr != nil {
		attrs = append(attrs,
			"sqlstate", pgErr.Code,
			"pg_table", pgErr.TableName,
			"pg_constraint", pgErr.ConstraintName,
		)
	}
	claimActivationLogger().Error("machine activation claim failed", attrs...)
}

func claimActivationInnermostErrorType(err error) string {
	for err != nil {
		next := errors.Unwrap(err)
		if next == nil {
			return fmt.Sprintf("%T", err)
		}
		err = next
	}
	return ""
}

func claimActivationLogErrorText(err error) string {
	if err == nil {
		return ""
	}
	text := err.Error()
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr != nil {
		for _, extra := range []string{pgErr.Detail, pgErr.Hint, pgErr.Where, pgErr.InternalQuery} {
			if s := strings.TrimSpace(extra); s != "" {
				text = strings.ReplaceAll(text, s, "")
			}
		}
	}
	text = sanitizeGRPCErrorMessage(text)
	return claimActivationCodePlaintextRE.ReplaceAllString(text, "[REDACTED_ACTIVATION_CODE]")
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
	}, s.deps.MQTTBrokerURL, s.deps.MQTTTopicPrefix, resolveMQTTTopicLayout(s.deps))
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
		MachineCode:           strings.TrimSpace(out.MachineCode),
		SiteId:                out.SiteID.String(),
		MachineName:           out.MachineName,
		AccessToken:           out.MachineToken,
		AccessTokenExpiresAt:  timestamppb.New(out.TokenExpiresAt),
		RefreshToken:          out.RefreshToken,
		RefreshTokenExpiresAt: timestamppb.New(out.RefreshExpiresAt),
		MqttBrokerUrl:         out.MQTTBrokerURL,
		MqttTopicPrefix:       out.MQTTTopicPrefix,
		MqttTopicLayout:       out.MQTTTopicLayout,
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
	layoutBundle := loadBootstrapLayoutBundle(ctx, s.deps, claims.MachineID)
	desiredFP := desiredLayoutFingerprint(layoutBundle)
	return &machinev1.CheckForUpdatesResponse{
		CatalogChanged:               req.GetCatalogFingerprint() != cat,
		PricingChanged:               req.GetPricingFingerprint() != pr,
		PlanogramChanged:             req.GetPlanogramFingerprint() != pl,
		MediaChanged:                 req.GetMediaFingerprint() != med,
		LayoutChanged:                layoutChangedSinceClient(desiredFP, ""),
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

func (s *machineBootstrapServer) ReportLocalLayout(ctx context.Context, req *machinev1.ReportLocalLayoutRequest) (*machinev1.ReportLocalLayoutResponse, error) {
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
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	localLayoutID, err := uuid.Parse(strings.TrimSpace(req.GetLocalLayoutId()))
	if err != nil || localLayoutID == uuid.Nil {
		return nil, status.Error(codes.InvalidArgument, "invalid_local_layout_id")
	}
	slotsJSON, err := marshalReportLocalLayoutSlots(req.GetSlots())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid_slots")
	}
	svc := &layoutassignment.Service{Pool: s.deps.Pool}
	out, rerr := svc.ReportLocalLayout(ctx, layoutassignment.MachineAuthContext{MachineID: claims.MachineID}, layoutassignment.ReportLocalLayoutInput{
		MachineID:        claims.MachineID,
		LocalLayoutID:    localLayoutID,
		Revision:         req.GetRevision(),
		Rows:             req.GetRows(),
		Columns:          req.GetColumns(),
		SlotsJSON:        slotsJSON,
		Fingerprint:      req.GetFingerprint(),
		DeviceInstanceID: req.GetDeviceInstanceId(),
		IdempotencyKey:   req.GetMeta().GetIdempotencyKey(),
		LocalGeneration:  req.GetLocalGeneration(),
	})
	if rerr != nil {
		return nil, mapReportLocalLayoutError(rerr)
	}
	rid := ""
	if req.GetMeta() != nil {
		rid = req.GetMeta().GetRequestId()
	}
	return &machinev1.ReportLocalLayoutResponse{
		Meta:              responseMetaCtx(ctx, rid, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED),
		Accepted:          out.Accepted,
		StoredRevision:    out.StoredRevision,
		StoredGeneration:  out.StoredGeneration,
		StoredFingerprint: out.StoredFingerprint,
	}, nil
}

func marshalReportLocalLayoutSlots(slots []*machinev1.ReportLocalLayoutSlot) ([]byte, error) {
	if len(slots) == 0 {
		return nil, fmt.Errorf("slots required")
	}
	type slotJSON struct {
		SlotCode             string `json:"slotCode"`
		SlotOrdinal          int32  `json:"slotOrdinal,omitempty"`
		LogicalCoordinate    string `json:"logicalCoordinate,omitempty"`
		PhysicalLane         int32  `json:"physicalLane,omitempty"`
		ProductID            string `json:"productId,omitempty"`
		MaxQuantity          int32  `json:"maxQuantity,omitempty"`
		PriceMinor           int64  `json:"priceMinor,omitempty"`
		LocalPricingRevision int64  `json:"localPricingRevision,omitempty"`
	}
	out := make([]slotJSON, 0, len(slots))
	for _, sl := range slots {
		if sl == nil {
			return nil, fmt.Errorf("slot entry required")
		}
		out = append(out, slotJSON{
			SlotCode:             strings.TrimSpace(sl.GetSlotCode()),
			SlotOrdinal:          sl.GetSlotOrdinal(),
			LogicalCoordinate:    strings.TrimSpace(sl.GetLogicalCoordinate()),
			PhysicalLane:         sl.GetPhysicalLane(),
			ProductID:            strings.TrimSpace(sl.GetProductId()),
			MaxQuantity:          sl.GetMaxQuantity(),
			PriceMinor:           sl.GetPriceMinor(),
			LocalPricingRevision: sl.GetLocalPricingRevision(),
		})
	}
	return json.Marshal(out)
}

func (s *machineBootstrapServer) GetMachineLayoutLibrary(ctx context.Context, req *machinev1.GetMachineLayoutLibraryRequest) (*machinev1.GetMachineLayoutLibraryResponse, error) {
	claims, ok := plauth.MachineAccessClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing machine credentials")
	}
	svc := &layoutassignment.Service{Pool: s.deps.Pool}
	lib, err := svc.GetMachineLayoutLibrary(ctx, claims.MachineID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "layout_library_not_found")
	}
	summaries := make([]*machinev1.MachineLayoutSummary, 0, len(lib.Layouts))
	for _, row := range lib.Layouts {
		summaries = append(summaries, &machinev1.MachineLayoutSummary{
			LayoutId:    row.LayoutID.String(),
			Name:        row.Name,
			Status:      row.Status,
			GridRows:    row.GridRows,
			GridCols:    row.GridCols,
			LayoutRevision: row.Revision,
			Fingerprint: row.Fingerprint,
		})
	}
	rid := ""
	if req != nil && req.GetMeta() != nil {
		rid = req.GetMeta().GetRequestId()
	}
	resp := &machinev1.GetMachineLayoutLibraryResponse{
		Meta:     responseMetaCtx(ctx, rid, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED),
		Layouts:  summaries,
	}
	if lib.ActiveLayoutID != nil {
		resp.ActiveLayoutId = lib.ActiveLayoutID.String()
	}
	if lib.DesiredActiveLayoutID != nil {
		resp.DesiredActiveLayoutId = lib.DesiredActiveLayoutID.String()
	}
	if lib.ReportedActiveLayoutID != nil {
		resp.ReportedActiveLayoutId = lib.ReportedActiveLayoutID.String()
	}
	return resp, nil
}

func (s *machineBootstrapServer) ReportLayoutSnapshot(ctx context.Context, req *machinev1.ReportLayoutSnapshotRequest) (*machinev1.ReportLayoutSnapshotResponse, error) {
	claims, ok := plauth.MachineAccessClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing machine credentials")
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	in, err := mapReportLayoutSnapshotRequest(claims.MachineID, req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	svc := &layoutassignment.Service{Pool: s.deps.Pool}
	out, rerr := svc.ReportLayoutSnapshot(ctx, layoutassignment.MachineAuthContext{MachineID: claims.MachineID}, in)
	if rerr != nil {
		return nil, mapReportLayoutSnapshotError(rerr)
	}
	rid := ""
	if req.GetMeta() != nil {
		rid = req.GetMeta().GetRequestId()
	}
	return &machinev1.ReportLayoutSnapshotResponse{
		Meta:                  responseMetaCtx(ctx, rid, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED),
		Accepted:              out.Accepted,
		Duplicate:             out.Duplicate,
		StoredSnapshotId:      out.StoredSnapshotID.String(),
		StoredCaptureSequence: out.StoredCaptureSequence,
		StoredFingerprint:     out.StoredFingerprint,
	}, nil
}

func (s *machineBootstrapServer) ReportLayoutSnapshotBatch(ctx context.Context, req *machinev1.ReportLayoutSnapshotBatchRequest) (*machinev1.ReportLayoutSnapshotBatchResponse, error) {
	if req == nil || len(req.GetSnapshots()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "snapshots required")
	}
	results := make([]*machinev1.ReportLayoutSnapshotResponse, 0, len(req.GetSnapshots()))
	for _, snap := range req.GetSnapshots() {
		out, err := s.ReportLayoutSnapshot(ctx, snap)
		if err != nil {
			return nil, err
		}
		results = append(results, out)
	}
	rid := ""
	if req.GetMeta() != nil {
		rid = req.GetMeta().GetRequestId()
	}
	return &machinev1.ReportLayoutSnapshotBatchResponse{
		Meta:    responseMetaCtx(ctx, rid, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED),
		Results: results,
	}, nil
}

func (s *machineBootstrapServer) AckLayoutActivation(ctx context.Context, req *machinev1.AckLayoutActivationRequest) (*machinev1.AckLayoutActivationResponse, error) {
	claims, ok := plauth.MachineAccessClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing machine credentials")
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	layoutID, err := uuid.Parse(strings.TrimSpace(req.GetLayoutId()))
	if err != nil || layoutID == uuid.Nil {
		return nil, status.Error(codes.InvalidArgument, "invalid_layout_id")
	}
	svc := &layoutassignment.Service{Pool: s.deps.Pool}
	out, rerr := svc.AckLayoutActivation(ctx, layoutassignment.MachineAuthContext{MachineID: claims.MachineID}, layoutassignment.AckLayoutActivationInput{
		MachineID:        claims.MachineID,
		LayoutID:         layoutID,
		CaptureSequence:  req.GetCaptureSequence(),
		Fingerprint:      req.GetFingerprint(),
		DeviceInstanceID: req.GetDeviceInstanceId(),
	})
	if rerr != nil {
		return nil, mapReportLayoutSnapshotError(rerr)
	}
	rid := ""
	if req.GetMeta() != nil {
		rid = req.GetMeta().GetRequestId()
	}
	return &machinev1.AckLayoutActivationResponse{
		Meta:                     responseMetaCtx(ctx, rid, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED),
		Accepted:                 out.Accepted,
		ReportedActiveLayoutId:   out.ReportedActiveLayoutID.String(),
	}, nil
}

func mapReportLayoutSnapshotRequest(machineID uuid.UUID, req *machinev1.ReportLayoutSnapshotRequest) (layoutassignment.ReportLayoutSnapshotInput, error) {
	snapshotID, err := uuid.Parse(strings.TrimSpace(req.GetSnapshotId()))
	if err != nil || snapshotID == uuid.Nil {
		return layoutassignment.ReportLayoutSnapshotInput{}, fmt.Errorf("invalid_snapshot_id")
	}
	layoutID, err := uuid.Parse(strings.TrimSpace(req.GetLayoutId()))
	if err != nil || layoutID == uuid.Nil {
		return layoutassignment.ReportLayoutSnapshotInput{}, fmt.Errorf("invalid_layout_id")
	}
	activeLayoutID := layoutID
	if strings.TrimSpace(req.GetActiveLayoutId()) != "" {
		activeLayoutID, err = uuid.Parse(strings.TrimSpace(req.GetActiveLayoutId()))
		if err != nil || activeLayoutID == uuid.Nil {
			return layoutassignment.ReportLayoutSnapshotInput{}, fmt.Errorf("invalid_active_layout_id")
		}
	}
	slotsJSON, err := marshalLayoutSnapshotSlots(req.GetSlots())
	if err != nil {
		return layoutassignment.ReportLayoutSnapshotInput{}, err
	}
	capturedAt := time.Now().UTC()
	if req.GetCapturedAt() != nil {
		capturedAt = req.GetCapturedAt().AsTime().UTC()
	}
	var baseRev *int32
	if req.GetBaseServerRevision() > 0 {
		v := req.GetBaseServerRevision()
		baseRev = &v
	}
	return layoutassignment.ReportLayoutSnapshotInput{
		MachineID:          machineID,
		SnapshotID:         snapshotID,
		LayoutID:           layoutID,
		LayoutName:         req.GetLayoutName(),
		DeviceInstanceID:   req.GetDeviceInstanceId(),
		CaptureSequence:    req.GetCaptureSequence(),
		DeviceGeneration:   req.GetDeviceGeneration(),
		CapturedAt:         capturedAt,
		IntervalKey:        req.GetIntervalKey(),
		BaseServerRevision: baseRev,
		Fingerprint:        req.GetFingerprint(),
		PayloadVersion:     req.GetPayloadVersion(),
		SnapshotReason:     mapSnapshotReasonProto(req.GetSnapshotReason()),
		GridRows:           req.GetGridRows(),
		GridCols:           req.GetGridCols(),
		ActiveLayoutID:     activeLayoutID,
		SlotsJSON:          slotsJSON,
	}, nil
}

func mapSnapshotReasonProto(reason machinev1.LayoutSnapshotReason) string {
	switch reason {
	case machinev1.LayoutSnapshotReason_LAYOUT_SNAPSHOT_REASON_PERIODIC_30M:
		return layoutassignment.SnapshotReasonPeriodic30M
	case machinev1.LayoutSnapshotReason_LAYOUT_SNAPSHOT_REASON_MANUAL_SYNC:
		return layoutassignment.SnapshotReasonManualSync
	case machinev1.LayoutSnapshotReason_LAYOUT_SNAPSHOT_REASON_RECONNECT:
		return layoutassignment.SnapshotReasonReconnect
	case machinev1.LayoutSnapshotReason_LAYOUT_SNAPSHOT_REASON_ACTIVATION:
		return layoutassignment.SnapshotReasonActivation
	default:
		return layoutassignment.SnapshotReasonLegacyReport
	}
}

func marshalLayoutSnapshotSlots(slots []*machinev1.LayoutSnapshotSlot) ([]byte, error) {
	if len(slots) == 0 {
		return nil, fmt.Errorf("slots required")
	}
	type slotJSON struct {
		SlotCode             string `json:"slotCode"`
		SlotOrdinal          int32  `json:"slotOrdinal,omitempty"`
		LogicalCoordinate    string `json:"logicalCoordinate,omitempty"`
		PhysicalLane         int32  `json:"physicalLane,omitempty"`
		ProductID            string `json:"productId,omitempty"`
		MaxQuantity          int32  `json:"maxQuantity,omitempty"`
		PriceMinor           int64  `json:"priceMinor,omitempty"`
		LocalPricingRevision int64  `json:"localPricingRevision,omitempty"`
		CurrentInventory     int32  `json:"currentInventory,omitempty"`
		Enabled              bool   `json:"enabled,omitempty"`
		OperationalState     string `json:"operationalState,omitempty"`
	}
	out := make([]slotJSON, 0, len(slots))
	for _, sl := range slots {
		if sl == nil {
			return nil, fmt.Errorf("slot entry required")
		}
		out = append(out, slotJSON{
			SlotCode:             strings.TrimSpace(sl.GetSlotCode()),
			SlotOrdinal:          sl.GetSlotOrdinal(),
			LogicalCoordinate:    strings.TrimSpace(sl.GetLogicalCoordinate()),
			PhysicalLane:         sl.GetPhysicalLane(),
			ProductID:            strings.TrimSpace(sl.GetProductId()),
			MaxQuantity:          sl.GetMaxQuantity(),
			PriceMinor:           sl.GetPriceMinor(),
			LocalPricingRevision: sl.GetLocalPricingRevision(),
			CurrentInventory:     sl.GetCurrentInventory(),
			Enabled:              sl.GetEnabled(),
			OperationalState:     strings.TrimSpace(sl.GetOperationalState()),
		})
	}
	return json.Marshal(out)
}

func mapReportLayoutSnapshotError(err error) error {
	switch {
	case errors.Is(err, layoutassignment.ErrLayoutNotFound):
		return status.Error(codes.NotFound, "layout_not_found")
	case errors.Is(err, layoutassignment.ErrSnapshotSchemaUnsupported):
		return status.Error(codes.InvalidArgument, "snapshot_schema_unsupported")
	case errors.Is(err, layoutassignment.ErrStaleLayoutRevision):
		return status.Error(codes.FailedPrecondition, "stale_layout_revision")
	case errors.Is(err, layoutassignment.ErrInvalidDimensions):
		return status.Error(codes.InvalidArgument, "invalid_dimensions")
	default:
		return status.Error(codes.InvalidArgument, err.Error())
	}
}

func mapReportLocalLayoutError(err error) error {
	switch {
	case errors.Is(err, layoutassignment.ErrLayoutRevisionConflict):
		return status.Error(codes.FailedPrecondition, "layout_revision_conflict")
	case errors.Is(err, layoutassignment.ErrInvalidDimensions):
		return status.Error(codes.InvalidArgument, "invalid_dimensions")
	default:
		return status.Error(codes.InvalidArgument, err.Error())
	}
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
		ActorType:    compliance.ActorMachine,
		ActorID:      &mid,
		Action:       actionMachineBootstrapRequested,
		ResourceType: "machine",
		ResourceID:   &mid,
		Metadata:     []byte("{}"),
	})
}

func mapBootstrapToProto(ctx context.Context, deps MachineGRPCServicesDeps, machineID uuid.UUID, b setupapp.MachineBootstrap) (*machinev1.GetBootstrapResponse, error) {
	m := b.Machine
	layoutBundle := loadBootstrapLayoutBundle(ctx, deps, machineID)
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
			GridRows:  layoutBundle.serverGridRows(),
			GridCols:  layoutBundle.serverGridCols(),
		})
	}
	products := make([]*machinev1.BootstrapCatalogProduct, 0, len(b.AssortmentProducts))
	var assortReady map[uuid.UUID]bool
	if deps.Pool != nil && len(b.AssortmentProducts) > 0 {
		apIDs := make([]uuid.UUID, 0, len(b.AssortmentProducts))
		for _, p := range b.AssortmentProducts {
			apIDs = append(apIDs, p.ProductID)
		}
		var err error
		assortReady, err = sellreadiness.PrimaryMediaReadyMap(ctx, pgxutil.NewQueries(deps.Pool), apIDs)
		if err != nil {
			return nil, err
		}
	}
	for _, p := range b.AssortmentProducts {
		pmReady := false
		if assortReady != nil {
			pmReady = assortReady[p.ProductID]
		}
		products = append(products, &machinev1.BootstrapCatalogProduct{
			ProductId:         p.ProductID.String(),
			Sku:               p.SKU,
			Name:              p.Name,
			SortOrder:         p.SortOrder,
			AssortmentId:      p.AssortmentID.String(),
			AssortmentName:    p.AssortmentName,
			PrimaryMediaReady: pmReady,
		})
	}
	prefix := deps.MQTTTopicPrefix
	if prefix == "" {
		prefix = "avf/devices"
	}
	resp := &machinev1.GetBootstrapResponse{
		Machine: &machinev1.BootstrapMachine{
			MachineId:         m.ID.String(),
			MachineCode:       strings.TrimSpace(m.Code),
			SiteId:            m.SiteID.String(),
			HardwareProfileId: hw,
			SerialNumber:      m.SerialNumber,
			Name:              m.Name,
			Status:            m.Status,
			CommandSequence:   m.CommandSequence,
			CreatedAt:         timestamppb.New(m.CreatedAt.UTC()),
			UpdatedAt:         timestamppb.New(m.UpdatedAt.UTC()),
		},
		Topology:                    &machinev1.BootstrapTopology{Cabinets: cabinets},
		Catalog:                     &machinev1.BootstrapCatalog{Products: products},
		CatalogFingerprint:          setupapp.CatalogFingerprint(b),
		PricingFingerprint:          setupapp.PricingFingerprint(b),
		PlanogramFingerprint:        setupapp.PlanogramFingerprint(b),
		MediaFingerprint:            setupapp.MediaFingerprint(b),
		ServerTime:                  timestamppb.New(time.Now().UTC()),
		Mqtt:                        mapMqttConfigMetadataProto(deps),
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
	if deps.Pool != nil && deps.MachineQueries != nil {
		sv, err := deps.MachineQueries.GetMachineSlotView(ctx, machineID)
		if err != nil {
			return nil, err
		}
		sr, err := sellreadiness.Compute(ctx, pgxutil.NewQueries(deps.Pool), sv, b)
		if err != nil {
			return nil, err
		}
		if resp.RuntimeHints == nil {
			resp.RuntimeHints = &machinev1.RuntimeHints{}
		}
		resp.RuntimeHints.SellReadiness = &machinev1.SellReadiness{
			CatalogSynced:   sr.CatalogSynced,
			MediaSynced:     sr.MediaSynced,
			InventorySynced: sr.InventorySynced,
			ReadyForSale:    sr.ReadyForSale,
			ReadinessIssues: sr.Issues,
		}
	}
	flags := map[string]bool{}
	if resp.RuntimeHints != nil && resp.RuntimeHints.FeatureFlags != nil {
		flags = resp.RuntimeHints.FeatureFlags
	}
	paymentMethods := resolveMachinePaymentMethods(ctx, deps, machineID, flags)
	providerDetails := make([]string, 0, len(paymentMethods.Providers))
	for _, p := range paymentMethods.Providers {
		providerDetails = append(providerDetails, fmt.Sprintf(
			"%s:enabled=%t:ready=%t:session_creatable=%t:reason=%s",
			p.Key, p.Enabled, p.Ready, p.SessionCreatable, p.UnavailableReason,
		))
	}
	slog.Info("PAYMENT_CAPABILITY_RESOLVED",
		"machine_id", machineID.String(),
		"cash_enabled", paymentMethods.CashEnabled,
		"qr_enabled", paymentMethods.QRCardEnabled,
		"payment_mode", paymentMethods.PaymentMode,
		"providers", strings.Join(platformpayments.EnabledSessionCreatableProviders(paymentMethods), ","),
		"provider_details", strings.Join(providerDetails, ";"),
	)
	resp.PaymentMethods = mapPaymentMethodsProto(paymentMethods)
	applyBootstrapLayoutFields(resp, layoutBundle)
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
