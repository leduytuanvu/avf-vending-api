package grpcserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/alerts"
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
	"google.golang.org/protobuf/encoding/protojson"
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
		// Mark machine connectivity as online/offline for inventory-gated RPCs.
		// Best-effort: telemetry is not allowed to fail due to connectivity bookkeeping.
		_ = q.TouchMachineConnectivity(ctx, claims.MachineID)
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
		eventType := strings.TrimSpace(ev.GetEventType())
		if alerts.IsProjectableIncidentEventType(eventType) {
			projector := s.deps.IncidentProjector
			if projector == nil && s.deps.TelemetryStore != nil {
				projector = s.deps.TelemetryStore
			}
			if projector != nil {
				occurrenceID := strings.TrimSpace(ev.GetEventId())
				if occurrenceID == "" {
					occurrenceID = alerts.ExtractOccurrenceIDFromDetail(payload)
				}
				sev := "high"
				if attrs := ev.GetAttributes(); attrs != nil {
					if v := strings.TrimSpace(attrs["severity"]); v != "" {
						sev = v
					}
				}
				title := eventType
				if attrs := ev.GetAttributes(); attrs != nil {
					if v := strings.TrimSpace(attrs["verified_message"]); v != "" {
						title = v
					} else if v := strings.TrimSpace(attrs["title"]); v != "" {
						title = v
					}
				}
				fingerprint := ""
				if attrs := ev.GetAttributes(); attrs != nil {
					fingerprint = strings.TrimSpace(attrs["fingerprint"])
					if fingerprint == "" {
						fingerprint = strings.TrimSpace(attrs["dedupe_key"])
					}
				}
				policy := alerts.DefaultPolicy()
				if s.deps.Config != nil {
					policy = alerts.Policy{
						Cooldown:   s.deps.Config.Telegram.IncidentCooldown,
						RepeatMode: alerts.NormalizeRepeatMode(s.deps.Config.Telegram.RepeatMode),
					}
				}
				// Always invoke projection (even on telemetry duplicate) to heal partial failures.
				_, projErr := projector.ProjectMachineIncident(ctx, alerts.ProjectInput{
					MachineID:    claims.MachineID.String(),
					OccurrenceID: occurrenceID,
					Fingerprint:  fingerprint,
					Severity:     sev,
					Code:         eventType,
					Title:        title,
					EventType:    eventType,
					Transport:    "grpc",
					Detail:       payload,
					OccurredAt:   occurred.AsTime().UTC(),
				}, policy)
				if projErr != nil {
					return nil, status.Errorf(codes.Internal, "incident projection failed")
				}
			}
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

func (s *machineTelemetryServer) SubmitEventEvidenceBatch(ctx context.Context, req *machinev1.SubmitEventEvidenceBatchRequest) (*machinev1.SubmitEventEvidenceBatchResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	claims, _, err := s.telemetryAuth(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := parseMachineMutationContext(ctx, req.GetContext()); err != nil {
		return nil, err
	}
	maxEvents := 500
	maxBytes := 2 << 20
	if s.deps.Config != nil {
		maxEvents = s.deps.Config.Capacity.MaxTelemetryGRPCBatchEvents
		maxBytes = s.deps.Config.Capacity.MaxTelemetryGRPCBatchBytes
	}
	if proto.Size(req) > maxBytes {
		return nil, status.Error(codes.ResourceExhausted, "evidence batch exceeds maximum serialized size")
	}
	if len(req.GetEvents()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "events required")
	}
	if len(req.GetEvents()) > maxEvents {
		return nil, status.Errorf(codes.ResourceExhausted, "too many evidence events (max %d)", maxEvents)
	}
	requestID := strings.TrimSpace(req.GetMeta().GetRequestId())
	if requestID == "" {
		requestID = strings.TrimSpace(req.GetContext().GetIdempotencyKey())
	}
	receivedAt := time.Now().UTC()
	results := make([]*machinev1.EventEvidenceResult, 0, len(req.GetEvents()))
	tx, err := s.deps.Pool.Begin(ctx)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "evidence ledger unavailable")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := db.New(tx)
	for _, ev := range req.GetEvents() {
		result := persistEventEvidence(ctx, qtx, claims.MachineID, requestID, ev)
		productionmetrics.RecordMachineEventEvidence(evidenceResultLabel(result.GetStatus()))
		results = append(results, result)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, status.Error(codes.Unavailable, "evidence commit failed")
	}
	return &machinev1.SubmitEventEvidenceBatchResponse{
		Meta:             responseMetaCtx(ctx, requestID, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED),
		Results:          results,
		ServerReceivedAt: timestamppb.New(receivedAt),
	}, nil
}

func persistEventEvidence(ctx context.Context, q *db.Queries, machineID uuid.UUID, requestID string, ev *machinev1.EventEvidence) *machinev1.EventEvidenceResult {
	if ev == nil {
		return &machinev1.EventEvidenceResult{
			Status: machinev1.EventEvidenceResultStatus_EVENT_EVIDENCE_RESULT_STATUS_REJECTED,
			Reason: "empty evidence event",
		}
	}
	eventID := strings.TrimSpace(ev.GetEventId())
	eventType := strings.TrimSpace(ev.GetEventType())
	if eventID == "" {
		return &machinev1.EventEvidenceResult{
			Status: machinev1.EventEvidenceResultStatus_EVENT_EVIDENCE_RESULT_STATUS_REJECTED,
			Reason: "event_id required",
		}
	}
	if eventType == "" {
		return &machinev1.EventEvidenceResult{
			EventId: eventID,
			Status:  machinev1.EventEvidenceResultStatus_EVENT_EVIDENCE_RESULT_STATUS_REJECTED,
			Reason:  "event_type required",
		}
	}
	if ev.GetOccurredAt() == nil || !ev.GetOccurredAt().IsValid() {
		return &machinev1.EventEvidenceResult{
			EventId: eventID,
			Status:  machinev1.EventEvidenceResultStatus_EVENT_EVIDENCE_RESULT_STATUS_REJECTED,
			Reason:  "occurred_at required",
		}
	}
	payload, err := protojson.Marshal(ev.GetPayload())
	if err != nil || len(payload) == 0 {
		payload = []byte(`{}`)
	}
	fingerprint := offlinePayloadFingerprint(eventType, payload)
	schemaVersion := ev.GetSchemaVersion()
	if schemaVersion < 1 {
		schemaVersion = 1
	}
	processingStatus := "accepted"
	if !knownSemanticEvidenceType(eventType) {
		processingStatus = "unrecognized"
	}
	row, err := q.InsertMachineEventEvidence(ctx, db.InsertMachineEventEvidenceParams{
		MachineID:          machineID,
		EventID:            eventID,
		EventType:          eventType,
		SchemaVersion:      schemaVersion,
		Category:           strings.TrimSpace(ev.GetCategory()),
		Severity:           strings.TrimSpace(ev.GetSeverity()),
		Source:             firstNonEmpty(strings.TrimSpace(ev.GetSource()), "device"),
		StreamID:           strings.TrimSpace(ev.GetStreamId()),
		ClientSequence:     ev.GetClientSequence(),
		BootID:             strings.TrimSpace(ev.GetBootId()),
		OccurredAt:         ev.GetOccurredAt().AsTime().UTC(),
		MonotonicElapsedMs: ev.GetMonotonicElapsedMs(),
		OrderID:            strings.TrimSpace(ev.GetOrderId()),
		PaymentID:          strings.TrimSpace(ev.GetPaymentId()),
		VendAttemptID:      strings.TrimSpace(ev.GetVendAttemptId()),
		CorrelationID:      strings.TrimSpace(ev.GetCorrelationId()),
		OperatorSessionID:  strings.TrimSpace(ev.GetOperatorSessionId()),
		RequestID:          requestID,
		Cause:              strings.TrimSpace(ev.GetCause()),
		RecoveryAction:     strings.TrimSpace(ev.GetRecoveryAction()),
		Payload:            payload,
		PayloadFingerprint: fingerprint,
		ProcessingStatus:   processingStatus,
	})
	if err != nil {
		return &machinev1.EventEvidenceResult{
			EventId: eventID,
			Status:  machinev1.EventEvidenceResultStatus_EVENT_EVIDENCE_RESULT_STATUS_REJECTED,
			Reason:  "evidence insert failed",
		}
	}
	if !row.Inserted {
		if offlineContentConflicts(row.EventType, row.PayloadFingerprint, row.Payload, eventType, fingerprint, payload) {
			return &machinev1.EventEvidenceResult{
				EventId: eventID,
				Status:  machinev1.EventEvidenceResultStatus_EVENT_EVIDENCE_RESULT_STATUS_CONFLICT,
				Reason:  "evidence identity conflict: payload or event_type differs",
			}
		}
		return &machinev1.EventEvidenceResult{
			EventId: eventID,
			Status:  machinev1.EventEvidenceResultStatus_EVENT_EVIDENCE_RESULT_STATUS_DUPLICATE,
			Reason:  "evidence already stored",
		}
	}
	return &machinev1.EventEvidenceResult{
		EventId: eventID,
		Status:  machinev1.EventEvidenceResultStatus_EVENT_EVIDENCE_RESULT_STATUS_ACCEPTED,
	}
}

func evidenceResultLabel(st machinev1.EventEvidenceResultStatus) string {
	switch st {
	case machinev1.EventEvidenceResultStatus_EVENT_EVIDENCE_RESULT_STATUS_ACCEPTED:
		return "accepted"
	case machinev1.EventEvidenceResultStatus_EVENT_EVIDENCE_RESULT_STATUS_DUPLICATE:
		return "duplicate"
	case machinev1.EventEvidenceResultStatus_EVENT_EVIDENCE_RESULT_STATUS_CONFLICT:
		return "conflict"
	case machinev1.EventEvidenceResultStatus_EVENT_EVIDENCE_RESULT_STATUS_REJECTED:
		return "rejected"
	default:
		return "unspecified"
	}
}

func knownSemanticEvidenceType(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case "payment_status_transition",
		"vend_result",
		"technician_session_open",
		"technician_session_close",
		"operator_session_open",
		"operator_session_close",
		"refill_confirmed",
		"config_apply_result",
		"machine_bootstrap_confirmation",
		"bill_stacked_cashbox",
		"bill_stored_recycler",
		"bill_rejected",
		"commerce_order_created",
		"commerce_cash_confirmed",
		"commerce_vend_started",
		"commerce_vend_succeeded",
		"commerce_vend_failed",
		"commerce_order_cancelled":
		return true
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
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
		if !telemetryJSONPayloadEqual(existing, payload) {
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
		if ok && !telemetryJSONPayloadEqual(existing, payload) {
			return false, status.Error(codes.Aborted, "idempotency key conflict")
		}
	}
	return dup, nil
}

// telemetryJSONPayloadEqual compares telemetry payloads semantically.
// device_telemetry_events.payload is jsonb; Postgres may re-serialize key order on read-back,
// so raw bytes.Equal falsely reports conflicts on legitimate idempotent retries.
func telemetryJSONPayloadEqual(a, b []byte) bool {
	if bytes.Equal(a, b) {
		return true
	}
	var xa, xb any
	if err := json.Unmarshal(a, &xa); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &xb); err != nil {
		return false
	}
	return reflect.DeepEqual(xa, xb)
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
		ActorType:    compliance.ActorMachine,
		ActorID:      &mid,
		Action:       action,
		ResourceType: "machine",
		ResourceID:   &mid,
		Metadata:     md,
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
