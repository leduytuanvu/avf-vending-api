package grpcserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/machineruntime"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/platform/observability/productionmetrics"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type machineSaleServer struct {
	machinev1.UnimplementedMachineSaleServiceServer
	deps MachineGRPCServicesDeps
}

type machineOfflineSyncServer struct {
	machinev1.UnimplementedMachineOfflineSyncServiceServer
	deps MachineGRPCServicesDeps
}

type machineCommandServer struct {
	machinev1.UnimplementedMachineCommandServiceServer
	deps MachineGRPCServicesDeps
}

func machineUnimplemented(reason string) error {
	st := status.New(codes.Unimplemented, reason)
	withDetails, err := st.WithDetails(&machinev1.GrpcErrorDetail{
		Domain: "avf.machine.v1",
		Reason: reason,
		Metadata: map[string]string{
			"status": "unimplemented",
		},
	})
	if err != nil {
		return st.Err()
	}
	return withDetails.Err()
}

func (s *machineBootstrapServer) AckConfigVersion(ctx context.Context, req *machinev1.AckConfigVersionRequest) (*machinev1.AckConfigVersionResponse, error) {
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
	if req != nil && req.GetAcknowledgedConfigVersion() > 0 {
		_ = q.PlanogramSnapshotUpdateMachineAckConfigRevision(ctx, db.PlanogramSnapshotUpdateMachineAckConfigRevisionParams{
			MachineID: claims.MachineID,
			LastAcknowledgedConfigRevision: pgtype.Int4{
				Int32: int32(req.GetAcknowledgedConfigVersion()),
				Valid: true,
			},
		})
	}
	if req != nil {
		if vid := strings.TrimSpace(req.GetAcknowledgedPlanogramVersionId()); vid != "" {
			uid, err := uuid.Parse(vid)
			if err != nil || uid == uuid.Nil {
				return nil, status.Error(codes.InvalidArgument, "invalid_acknowledged_planogram_version_id")
			}
			_ = q.PlanogramSnapshotUpdateMachineAckPlanogram(ctx, db.PlanogramSnapshotUpdateMachineAckPlanogramParams{
				MachineID:                          claims.MachineID,
				LastAcknowledgedPlanogramVersionID: pgtype.UUID{Bytes: uid, Valid: true},
			})
		}
	}
	var rid string
	if req != nil && req.GetMeta() != nil {
		rid = req.GetMeta().GetRequestId()
	}
	return &machinev1.AckConfigVersionResponse{
		Meta: responseMetaCtx(ctx, rid, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED),
	}, nil
}

func (s *machineInventoryServer) PushInventoryDelta(ctx context.Context, req *machinev1.ReportInventoryDeltaRequest) (*machinev1.ReportInventoryDeltaResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	out, err := s.SubmitInventoryAdjustment(ctx, &machinev1.SubmitInventoryAdjustmentRequest{
		Context: req.GetContext(),
		Reason:  req.GetReason(),
		Lines:   req.GetLines(),
	})
	if err != nil {
		return nil, err
	}
	rid := ""
	if req.GetMeta() != nil {
		rid = req.GetMeta().GetRequestId()
	}
	return &machinev1.ReportInventoryDeltaResponse{
		Replay:   out.GetReplay(),
		EventIds: out.GetEventIds(),
		Meta:     responseMetaCtx(ctx, rid, responseStatusFromReplay(out.GetReplay())),
	}, nil
}

func (s *machineTelemetryServer) PushTelemetryBatch(ctx context.Context, req *machinev1.PushTelemetryBatchRequest) (*machinev1.PushTelemetryBatchResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	out, err := s.SubmitTelemetryBatch(ctx, &machinev1.SubmitTelemetryBatchRequest{
		Context: req.GetContext(),
		Events:  req.GetEvents(),
	})
	if err != nil {
		return nil, err
	}
	rid := ""
	if req.GetMeta() != nil {
		rid = req.GetMeta().GetRequestId()
	}
	return &machinev1.PushTelemetryBatchResponse{
		Accepted:          out.GetAccepted(),
		AcceptedCount:     out.GetAcceptedCount(),
		DuplicateEventIds: out.GetDuplicateEventIds(),
		ServerReceivedAt:  out.GetServerReceivedAt(),
		Meta:              responseMetaCtx(ctx, rid, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED),
	}, nil
}

func (s *machineTelemetryServer) PushCriticalEvent(ctx context.Context, req *machinev1.PushCriticalEventRequest) (*machinev1.PushCriticalEventResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	ev := req.GetEvent()
	if ev != nil && strings.TrimSpace(req.GetSeverity()) != "" {
		cloned, ok := proto.Clone(ev).(*machinev1.TelemetryEvent)
		if !ok {
			return nil, status.Error(codes.Internal, "failed to clone telemetry event")
		}
		cloned.Attributes = cloneStringMap(ev.GetAttributes())
		cloned.Attributes["severity"] = strings.TrimSpace(req.GetSeverity())
		ev = cloned
	}
	out, err := s.SubmitTelemetryBatch(ctx, &machinev1.SubmitTelemetryBatchRequest{
		Context: req.GetContext(),
		Events:  []*machinev1.TelemetryEvent{ev},
	})
	if err != nil {
		return nil, err
	}
	rid := ""
	if req.GetMeta() != nil {
		rid = req.GetMeta().GetRequestId()
	}
	return &machinev1.PushCriticalEventResponse{
		Accepted:         out.GetAccepted(),
		Replay:           len(out.GetDuplicateEventIds()) > 0,
		ServerReceivedAt: out.GetServerReceivedAt(),
		Meta:             responseMetaCtx(ctx, rid, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED),
	}, nil
}

func (s *machineSaleServer) CreateSale(ctx context.Context, req *machinev1.CreateSaleRequest) (*machinev1.CreateSaleResponse, error) {
	out, err := (&machineCommerceServer{deps: s.deps}).CreateOrder(ctx, req.GetOrder())
	if err != nil {
		return nil, err
	}
	return &machinev1.CreateSaleResponse{Order: out}, nil
}

func (s *machineSaleServer) AttachPayment(ctx context.Context, req *machinev1.AttachPaymentRequest) (*machinev1.AttachPaymentResponse, error) {
	out, err := (&machineCommerceServer{deps: s.deps}).CreatePaymentSession(ctx, req.GetPaymentSession())
	if err != nil {
		return nil, err
	}
	return &machinev1.AttachPaymentResponse{PaymentSession: out}, nil
}

func (s *machineSaleServer) StartVend(ctx context.Context, req *machinev1.StartVendRequest) (*machinev1.StartVendResponse, error) {
	return (&machineCommerceServer{deps: s.deps}).StartVend(ctx, req)
}

func (s *machineSaleServer) CompleteVend(ctx context.Context, req *machinev1.ConfirmVendSuccessRequest) (*machinev1.ConfirmVendSuccessResponse, error) {
	return (&machineCommerceServer{deps: s.deps}).ConfirmVendSuccess(ctx, req)
}

func (s *machineSaleServer) FailVend(ctx context.Context, req *machinev1.ReportVendFailureRequest) (*machinev1.ReportVendFailureResponse, error) {
	return (&machineCommerceServer{deps: s.deps}).ReportVendFailure(ctx, req)
}

func (s *machineSaleServer) ConfirmCashReceived(ctx context.Context, req *machinev1.ConfirmCashReceivedRequest) (*machinev1.ConfirmCashReceivedResponse, error) {
	out, err := (&machineCommerceServer{deps: s.deps}).ConfirmCashPayment(ctx, req.GetPayment())
	if err != nil {
		return nil, err
	}
	return &machinev1.ConfirmCashReceivedResponse{Payment: out}, nil
}

func (s *machineSaleServer) CancelSale(ctx context.Context, req *machinev1.CancelOrderRequest) (*machinev1.CancelOrderResponse, error) {
	return (&machineCommerceServer{deps: s.deps}).CancelOrder(ctx, req)
}

func (s *machineOfflineSyncServer) PushOfflineEvents(ctx context.Context, req *machinev1.SyncOfflineEventsRequest) (*machinev1.SyncOfflineEventsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	claims, ok := plauth.MachineAccessClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing machine credentials")
	}
	if s.deps.Pool == nil {
		return nil, status.Error(codes.Unavailable, "offline sync ledger not configured")
	}
	maxOffline := 200
	if s.deps.Config != nil {
		maxOffline = s.deps.Config.Capacity.MaxOfflineEventsPerRequest
	}
	if len(req.GetEvents()) > maxOffline {
		return nil, status.Errorf(codes.InvalidArgument, "too many offline events (max %d)", maxOffline)
	}
	q := db.New(s.deps.Pool)
	if err := machineCredentialGate(ctx, q, claims); err != nil {
		return nil, err
	}
	events := append([]*machinev1.OfflineEvent(nil), req.GetEvents()...)
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].GetMeta().GetOfflineSequence() < events[j].GetMeta().GetOfflineSequence()
	})
	cursor, err := q.GetMachineSyncCursor(ctx, db.GetMachineSyncCursorParams{
		OrganizationID: claims.OrganizationID,
		MachineID:      claims.MachineID,
		StreamName:     "offline",
	})
	if errors.Is(err, pgx.ErrNoRows) {
		cursor.LastSequence = 0
	} else if err != nil {
		return nil, status.Error(codes.Internal, "offline cursor lookup failed")
	}
	expected := cursor.LastSequence + 1
	results := make([]*machinev1.OfflineEventResult, 0, len(events))
	for _, ev := range events {
		if ev == nil || ev.GetMeta() == nil {
			return nil, status.Error(codes.InvalidArgument, "offline event meta required")
		}
		seq := ev.GetMeta().GetOfflineSequence()
		if seq <= cursor.LastSequence {
			productionmetrics.RecordOfflineEventResult("skipped_already_synced")
			results = append(results, &machinev1.OfflineEventResult{
				OfflineSequence: seq,
				IdempotencyKey:  ev.GetMeta().GetIdempotencyKey(),
				Status:          machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_REPLAYED,
				Reason:          "offline sequence already synced",
			})
			continue
		}
		if seq != expected {
			return nil, machineruntime.OfflineSequenceOutOfOrder(expected, seq)
		}
		occAt := time.Now().UTC()
		if ev.GetMeta().GetOccurredAt() != nil && ev.GetMeta().GetOccurredAt().IsValid() {
			occAt = ev.GetMeta().GetOccurredAt().AsTime().UTC()
		}
		result := s.processOfflineEvent(ctx, q, claims, ev)
		if lag := time.Since(occAt); lag >= 0 {
			productionmetrics.ObserveMachineSyncLag(lag)
		}
		recordOfflineOutcomeMetrics(result)
		results = append(results, result)
		if result.GetStatus() == machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_REJECTED {
			break
		}
		cursor.LastSequence = seq
		expected++
		if _, err := q.UpsertMachineSyncCursor(ctx, db.UpsertMachineSyncCursorParams{
			OrganizationID: claims.OrganizationID,
			MachineID:      claims.MachineID,
			StreamName:     "offline",
			LastSequence:   seq,
		}); err != nil {
			return nil, status.Error(codes.Internal, "offline cursor update failed")
		}
	}
	return &machinev1.SyncOfflineEventsResponse{
		Meta:           responseMetaCtx(ctx, req.GetMeta().GetRequestId(), machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED),
		Results:        results,
		NextSyncCursor: strings.TrimPrefix(strings.TrimSpace(time.Now().UTC().Format(time.RFC3339Nano)), ""),
	}, nil
}

func (s *machineOfflineSyncServer) processOfflineEvent(ctx context.Context, q *db.Queries, claims plauth.MachineAccessClaims, ev *machinev1.OfflineEvent) *machinev1.OfflineEventResult {
	meta := ev.GetMeta()
	seq := meta.GetOfflineSequence()
	idem := strings.TrimSpace(meta.GetIdempotencyKey())
	clientEventID := strings.TrimSpace(meta.GetClientEventId())
	eventRequestID := strings.TrimSpace(meta.GetRequestId())
	eventType := strings.TrimSpace(ev.GetEventType())
	payload, err := protojson.Marshal(ev.GetPayload())
	if err != nil || len(payload) == 0 {
		payload = []byte(`{}`)
	}
	occurredAt := time.Now().UTC()
	if ts := meta.GetOccurredAt(); ts != nil && ts.IsValid() {
		occurredAt = ts.AsTime().UTC()
	}

	if clientEventID != "" {
		prior, err := q.GetMachineOfflineEventByClientEventID(ctx, db.GetMachineOfflineEventByClientEventIDParams{
			OrganizationID: claims.OrganizationID,
			MachineID:      claims.MachineID,
			ClientEventID:  clientEventID,
		})
		switch {
		case err == nil:
			if prior.OfflineSequence != seq {
				return &machinev1.OfflineEventResult{
					OfflineSequence: seq,
					IdempotencyKey:  idem,
					Status:          machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_REJECTED,
					Reason:          fmt.Sprintf("duplicate client_event_id %q already recorded at offline_sequence %d", clientEventID, prior.OfflineSequence),
				}
			}
			if offlineLedgerTerminalStatus(prior.ProcessingStatus) {
				return &machinev1.OfflineEventResult{
					OfflineSequence: seq,
					IdempotencyKey:  idem,
					Status:          machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_REPLAYED,
					Reason:          "offline event replayed",
				}
			}
		case errors.Is(err, pgx.ErrNoRows):
		default:
			return &machinev1.OfflineEventResult{
				OfflineSequence: seq,
				IdempotencyKey:  idem,
				Status:          machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_REJECTED,
				Reason:          "offline duplicate lookup failed",
			}
		}
	}

	row, err := q.InsertMachineOfflineEvent(ctx, db.InsertMachineOfflineEventParams{
		OrganizationID:   claims.OrganizationID,
		MachineID:        claims.MachineID,
		OfflineSequence:  seq,
		EventType:        eventType,
		EventID:          eventRequestID,
		ClientEventID:    clientEventID,
		OccurredAt:       occurredAt,
		Payload:          payload,
		ProcessingStatus: "processing",
		ProcessingError:  "",
		IdempotencyKey:   idem,
	})
	if err != nil {
		var pe *pgconn.PgError
		if errors.As(err, &pe) && pe.Code == "23505" && clientEventID != "" {
			prior, qerr := q.GetMachineOfflineEventByClientEventID(ctx, db.GetMachineOfflineEventByClientEventIDParams{
				OrganizationID: claims.OrganizationID,
				MachineID:      claims.MachineID,
				ClientEventID:  clientEventID,
			})
			if qerr == nil {
				if prior.OfflineSequence != seq {
					return &machinev1.OfflineEventResult{
						OfflineSequence: seq,
						IdempotencyKey:  idem,
						Status:          machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_REJECTED,
						Reason:          fmt.Sprintf("duplicate client_event_id %q already recorded at offline_sequence %d", clientEventID, prior.OfflineSequence),
					}
				}
				if offlineLedgerTerminalStatus(prior.ProcessingStatus) {
					return &machinev1.OfflineEventResult{
						OfflineSequence: seq,
						IdempotencyKey:  idem,
						Status:          machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_REPLAYED,
						Reason:          "offline event replayed",
					}
				}
			}
		}
		return &machinev1.OfflineEventResult{OfflineSequence: seq, IdempotencyKey: idem, Status: machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_REJECTED, Reason: "offline event insert failed"}
	}
	if !row.Inserted && offlineLedgerTerminalStatus(row.ProcessingStatus) {
		return &machinev1.OfflineEventResult{OfflineSequence: seq, IdempotencyKey: idem, Status: machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_REPLAYED, Reason: "offline event replayed"}
	}
	if err := s.dispatchOfflineEvent(ctx, eventType, payload); err != nil {
		code := status.Code(err)
		productionmetrics.RecordOfflineReplayFailure(code.String())
		st := "failed"
		resultStatus := machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_REJECTED
		if code == codes.Unimplemented || code == codes.InvalidArgument {
			st = "rejected"
		}
		_ = q.UpdateMachineOfflineEventStatus(ctx, db.UpdateMachineOfflineEventStatusParams{
			OrganizationID:   claims.OrganizationID,
			MachineID:        claims.MachineID,
			OfflineSequence:  seq,
			ProcessingStatus: st,
			ProcessingError:  err.Error(),
		})
		return &machinev1.OfflineEventResult{OfflineSequence: seq, IdempotencyKey: idem, Status: resultStatus, Reason: err.Error()}
	}
	_ = q.UpdateMachineOfflineEventStatus(ctx, db.UpdateMachineOfflineEventStatusParams{
		OrganizationID:   claims.OrganizationID,
		MachineID:        claims.MachineID,
		OfflineSequence:  seq,
		ProcessingStatus: "processed",
		ProcessingError:  "",
	})
	return &machinev1.OfflineEventResult{OfflineSequence: seq, IdempotencyKey: idem, Status: machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED}
}

func recordOfflineOutcomeMetrics(result *machinev1.OfflineEventResult) {
	if result == nil {
		return
	}
	switch result.GetStatus() {
	case machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED:
		productionmetrics.RecordOfflineEventResult("accepted")
	case machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_REPLAYED:
		productionmetrics.RecordOfflineEventResult("replayed")
	case machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_REJECTED:
		productionmetrics.RecordOfflineEventResult("rejected")
	default:
		productionmetrics.RecordOfflineEventResult("unknown")
	}
}

func offlineLedgerTerminalStatus(st string) bool {
	switch strings.ToLower(strings.TrimSpace(st)) {
	case "succeeded", "processed", "replayed", "duplicate", "rejected":
		return true
	default:
		return false
	}
}

func (s *machineOfflineSyncServer) dispatchOfflineEvent(ctx context.Context, eventType string, payload []byte) error {
	switch strings.ToLower(strings.TrimSpace(eventType)) {
	case "commerce.create_order", "sale.create_order":
		var req machinev1.CreateOrderRequest
		if err := protojson.Unmarshal(payload, &req); err != nil {
			return status.Error(codes.InvalidArgument, "invalid create_order payload")
		}
		_, err := (&machineCommerceServer{deps: s.deps}).CreateOrder(ctx, &req)
		return err
	case "commerce.create_payment_session", "sale.create_payment_session", "commerce.attach_payment_result":
		var req machinev1.CreatePaymentSessionRequest
		if err := protojson.Unmarshal(payload, &req); err != nil {
			return status.Error(codes.InvalidArgument, "invalid create_payment_session payload")
		}
		_, err := (&machineCommerceServer{deps: s.deps}).CreatePaymentSession(ctx, &req)
		return err
	case "commerce.confirm_cash_payment", "sale.report_cash_payment", "commerce.create_cash_checkout":
		var req machinev1.ConfirmCashPaymentRequest
		if err := protojson.Unmarshal(payload, &req); err != nil {
			return status.Error(codes.InvalidArgument, "invalid cash payment payload")
		}
		_, err := (&machineCommerceServer{deps: s.deps}).ConfirmCashPayment(ctx, &req)
		return err
	case "commerce.start_vend", "sale.start_vend":
		var req machinev1.StartVendRequest
		if err := protojson.Unmarshal(payload, &req); err != nil {
			return status.Error(codes.InvalidArgument, "invalid start_vend payload")
		}
		_, err := (&machineCommerceServer{deps: s.deps}).StartVend(ctx, &req)
		return err
	case "commerce.confirm_vend_success", "sale.confirm_vend_success":
		var req machinev1.ConfirmVendSuccessRequest
		if err := protojson.Unmarshal(payload, &req); err != nil {
			return status.Error(codes.InvalidArgument, "invalid vend success payload")
		}
		_, err := (&machineCommerceServer{deps: s.deps}).ConfirmVendSuccess(ctx, &req)
		return err
	case "commerce.confirm_vend_failure", "sale.confirm_vend_failure":
		var req machinev1.ReportVendFailureRequest
		if err := protojson.Unmarshal(payload, &req); err != nil {
			return status.Error(codes.InvalidArgument, "invalid vend failure payload")
		}
		_, err := (&machineCommerceServer{deps: s.deps}).ReportVendFailure(ctx, &req)
		return err
	case "commerce.cancel_order", "sale.cancel_sale":
		var req machinev1.CancelOrderRequest
		if err := protojson.Unmarshal(payload, &req); err != nil {
			return status.Error(codes.InvalidArgument, "invalid cancel payload")
		}
		_, err := (&machineCommerceServer{deps: s.deps}).CancelOrder(ctx, &req)
		return err
	case "inventory.report_delta", "inventory.adjustment", "inventory.submit_stock_adjustment":
		var req machinev1.SubmitInventoryAdjustmentRequest
		if err := protojson.Unmarshal(payload, &req); err != nil {
			return status.Error(codes.InvalidArgument, "invalid inventory adjustment payload")
		}
		_, err := (&machineInventoryServer{deps: s.deps}).SubmitInventoryAdjustment(ctx, &req)
		return err
	case "inventory.restock":
		var req machinev1.SubmitRestockRequest
		if err := protojson.Unmarshal(payload, &req); err != nil {
			return status.Error(codes.InvalidArgument, "invalid restock payload")
		}
		_, err := (&machineInventoryServer{deps: s.deps}).SubmitRestock(ctx, &req)
		return err
	case "inventory.submit_fill_report":
		var req machinev1.SubmitFillResultRequest
		if err := protojson.Unmarshal(payload, &req); err != nil {
			return status.Error(codes.InvalidArgument, "invalid fill report payload")
		}
		_, err := (&machineInventoryServer{deps: s.deps}).SubmitFillResult(ctx, &req)
		return err
	case "telemetry.batch":
		var req machinev1.SubmitTelemetryBatchRequest
		if err := protojson.Unmarshal(payload, &req); err != nil {
			return status.Error(codes.InvalidArgument, "invalid telemetry batch payload")
		}
		_, err := (&machineTelemetryServer{deps: s.deps}).SubmitTelemetryBatch(ctx, &req)
		return err
	case "telemetry.critical":
		var req machinev1.PushCriticalEventRequest
		if err := protojson.Unmarshal(payload, &req); err != nil {
			return status.Error(codes.InvalidArgument, "invalid critical telemetry payload")
		}
		_, err := (&machineTelemetryServer{deps: s.deps}).PushCriticalEvent(ctx, &req)
		return err
	default:
		return status.Errorf(codes.Unimplemented, "offline event_type %q not supported", eventType)
	}
}

func (s *machineOfflineSyncServer) GetSyncCursor(ctx context.Context, req *machinev1.GetSyncCursorRequest) (*machinev1.GetSyncCursorResponse, error) {
	claims, ok := plauth.MachineAccessClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing machine credentials")
	}
	if s.deps.Pool == nil {
		return nil, status.Error(codes.Unavailable, "offline sync ledger not configured")
	}
	q := db.New(s.deps.Pool)
	cursor, err := q.GetMachineSyncCursor(ctx, db.GetMachineSyncCursorParams{
		OrganizationID: claims.OrganizationID,
		MachineID:      claims.MachineID,
		StreamName:     "offline",
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return &machinev1.GetSyncCursorResponse{
			Meta:       responseMetaCtx(ctx, req.GetMeta().GetRequestId(), machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED),
			SyncCursor: "0",
		}, nil
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "offline cursor lookup failed")
	}
	return &machinev1.GetSyncCursorResponse{
		Meta:       responseMetaCtx(ctx, req.GetMeta().GetRequestId(), machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED),
		SyncCursor: strconv.FormatInt(cursor.LastSequence, 10),
	}, nil
}

func (s *machineCommandServer) GetPendingCommands(context.Context, *machinev1.GetPendingCommandsRequest) (*machinev1.GetPendingCommandsResponse, error) {
	return nil, machineUnimplemented("GetPendingCommands is not a primary delivery path — backend→machine commands use MQTT TLS + command ledger + ACK (see docs/api/mqtt-contract.md)")
}

func (s *machineCommandServer) AckCommand(context.Context, *machinev1.AckCommandRequest) (*machinev1.AckCommandResponse, error) {
	return nil, machineUnimplemented("AckCommand is not a primary delivery path — backend→machine commands use MQTT TLS + command ledger + ACK (see docs/api/mqtt-contract.md)")
}

func (s *machineCommandServer) RejectCommand(context.Context, *machinev1.RejectCommandRequest) (*machinev1.RejectCommandResponse, error) {
	return nil, machineUnimplemented("RejectCommand is not a primary delivery path — backend→machine commands use MQTT TLS + command ledger + ACK (see docs/api/mqtt-contract.md)")
}

func (s *machineCommandServer) GetAssignedUpdate(ctx context.Context, req *machinev1.GetAssignedUpdateRequest) (*machinev1.GetAssignedUpdateResponse, error) {
	if req == nil {
		req = &machinev1.GetAssignedUpdateRequest{}
	}
	claims, ok := plauth.MachineAccessClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing machine credentials")
	}
	q := db.New(s.deps.Pool)
	if err := machineCredentialGate(ctx, q, claims); err != nil {
		return nil, err
	}
	row, err := q.DeviceGetAssignedOTAUpdate(ctx, db.DeviceGetAssignedOTAUpdateParams{
		MachineID:      claims.MachineID,
		OrganizationID: claims.OrganizationID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return &machinev1.GetAssignedUpdateResponse{
			Meta: responseMetaCtx(ctx, req.GetMeta().GetRequestId(), machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED),
		}, nil
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "assigned update lookup failed")
	}
	return &machinev1.GetAssignedUpdateResponse{
		Meta: responseMetaCtx(ctx, req.GetMeta().GetRequestId(), machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED),
		Update: &machinev1.AssignedUpdate{
			CampaignId:      row.CampaignID.String(),
			ArtifactId:      row.ArtifactID.String(),
			ArtifactVersion: row.ArtifactVersion,
			CampaignType:    row.CampaignType,
			StorageKey:      row.StorageKey,
			Sha256Hex:       pgTextToString(row.Sha256Hex),
			SizeBytes:       pgInt8ToInt64(row.SizeBytes),
			Status:          row.Status,
		},
	}, nil
}

func (s *machineCommandServer) ReportUpdateStatus(ctx context.Context, req *machinev1.ReportUpdateStatusRequest) (*machinev1.ReportUpdateStatusResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	claims, ok := plauth.MachineAccessClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing machine credentials")
	}
	campaignID, err := uuidFromString(req.GetCampaignId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid campaign_id")
	}
	st := normalizeOTAStatus(req.GetStatus())
	if st == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid update status")
	}
	q := db.New(s.deps.Pool)
	if err := machineCredentialGate(ctx, q, claims); err != nil {
		return nil, err
	}
	_, err = q.DeviceReportOTAResult(ctx, db.DeviceReportOTAResultParams{
		MachineID:      claims.MachineID,
		CampaignID:     campaignID,
		OrganizationID: claims.OrganizationID,
		Status:         st,
		LastError:      pgText(strings.TrimSpace(req.GetErrorMessage())),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, status.Error(codes.PermissionDenied, "ota_result_not_assigned_to_machine")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "ota result update failed")
	}
	s.recordMachineCommandAudit(ctx, claims, "machine.ota.status_reported", "ota_campaign", campaignID.String(), map[string]any{
		"status": st,
	})
	return &machinev1.ReportUpdateStatusResponse{
		Meta: responseMetaCtx(ctx, req.GetMeta().GetRequestId(), machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED),
	}, nil
}

func (s *machineCommandServer) ReportDiagnosticBundleResult(ctx context.Context, req *machinev1.ReportDiagnosticBundleResultRequest) (*machinev1.ReportDiagnosticBundleResultResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	claims, ok := plauth.MachineAccessClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing machine credentials")
	}
	if strings.TrimSpace(req.GetStorageKey()) == "" {
		return nil, status.Error(codes.InvalidArgument, "storage_key required")
	}
	requestID, err := uuidFromString(req.GetRequestId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request_id")
	}
	meta := []byte(`{}`)
	if req.GetMetadata() != nil {
		if b, err := protojson.Marshal(req.GetMetadata()); err == nil && json.Valid(b) {
			meta = b
		}
	}
	var expires pgtype.Timestamptz
	if ts := req.GetExpiresAt(); ts != nil && ts.IsValid() {
		expires = pgtype.Timestamptz{Time: ts.AsTime().UTC(), Valid: true}
	}
	q := db.New(s.deps.Pool)
	if err := machineCredentialGate(ctx, q, claims); err != nil {
		return nil, err
	}
	row, err := q.DeviceInsertDiagnosticBundleManifest(ctx, db.DeviceInsertDiagnosticBundleManifestParams{
		OrganizationID:  pgtype.UUID{Bytes: claims.OrganizationID, Valid: true},
		MachineID:       claims.MachineID,
		RequestID:       pgtype.UUID{Bytes: requestID, Valid: true},
		CommandID:       pgtype.UUID{},
		StorageKey:      strings.TrimSpace(req.GetStorageKey()),
		StorageProvider: defaultString(req.GetStorageProvider(), "s3"),
		ContentType:     pgText(strings.TrimSpace(req.GetContentType())),
		SizeBytes:       pgtype.Int8{Int64: req.GetSizeBytes(), Valid: req.GetSizeBytes() > 0},
		Sha256Hex:       pgText(strings.TrimSpace(req.GetSha256Hex())),
		Metadata:        meta,
		Status:          "available",
		ExpiresAt:       expires,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "diagnostic manifest insert failed")
	}
	s.recordMachineCommandAudit(ctx, claims, "machine.diagnostic.bundle_reported", "diagnostic_bundle", row.ID.String(), map[string]any{
		"request_id":  requestID.String(),
		"storage_key": strings.TrimSpace(req.GetStorageKey()),
	})
	return &machinev1.ReportDiagnosticBundleResultResponse{
		Meta:     responseMetaCtx(ctx, req.GetMeta().GetRequestId(), machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED),
		BundleId: row.ID.String(),
	}, nil
}

func (s *machineCommandServer) recordMachineCommandAudit(ctx context.Context, claims plauth.MachineAccessClaims, action, resourceType, resourceID string, metadata map[string]any) {
	if s == nil || s.deps.EnterpriseAudit == nil {
		return
	}
	actorID := claims.MachineID.String()
	rid := strings.TrimSpace(resourceID)
	meta, _ := json.Marshal(metadata)
	if len(meta) == 0 {
		meta = []byte(`{}`)
	}
	_ = s.deps.EnterpriseAudit.Record(ctx, compliance.EnterpriseAuditRecord{
		OrganizationID: claims.OrganizationID,
		ActorType:      compliance.ActorMachine,
		ActorID:        &actorID,
		Action:         action,
		ResourceType:   resourceType,
		ResourceID:     &rid,
		Metadata:       meta,
	})
}

func normalizeOTAStatus(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "acked":
		return "acked"
	case "downloaded":
		return "downloaded"
	case "installed", "success":
		return "installed"
	case "failed":
		return "failed"
	default:
		return ""
	}
}

func uuidFromString(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(s))
	if err != nil || id == uuid.Nil {
		return uuid.Nil, errors.New("invalid uuid")
	}
	return id, nil
}

func pgText(s string) pgtype.Text {
	s = strings.TrimSpace(s)
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func pgTextToString(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

func pgInt8ToInt64(v pgtype.Int8) int64 {
	if !v.Valid {
		return 0
	}
	return v.Int64
}

func defaultString(s, def string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	return s
}

func responseStatusFromReplay(replay bool) machinev1.MachineResponseStatus {
	if replay {
		return machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_REPLAYED
	}
	return machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED
}

func cloneStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in)+1)
	for k, v := range in {
		out[k] = v
	}
	return out
}
