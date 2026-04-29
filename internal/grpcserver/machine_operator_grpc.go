package grpcserver

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/avf/avf-vending-api/internal/domain/compliance"
	domainoperator "github.com/avf/avf-vending-api/internal/domain/operator"
	"github.com/avf/avf-vending-api/internal/gen/db"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type machineOperatorServer struct {
	machinev1.UnimplementedMachineOperatorServiceServer
	deps MachineGRPCServicesDeps
}

func (s *machineOperatorServer) OpenOperatorSession(context.Context, *machinev1.OpenOperatorSessionRequest) (*machinev1.OpenOperatorSessionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "OpenOperatorSession requires human identity proof; use HTTP POST .../operator-sessions/start (not Machine JWT alone)")
}

func (s *machineOperatorServer) CloseOperatorSession(context.Context, *machinev1.CloseOperatorSessionRequest) (*machinev1.CloseOperatorSessionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "CloseOperatorSession requires human identity proof; use HTTP POST .../operator-sessions/end")
}

func (s *machineOperatorServer) SubmitFillReport(ctx context.Context, req *machinev1.SubmitFillReportRequest) (*machinev1.SubmitFillReportResponse, error) {
	if req == nil || req.GetFill() == nil {
		return nil, status.Error(codes.InvalidArgument, "fill required")
	}
	out, err := (&machineInventoryServer{deps: s.deps}).SubmitFillResult(ctx, req.GetFill())
	if err != nil {
		return nil, err
	}
	return &machinev1.SubmitFillReportResponse{Fill: out}, nil
}

func (s *machineOperatorServer) SubmitStockAdjustment(ctx context.Context, req *machinev1.SubmitStockAdjustmentRequest) (*machinev1.SubmitStockAdjustmentResponse, error) {
	if req == nil || req.GetAdjustment() == nil {
		return nil, status.Error(codes.InvalidArgument, "adjustment required")
	}
	out, err := (&machineInventoryServer{deps: s.deps}).SubmitInventoryAdjustment(ctx, req.GetAdjustment())
	if err != nil {
		return nil, err
	}
	return &machinev1.SubmitStockAdjustmentResponse{Adjustment: out}, nil
}

func (s *machineOperatorServer) LoginOperator(context.Context, *machinev1.LoginOperatorRequest) (*machinev1.LoginOperatorResponse, error) {
	return nil, status.Error(codes.Unimplemented, "LoginOperator is deprecated; use OpenOperatorSession or HTTP operator-sessions routes")
}

func (s *machineOperatorServer) LogoutOperator(context.Context, *machinev1.LogoutOperatorRequest) (*machinev1.LogoutOperatorResponse, error) {
	return nil, status.Error(codes.Unimplemented, "LogoutOperator is deprecated; use CloseOperatorSession or HTTP operator-sessions routes")
}

func (s *machineOperatorServer) HeartbeatOperatorSession(ctx context.Context, req *machinev1.HeartbeatOperatorSessionRequest) (*machinev1.HeartbeatOperatorSessionResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	wctx, err := parseMachineMutationContext(ctx, req.GetContext())
	if err != nil {
		return nil, err
	}
	claims, ok := plauth.MachineAccessClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing machine credentials")
	}
	q := db.New(s.deps.Pool)
	if err := machineRuntimeInventoryGate(ctx, q, claims); err != nil {
		return nil, err
	}
	if s.deps.Operator == nil {
		return nil, status.Error(codes.Unavailable, "operator service not configured")
	}
	sid := strings.TrimSpace(req.GetSessionId())
	if sid == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id required")
	}
	sessionUUID, err := uuid.Parse(sid)
	if err != nil || sessionUUID == uuid.Nil {
		return nil, status.Error(codes.InvalidArgument, "invalid session_id")
	}
	if wctx.OperatorSessionID != nil && *wctx.OperatorSessionID != sessionUUID {
		return nil, status.Error(codes.InvalidArgument, "operator_session_id must match session_id")
	}

	sess, err := s.deps.Operator.HeartbeatOperatorSession(ctx, claims.OrganizationID, claims.MachineID, sessionUUID)
	if err != nil {
		return nil, mapOperatorHeartbeatError(err)
	}

	resp := &machinev1.HeartbeatOperatorSessionResponse{
		SessionId: sess.ID.String(),
		Status:    strings.TrimSpace(sess.Status),
	}
	if sess.ExpiresAt != nil {
		resp.ExpiresAt = timestamppb.New(*sess.ExpiresAt)
	}

	if s.deps.EnterpriseAudit != nil {
		meta, _ := json.Marshal(map[string]any{
			"idempotency_key": wctx.IdempotencyKey,
			"client_event_id": wctx.ClientEventID,
			"session_id":      sess.ID.String(),
			"session_status":  sess.Status,
		})
		machineIDStr := claims.MachineID.String()
		sessRID := sess.ID.String()
		_ = s.deps.EnterpriseAudit.Record(ctx, compliance.EnterpriseAuditRecord{
			OrganizationID: claims.OrganizationID,
			ActorType:      compliance.ActorMachine,
			ActorID:        &machineIDStr,
			Action:         compliance.ActionOperatorSessionHeartbeat,
			ResourceType:   "operator_session",
			ResourceID:     &sessRID,
			Metadata:       meta,
		})
	}

	return resp, nil
}

func mapOperatorHeartbeatError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domainoperator.ErrSessionNotFound):
		return status.Error(codes.NotFound, "operator session not found")
	case errors.Is(err, domainoperator.ErrSessionMachineMismatch):
		return status.Error(codes.PermissionDenied, "operator session machine mismatch")
	case errors.Is(err, domainoperator.ErrOrganizationMismatch):
		return status.Error(codes.PermissionDenied, "organization mismatch")
	case errors.Is(err, domainoperator.ErrSessionNotActive):
		return status.Error(codes.FailedPrecondition, "operator session not active")
	default:
		return status.Error(codes.Internal, "heartbeat failed")
	}
}
