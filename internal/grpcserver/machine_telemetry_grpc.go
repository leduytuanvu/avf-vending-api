package grpcserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/platform/observability/productionmetrics"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	actionTelemetryCheckIn        = "telemetry.checkin"
	actionTelemetryBatchSubmitted = "telemetry.batch_submitted"
)

type machineTelemetryServer struct {
	machinev1.UnimplementedMachineTelemetryServiceServer
	deps MachineGRPCServicesDeps
}

func (s *machineTelemetryServer) CheckIn(ctx context.Context, req *machinev1.CheckInRequest) (*machinev1.CheckInResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	claims, q, err := s.telemetryAuth(ctx)
	if err != nil {
		return nil, err
	}
	if err := resolveTelemetryMachineScope(claims.MachineID, req.GetMachineId()); err != nil {
		return nil, err
	}
	wctx, err := parseMachineMutationContext(ctx, req.GetContext())
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(map[string]any{
		"android_id":        strings.TrimSpace(req.GetAndroidId()),
		"sim_serial":        strings.TrimSpace(req.GetSimSerial()),
		"package_name":      strings.TrimSpace(req.GetPackageName()),
		"version_name":      strings.TrimSpace(req.GetVersionName()),
		"version_code":      req.GetVersionCode(),
		"android_release":   strings.TrimSpace(req.GetAndroidRelease()),
		"sdk_int":           req.GetSdkInt(),
		"manufacturer":      strings.TrimSpace(req.GetManufacturer()),
		"model":             strings.TrimSpace(req.GetModel()),
		"timezone":          strings.TrimSpace(req.GetTimezone()),
		"network_state":     strings.TrimSpace(req.GetNetworkState()),
		"boot_id":           strings.TrimSpace(req.GetBootId()),
		"metadata":          req.GetMetadata(),
		"client_event_id":   wctx.ClientEventID,
		"client_created_at": wctx.ClientCreatedAt.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "check-in payload failed")
	}
	dup, err := s.appendTelemetryEventWithConflictCheck(ctx, claims.MachineID, "checkin", payload, wctx.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if !dup {
		meta, _ := json.Marshal(req.GetMetadata())
		if len(meta) == 0 {
			meta = []byte("{}")
		}
		if !json.Valid(meta) {
			return nil, status.Error(codes.InvalidArgument, "metadata must be valid")
		}
		if _, err := q.InsertMachineCheckIn(ctx, db.InsertMachineCheckInParams{
			ID:             claims.MachineID,
			AndroidID:      stringToPgText(req.GetAndroidId()),
			SimSerial:      stringToPgText(req.GetSimSerial()),
			PackageName:    strings.TrimSpace(req.GetPackageName()),
			VersionName:    strings.TrimSpace(req.GetVersionName()),
			VersionCode:    req.GetVersionCode(),
			AndroidRelease: strings.TrimSpace(req.GetAndroidRelease()),
			SdkInt:         req.GetSdkInt(),
			Manufacturer:   strings.TrimSpace(req.GetManufacturer()),
			Model:          strings.TrimSpace(req.GetModel()),
			Timezone:       strings.TrimSpace(req.GetTimezone()),
			NetworkState:   strings.TrimSpace(req.GetNetworkState()),
			BootID:         strings.TrimSpace(req.GetBootId()),
			OccurredAt:     wctx.ClientCreatedAt,
			Metadata:       meta,
		}); err != nil {
			return nil, status.Error(codes.Internal, "check-in insert failed")
		}
		_ = q.UpdateMachineCurrentSnapshotLastCheckIn(ctx, db.UpdateMachineCurrentSnapshotLastCheckInParams{
			MachineID:     claims.MachineID,
			LastCheckInAt: pgtype.Timestamptz{Time: wctx.ClientCreatedAt, Valid: true},
		})
		s.recordTelemetryAudit(ctx, claims, actionTelemetryCheckIn, map[string]any{
			"idempotency_key": wctx.IdempotencyKey,
			"client_event_id": wctx.ClientEventID,
		})
	}
	productionmetrics.RecordMachineCheckIn("grpc")
	return &machinev1.CheckInResponse{
		Replay:           dup,
		MachineId:        claims.MachineID.String(),
		ServerReceivedAt: timestamppb.New(now),
	}, nil
}

func (s *machineTelemetryServer) SubmitTelemetryBatch(ctx context.Context, req *machinev1.SubmitTelemetryBatchRequest) (*machinev1.SubmitTelemetryBatchResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	claims, _, err := s.telemetryAuth(ctx)
	if err != nil {
		return nil, err
	}
	wctx, err := parseMachineMutationContext(ctx, req.GetContext())
	if err != nil {
		return nil, err
	}
	maxEvents := 500
	maxBytes := 2 << 20
	if s.deps.Config != nil {
		maxEvents = s.deps.Config.Capacity.MaxTelemetryGRPCBatchEvents
		maxBytes = s.deps.Config.Capacity.MaxTelemetryGRPCBatchBytes
	}
	if proto.Size(req) > maxBytes {
		return nil, status.Error(codes.InvalidArgument, "telemetry batch exceeds maximum serialized size")
	}
	if len(req.GetEvents()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "events required")
	}
	if len(req.GetEvents()) > maxEvents {
		return nil, status.Error(codes.InvalidArgument, "too many events")
	}
	duplicates := make([]string, 0)
	accepted := int32(0)
	for i, ev := range req.GetEvents() {
		if ev == nil {
			return nil, status.Error(codes.InvalidArgument, "event must not be null")
		}
		if strings.TrimSpace(ev.GetEventType()) == "" {
			return nil, status.Error(codes.InvalidArgument, "event_type required")
		}
		occurred := ev.GetOccurredAt()
		if occurred == nil || !occurred.IsValid() {
			return nil, status.Error(codes.InvalidArgument, "event occurred_at required")
		}
		eventKey := telemetryEventDedupeKey(wctx.IdempotencyKey, ev.GetEventId(), i)
		payload, err := json.Marshal(map[string]any{
			"event_id":        strings.TrimSpace(ev.GetEventId()),
			"event_type":      strings.TrimSpace(ev.GetEventType()),
			"occurred_at":     occurred.AsTime().UTC().Format(time.RFC3339Nano),
			"attributes":      ev.GetAttributes(),
			"boot_id":         strings.TrimSpace(ev.GetBootId()),
			"client_sequence": ev.GetClientSequence(),
			"batch_event_id":  wctx.ClientEventID,
		})
		if err != nil {
			return nil, status.Error(codes.Internal, "telemetry payload failed")
		}
		dup, err := s.appendTelemetryEventWithConflictCheck(ctx, claims.MachineID, ev.GetEventType(), payload, eventKey)
		if err != nil {
			return nil, err
		}
		if dup {
			id := strings.TrimSpace(ev.GetEventId())
			if id == "" {
				id = eventKey
			}
			duplicates = append(duplicates, id)
			continue
		}
		accepted++
	}
	if accepted > 0 {
		s.recordTelemetryAudit(ctx, claims, actionTelemetryBatchSubmitted, map[string]any{
			"idempotency_key": wctx.IdempotencyKey,
			"client_event_id": wctx.ClientEventID,
			"accepted_count":  accepted,
			"duplicate_count": len(duplicates),
		})
	}
	return &machinev1.SubmitTelemetryBatchResponse{
		Accepted:          true,
		AcceptedCount:     accepted,
		DuplicateEventIds: duplicates,
		ServerReceivedAt:  timestamppb.New(time.Now().UTC()),
	}, nil
}

func (s *machineTelemetryServer) ReconcileEvents(ctx context.Context, req *machinev1.ReconcileEventsRequest) (*machinev1.ReconcileEventsResponse, error) {
	claims, q, err := s.telemetryAuth(ctx)
	if err != nil {
		return nil, err
	}
	keys := req.GetIdempotencyKeys()
	if len(keys) < 1 || len(keys) > 500 {
		return nil, status.Error(codes.InvalidArgument, "idempotency_keys must contain 1 to 500 entries")
	}
	items := make([]*machinev1.TelemetryEventStatus, 0, len(keys))
	for _, raw := range keys {
		item, err := telemetryStatusForKey(ctx, q, claims.MachineID, raw)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return &machinev1.ReconcileEventsResponse{MachineId: claims.MachineID.String(), Items: items}, nil
}

func (s *machineTelemetryServer) GetEventStatus(ctx context.Context, req *machinev1.GetEventStatusRequest) (*machinev1.GetEventStatusResponse, error) {
	claims, q, err := s.telemetryAuth(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	item, err := telemetryStatusForKey(ctx, q, claims.MachineID, req.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	return &machinev1.GetEventStatusResponse{Item: item}, nil
}

func (s *machineTelemetryServer) telemetryAuth(ctx context.Context) (plauth.MachineAccessClaims, *db.Queries, error) {
	claims, ok := plauth.MachineAccessClaimsFromContext(ctx)
	if !ok {
		return plauth.MachineAccessClaims{}, nil, status.Error(codes.Unauthenticated, "missing machine credentials")
	}
	if s.deps.TelemetryStore == nil || s.deps.Pool == nil {
		return plauth.MachineAccessClaims{}, nil, status.Error(codes.Unavailable, "telemetry store not configured")
	}
	q := db.New(s.deps.Pool)
	if err := machineCredentialGate(ctx, q, claims); err != nil {
		return plauth.MachineAccessClaims{}, nil, err
	}
	return claims, q, nil
}

func (s *machineTelemetryServer) appendTelemetryEventWithConflictCheck(ctx context.Context, machineID uuid.UUID, eventType string, payload []byte, dedupeKey string) (bool, error) {
	existing, ok, err := s.existingTelemetryPayload(ctx, machineID, dedupeKey)
	if err != nil {
		return false, err
	}
	if ok {
		if !bytes.Equal(existing, payload) {
			return false, status.Error(codes.Aborted, "idempotency key conflict")
		}
		return true, nil
	}
	dup, err := s.deps.TelemetryStore.AppendDeviceTelemetryEdgeEvent(ctx, machineID, strings.TrimSpace(eventType), payload, dedupeKey)
	if err != nil {
		return false, status.Error(codes.Internal, "telemetry append failed")
	}
	if dup {
		existing, ok, err := s.existingTelemetryPayload(ctx, machineID, dedupeKey)
		if err != nil {
			return false, err
		}
		if ok && !bytes.Equal(existing, payload) {
			return false, status.Error(codes.Aborted, "idempotency key conflict")
		}
	}
	return dup, nil
}

func (s *machineTelemetryServer) existingTelemetryPayload(ctx context.Context, machineID uuid.UUID, dedupeKey string) ([]byte, bool, error) {
	var payload []byte
	err := s.deps.Pool.QueryRow(ctx, `SELECT payload FROM device_telemetry_events WHERE machine_id = $1 AND dedupe_key = $2`, machineID, strings.TrimSpace(dedupeKey)).Scan(&payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, status.Error(codes.Internal, "telemetry idempotency lookup failed")
	}
	return payload, true, nil
}

func telemetryStatusForKey(ctx context.Context, q *db.Queries, machineID uuid.UUID, raw string) (*machinev1.TelemetryEventStatus, error) {
	key := strings.TrimSpace(raw)
	if key == "" {
		return nil, status.Error(codes.InvalidArgument, "idempotency_key required")
	}
	row, err := q.GetCriticalTelemetryEventStatus(ctx, db.GetCriticalTelemetryEventStatusParams{
		MachineID:      machineID,
		IdempotencyKey: key,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return &machinev1.TelemetryEventStatus{IdempotencyKey: key, Status: "not_found", Retryable: true}, nil
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "event status lookup failed")
	}
	out := &machinev1.TelemetryEventStatus{
		IdempotencyKey: key,
		Status:         row.Status,
		Retryable:      row.Status != "processed" && row.Status != "failed_terminal",
	}
	if row.EventType.Valid {
		out.EventType = row.EventType.String
	}
	if row.AcceptedAt.Valid {
		out.AcceptedAt = timestamppb.New(row.AcceptedAt.Time.UTC())
	}
	if row.ProcessedAt.Valid {
		out.ProcessedAt = timestamppb.New(row.ProcessedAt.Time.UTC())
	}
	return out, nil
}

func (s *machineTelemetryServer) recordTelemetryAudit(ctx context.Context, claims plauth.MachineAccessClaims, action string, meta map[string]any) {
	if s.deps.EnterpriseAudit == nil {
		return
	}
	md, _ := json.Marshal(meta)
	mid := claims.MachineID.String()
	_ = s.deps.EnterpriseAudit.Record(ctx, compliance.EnterpriseAuditRecord{
		OrganizationID: claims.OrganizationID,
		ActorType:      compliance.ActorMachine,
		ActorID:        &mid,
		Action:         action,
		ResourceType:   "machine",
		ResourceID:     &mid,
		Metadata:       md,
	})
}

func telemetryEventDedupeKey(batchKey, eventID string, index int) string {
	eventID = strings.TrimSpace(eventID)
	if eventID != "" {
		return strings.TrimSpace(batchKey) + ":" + eventID
	}
	return fmt.Sprintf("%s:%d", strings.TrimSpace(batchKey), index)
}

func resolveTelemetryMachineScope(tokenMachine uuid.UUID, requestMachine string) error {
	if strings.TrimSpace(requestMachine) == "" {
		return nil
	}
	_, err := resolveMachineScope(tokenMachine, requestMachine)
	return err
}

func stringToPgText(s string) pgtype.Text {
	if strings.TrimSpace(s) == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: strings.TrimSpace(s), Valid: true}
}
