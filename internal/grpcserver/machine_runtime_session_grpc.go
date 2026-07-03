package grpcserver

import (
	"context"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/machineruntime"
	"github.com/avf/avf-vending-api/internal/gen/db"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type machineRuntimeSessionServer struct {
	machinev1.UnimplementedMachineRuntimeSessionServiceServer
	deps MachineGRPCServicesDeps
}

func (s *machineRuntimeSessionServer) requireRuntime(ctx context.Context) (plauth.MachineAccessClaims, *machineruntime.Service, error) {
	if s.deps.MachineRuntime == nil {
		return plauth.MachineAccessClaims{}, nil, status.Error(codes.Unavailable, "runtime session service not configured")
	}
	claims, ok := plauth.MachineAccessClaimsFromContext(ctx)
	if !ok {
		return plauth.MachineAccessClaims{}, nil, status.Error(codes.Unauthenticated, "missing machine credentials")
	}
	q := db.New(s.deps.Pool)
	if err := machineCredentialGate(ctx, q, claims); err != nil {
		return plauth.MachineAccessClaims{}, nil, err
	}
	return claims, s.deps.MachineRuntime, nil
}

func (s *machineRuntimeSessionServer) StartRuntimeSession(ctx context.Context, req *machinev1.StartRuntimeSessionRequest) (*machinev1.StartRuntimeSessionResponse, error) {
	if req == nil || req.GetIdentity() == nil {
		return nil, status.Error(codes.InvalidArgument, "identity required")
	}
	claims, rt, err := s.requireRuntime(ctx)
	if err != nil {
		return nil, err
	}
	if err := validateRuntimeMetaMachineID(req.GetMeta(), claims.MachineID); err != nil {
		return nil, err
	}
	id := req.GetIdentity()
	var attachID, sessID, opID *uuid.UUID
	if v := strings.TrimSpace(id.GetDeviceAttachmentId()); v != "" {
		u, err := uuid.Parse(v)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid device_attachment_id")
		}
		attachID = &u
	}
	if v := strings.TrimSpace(id.GetMachineSessionId()); v != "" {
		u, err := uuid.Parse(v)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid machine_session_id")
		}
		sessID = &u
	} else if claims.SessionID != uuid.Nil {
		sid := claims.SessionID
		sessID = &sid
	}
	if v := strings.TrimSpace(id.GetOperatorSessionId()); v != "" {
		u, err := uuid.Parse(v)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid operator_session_id")
		}
		opID = &u
	}
	row, err := rt.StartRuntimeAppSession(ctx, machineruntime.StartInput{
		MachineID:          claims.MachineID,
		DeviceAttachmentID: attachID,
		MachineSessionID:   sessID,
		OperatorSessionID:  opID,
		BootID:             strings.TrimSpace(id.GetBootId()),
		AppStartID:         strings.TrimSpace(id.GetAppStartId()),
		AppInstanceID:      strings.TrimSpace(id.GetAppInstanceId()),
		PackageName:        strings.TrimSpace(id.GetPackageName()),
		AppVersion:         strings.TrimSpace(id.GetAppVersion()),
		AppBuildSHA:        strings.TrimSpace(id.GetAppBuildSha()),
		StartReason:        strings.TrimSpace(req.GetStartReason()),
		NetworkState:       strings.TrimSpace(req.GetNetworkState()),
		MqttState:          strings.TrimSpace(req.GetMqttState()),
		StorefrontState:    strings.TrimSpace(req.GetStorefrontState()),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "start runtime session: %v", err)
	}
	online, _ := rt.ComputeMachineOnlineStatus(ctx, claims.MachineID)
	return &machinev1.StartRuntimeSessionResponse{
		Meta:    machineResponseMetaOK(req.GetMeta()),
		Session: mapRuntimeSessionProto(row, online),
	}, nil
}

func (s *machineRuntimeSessionServer) HeartbeatRuntimeSession(ctx context.Context, req *machinev1.HeartbeatRuntimeSessionRequest) (*machinev1.HeartbeatRuntimeSessionResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	claims, rt, err := s.requireRuntime(ctx)
	if err != nil {
		return nil, err
	}
	if err := validateRuntimeMetaMachineID(req.GetMeta(), claims.MachineID); err != nil {
		return nil, err
	}
	sid, err := uuid.Parse(strings.TrimSpace(req.GetSessionId()))
	if err != nil || sid == uuid.Nil {
		return nil, status.Error(codes.InvalidArgument, "invalid session_id")
	}
	row, err := rt.HeartbeatRuntimeAppSession(ctx, machineruntime.HeartbeatInput{
		SessionID:       sid,
		MachineID:       claims.MachineID,
		NetworkState:    strings.TrimSpace(req.GetNetworkState()),
		MqttState:       strings.TrimSpace(req.GetMqttState()),
		StorefrontState: strings.TrimSpace(req.GetStorefrontState()),
		SellReady:       req.GetSellReady(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "heartbeat runtime session: %v", err)
	}
	online, _ := rt.ComputeMachineOnlineStatus(ctx, claims.MachineID)
	return &machinev1.HeartbeatRuntimeSessionResponse{
		Meta:    machineResponseMetaOK(req.GetMeta()),
		Session: mapRuntimeSessionProto(row, online),
	}, nil
}

func (s *machineRuntimeSessionServer) EndRuntimeSession(ctx context.Context, req *machinev1.EndRuntimeSessionRequest) (*machinev1.EndRuntimeSessionResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	claims, rt, err := s.requireRuntime(ctx)
	if err != nil {
		return nil, err
	}
	if err := validateRuntimeMetaMachineID(req.GetMeta(), claims.MachineID); err != nil {
		return nil, err
	}
	sid, err := uuid.Parse(strings.TrimSpace(req.GetSessionId()))
	if err != nil || sid == uuid.Nil {
		return nil, status.Error(codes.InvalidArgument, "invalid session_id")
	}
	row, err := rt.EndRuntimeAppSession(ctx, machineruntime.EndInput{
		SessionID: sid,
		MachineID: claims.MachineID,
		EndReason: strings.TrimSpace(req.GetEndReason()),
		Status:    strings.TrimSpace(req.GetStatus()),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "end runtime session: %v", err)
	}
	return &machinev1.EndRuntimeSessionResponse{
		Meta:    machineResponseMetaOK(req.GetMeta()),
		Session: mapRuntimeSessionProto(row, "offline"),
	}, nil
}

func (s *machineRuntimeSessionServer) GetRuntimeSessionState(ctx context.Context, req *machinev1.GetRuntimeSessionStateRequest) (*machinev1.GetRuntimeSessionStateResponse, error) {
	claims, rt, err := s.requireRuntime(ctx)
	if err != nil {
		return nil, err
	}
	if req != nil {
		if err := validateRuntimeMetaMachineID(req.GetMeta(), claims.MachineID); err != nil {
			return nil, err
		}
	}
	row, err := rt.GetCurrentRuntimeAppSession(ctx, claims.MachineID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "no active runtime session")
	}
	online, _ := rt.ComputeMachineOnlineStatus(ctx, claims.MachineID)
	return &machinev1.GetRuntimeSessionStateResponse{
		Meta:    machineResponseMetaOK(nil),
		Session: mapRuntimeSessionProto(row, online),
	}, nil
}

func validateRuntimeMetaMachineID(meta *machinev1.MachineRequestMeta, machineID uuid.UUID) error {
	if meta == nil {
		return nil
	}
	raw := strings.TrimSpace(meta.GetMachineId())
	if raw == "" {
		return nil
	}
	mid, err := uuid.Parse(raw)
	if err != nil || mid != machineID {
		return status.Error(codes.PermissionDenied, "machine_id does not match token")
	}
	return nil
}

func mapRuntimeSessionProto(row db.MachineRuntimeAppSession, online string) *machinev1.RuntimeSessionStatus {
	out := &machinev1.RuntimeSessionStatus{
		SessionId:        row.ID.String(),
		MachineId:        row.MachineID.String(),
		Status:           row.Status,
		StartReason:      row.StartReason,
		LastNetworkState: row.LastNetworkState,
		LastMqttState:    row.LastMqttState,
		StorefrontState:  row.StorefrontState,
		SellReady:        row.SellReady,
		OnlineStatus:     online,
		StartedAt:        timestamppb.New(row.StartedAt.UTC()),
	}
	if row.EndReason.Valid {
		er := row.EndReason.String
		out.EndReason = &er
	}
	if row.LastHeartbeatAt.Valid {
		out.LastHeartbeatAt = timestamppb.New(row.LastHeartbeatAt.Time.UTC())
	}
	if row.LastMqttSeenAt.Valid {
		out.LastMqttSeenAt = timestamppb.New(row.LastMqttSeenAt.Time.UTC())
	}
	return out
}

func machineResponseMetaOK(meta *machinev1.MachineRequestMeta) *machinev1.MachineResponseMeta {
	rid := ""
	if meta != nil {
		rid = strings.TrimSpace(meta.GetRequestId())
	}
	return &machinev1.MachineResponseMeta{
		ServerTime: timestamppb.Now(),
		RequestId:  rid,
		Status:     machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED,
	}
}
