package loadtest

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func authCtx(ctx context.Context, jwt string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+jwt)
}

func idem(key, evt string) *machinev1.IdempotencyContext {
	return &machinev1.IdempotencyContext{
		IdempotencyKey:  key,
		ClientEventId:   evt,
		ClientCreatedAt: timestamppb.Now(),
	}
}

// GRPCDial connects with plaintext credentials (staging tunnels often terminate TLS elsewhere).
func GRPCDial(ctx context.Context, addr string) (*grpc.ClientConn, error) {
	return grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
}

// GRPCPhasedRecorders splits load-test samples for storm reporting (recorders must be non-nil for phased calls).
type GRPCPhasedRecorders struct {
	Sync      *LatencyRecorder
	Telemetry *LatencyRecorder
	Offline   *LatencyRecorder
	Commerce  *LatencyRecorder // optional; used when productID is non-empty
}

// RunGRPCRuntime exercises bootstrap, catalog, media, telemetry, offline replay, optional cash+vend chain.
func RunGRPCRuntime(ctx context.Context, conn *grpc.ClientConn, m MachineRow, productID string, slotIndex int32, recorder *LatencyRecorder) error {
	return RunGRPCRuntimePhased(ctx, conn, m, productID, slotIndex, &GRPCPhasedRecorders{
		Sync: recorder, Telemetry: recorder, Offline: recorder, Commerce: recorder,
	})
}

// RunGRPCRuntimePhased is like RunGRPCRuntime but records sync (bootstrap/catalog/media), telemetry, offline, and commerce separately.
func RunGRPCRuntimePhased(ctx context.Context, conn *grpc.ClientConn, m MachineRow, productID string, slotIndex int32, phases *GRPCPhasedRecorders) error {
	if phases == nil || phases.Sync == nil || phases.Telemetry == nil || phases.Offline == nil {
		return fmt.Errorf("grpc phased: need non-nil Sync, Telemetry, Offline recorders")
	}
	jwt := m.JWT
	call := func(rec *LatencyRecorder, name string, fn func(context.Context) error) error {
		t0 := time.Now()
		err := fn(authCtx(ctx, jwt))
		rec.Add(time.Since(t0), err != nil)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		return nil
	}

	base := "lt-" + uuid.NewString()

	if err := call(phases.Sync, "GetBootstrap", func(c context.Context) error {
		_, err := machinev1.NewMachineBootstrapServiceClient(conn).GetBootstrap(c, &machinev1.GetBootstrapRequest{
			Meta: &machinev1.MachineRequestMeta{RequestId: base + "-gb"},
		})
		return err
	}); err != nil {
		return err
	}

	if err := call(phases.Sync, "GetCatalogSnapshot", func(c context.Context) error {
		_, err := machinev1.NewMachineCatalogServiceClient(conn).GetCatalogSnapshot(c, &machinev1.GetCatalogSnapshotRequest{
			MachineId:          m.MachineID.String(),
			IncludeUnavailable: false,
			Meta:               &machinev1.MachineRequestMeta{RequestId: base + "-cat"},
		})
		return err
	}); err != nil {
		return err
	}

	if err := call(phases.Sync, "GetMediaManifest", func(c context.Context) error {
		_, err := machinev1.NewMachineMediaServiceClient(conn).GetMediaManifest(c, &machinev1.MachineMediaServiceGetMediaManifestRequest{
			MachineId:          m.MachineID.String(),
			IncludeUnavailable: false,
			Meta:               &machinev1.MachineRequestMeta{RequestId: base + "-mm"},
		})
		return err
	}); err != nil {
		return err
	}

	if err := call(phases.Sync, "GetMediaDelta", func(c context.Context) error {
		_, err := machinev1.NewMachineMediaServiceClient(conn).GetMediaDelta(c, &machinev1.GetMediaDeltaRequest{
			MachineId:             m.MachineID.String(),
			BasisMediaFingerprint: "",
			Meta:                  &machinev1.MachineRequestMeta{RequestId: base + "-md"},
		})
		return err
	}); err != nil {
		return err
	}

	if err := call(phases.Telemetry, "PushTelemetryBatch", func(c context.Context) error {
		_, err := machinev1.NewMachineTelemetryServiceClient(conn).PushTelemetryBatch(c, &machinev1.PushTelemetryBatchRequest{
			Meta: &machinev1.MachineRequestMeta{RequestId: base + "-tel", IdempotencyKey: base + "-tel"},
			Events: []*machinev1.TelemetryEvent{
				{
					EventId:    base + "-e1",
					EventType:  "heartbeat",
					OccurredAt: timestamppb.Now(),
				},
			},
		})
		return err
	}); err != nil {
		return err
	}

	if err := call(phases.Offline, "GetSyncCursor", func(c context.Context) error {
		_, err := machinev1.NewMachineOfflineSyncServiceClient(conn).GetSyncCursor(c, &machinev1.GetSyncCursorRequest{
			Meta: &machinev1.MachineRequestMeta{RequestId: base + "-cur"},
		})
		return err
	}); err != nil {
		return err
	}

	if err := call(phases.Offline, "PushOfflineEvents", func(c context.Context) error {
		off := machinev1.NewMachineOfflineSyncServiceClient(conn)
		cur, err := off.GetSyncCursor(c, &machinev1.GetSyncCursorRequest{
			Meta: &machinev1.MachineRequestMeta{RequestId: base + "-cur2"},
		})
		if err != nil {
			return err
		}
		last := int64(0)
		if s := cur.GetSyncCursor(); s != "" {
			var perr error
			last, perr = strconv.ParseInt(s, 10, 64)
			if perr != nil {
				return perr
			}
		}
		seq := last + 1
		tb := &machinev1.SubmitTelemetryBatchRequest{
			Context: idem(base+"-offtb", base+"-offev"),
			Events: []*machinev1.TelemetryEvent{
				{EventId: base + "-oe1", EventType: "heartbeat", OccurredAt: timestamppb.Now()},
			},
		}
		payloadJSON, err := protojson.Marshal(tb)
		if err != nil {
			return err
		}
		var payloadStruct structpb.Struct
		if err := protojson.Unmarshal(payloadJSON, &payloadStruct); err != nil {
			return err
		}
		_, err = off.PushOfflineEvents(c, &machinev1.SyncOfflineEventsRequest{
			Meta: &machinev1.MachineRequestMeta{RequestId: base + "-sync", IdempotencyKey: base + "-sync"},
			Events: []*machinev1.OfflineEvent{
				{
					Meta: &machinev1.MachineRequestMeta{
						OfflineSequence: seq,
						IdempotencyKey:  base + "-off",
						RequestId:       base + "-oreq",
						ClientEventId:   base + "-ocli",
						OccurredAt:      timestamppb.Now(),
					},
					EventType: "telemetry.batch",
					Payload:   &payloadStruct,
				},
			},
		})
		return err
	}); err != nil {
		return err
	}

	if strings.TrimSpace(productID) == "" {
		return nil
	}

	if phases.Commerce == nil {
		return nil
	}

	pid := strings.TrimSpace(productID)
	return call(phases.Commerce, "CommerceCashVend", func(c context.Context) error {
		cc := machinev1.NewMachineCommerceServiceClient(conn)
		id := base + "-sale"
		co, err := cc.CreateOrder(authCtx(ctx, jwt), &machinev1.CreateOrderRequest{
			Context:   idem(id+":co", id+":evt-co"),
			ProductId: pid,
			Currency:  "USD",
			Slot:      &machinev1.SlotSelection{SlotIndex: &slotIndex},
		})
		if err != nil {
			return err
		}
		if _, err := cc.ConfirmCashPayment(authCtx(ctx, jwt), &machinev1.ConfirmCashPaymentRequest{
			Context: idem(id+":cash", id+":evt-cash"),
			OrderId: co.GetOrderId(),
		}); err != nil {
			return err
		}
		if _, err := cc.StartVend(authCtx(ctx, jwt), &machinev1.StartVendRequest{
			Context:   idem(id+":vstart", id+":evt-vs"),
			OrderId:   co.GetOrderId(),
			SlotIndex: slotIndex,
		}); err != nil {
			return err
		}
		_, err = cc.ConfirmVendSuccess(authCtx(ctx, jwt), &machinev1.ConfirmVendSuccessRequest{
			Context:   idem(id+":vsucc", id+":evt-vs"),
			OrderId:   co.GetOrderId(),
			SlotIndex: slotIndex,
		})
		return err
	})
}
