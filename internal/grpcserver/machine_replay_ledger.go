package grpcserver

import (
	"context"
	"fmt"

	"github.com/avf/avf-vending-api/internal/app/machineidempotency"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// MachineReplayLedger persists mutation idempotency keys and JSON response snapshots in PostgreSQL.
type MachineReplayLedger = machineidempotency.Ledger

// NewMachineReplayLedger wires the sqlc-backed ledger used by the unary replay interceptor.
func NewMachineReplayLedger(pool *pgxpool.Pool, audit compliance.EnterpriseRecorder) *MachineReplayLedger {
	return machineidempotency.NewLedger(pool, audit)
}

func canonicalMachineMutationOperation(fullMethod string) string {
	switch fullMethod {
	case machinev1.MachineCommerceService_CreateCashCheckout_FullMethodName:
		return machinev1.MachineCommerceService_ConfirmCashPayment_FullMethodName
	case machinev1.MachineCommerceService_AttachPaymentResult_FullMethodName:
		return machinev1.MachineCommerceService_CreatePaymentSession_FullMethodName
	case machinev1.MachineInventoryService_SubmitFillReport_FullMethodName:
		return machinev1.MachineInventoryService_SubmitFillResult_FullMethodName
	case machinev1.MachineInventoryService_SubmitStockAdjustment_FullMethodName:
		return machinev1.MachineInventoryService_SubmitInventoryAdjustment_FullMethodName
	default:
		return fullMethod
	}
}

func newUnaryMachineReplayInterceptor(cfg *config.Config, ledger *MachineReplayLedger) grpc.UnaryServerInterceptor {
	if cfg != nil && !cfg.GRPC.RequireGRPCIdempotency {
		return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			return handler(ctx, req)
		}
	}
	if ledger == nil {
		return machineGRPCMutationsRequireLedgerInterceptor()
	}
	return unaryMachineReplayInterceptor(ledger)
}

func machineGRPCMutationsRequireLedgerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if isMachineIdempotentMutation(info.FullMethod) {
			return nil, status.Error(codes.Internal, "machine idempotency ledger required but not configured")
		}
		return handler(ctx, req)
	}
}

func unaryMachineReplayInterceptor(ledger *MachineReplayLedger) grpc.UnaryServerInterceptor {
	return machineidempotency.UnaryServerInterceptor(ledger, machineidempotency.InterceptorConfig{
		IsIdempotentMutation:         isMachineIdempotentMutation,
		NewMutationResponse:          newMachineMutationResponse,
		CanonicalOperation:           canonicalMachineMutationOperation,
		DecorateLedgerReplayResponse: decorateMachineLedgerReplayResponse,
		TraceID: func(ctx context.Context) string {
			if meta, ok := GRPCRequestMetaFromContext(ctx); ok {
				return meta.RequestID
			}
			return ""
		},
	})
}

// decorateMachineLedgerReplayResponse sets replay flags on outer responses returned from the ledger so
// clients observe consistent retry semantics (first completion stores replay=false).
func decorateMachineLedgerReplayResponse(resp proto.Message) {
	switch m := resp.(type) {
	case *machinev1.CreateOrderResponse:
		m.Replay = true
	case *machinev1.CreatePaymentSessionResponse:
		m.Replay = true
	case *machinev1.ConfirmCashPaymentResponse:
		m.Replay = true
	case *machinev1.StartVendResponse:
		m.Replay = true
	case *machinev1.ConfirmVendSuccessResponse:
		m.Replay = true
	case *machinev1.ReportVendSuccessResponse:
		m.Replay = true
	case *machinev1.ReportVendFailureResponse:
		m.Replay = true
	case *machinev1.CancelOrderResponse:
		m.Replay = true
	case *machinev1.CreateSaleResponse:
		if o := m.GetOrder(); o != nil {
			o.Replay = true
		}
	case *machinev1.AttachPaymentResponse:
		if p := m.GetPaymentSession(); p != nil {
			p.Replay = true
		}
	case *machinev1.ConfirmCashReceivedResponse:
		if p := m.GetPayment(); p != nil {
			p.Replay = true
		}
	case *machinev1.ReportInventoryDeltaResponse:
		m.Replay = true
	case *machinev1.SubmitStockSnapshotResponse:
		m.Replay = true
	case *machinev1.SubmitFillResultResponse:
		m.Replay = true
	case *machinev1.SubmitRestockResponse:
		m.Replay = true
	case *machinev1.SubmitInventoryAdjustmentResponse:
		m.Replay = true
	case *machinev1.SubmitFillReportResponse:
		if f := m.GetFill(); f != nil {
			f.Replay = true
		}
	case *machinev1.SubmitStockAdjustmentResponse:
		if a := m.GetAdjustment(); a != nil {
			a.Replay = true
		}
	case *machinev1.CheckInResponse:
		m.Replay = true
	case *machinev1.PushCriticalEventResponse:
		m.Replay = true
	default:
	}
}

func isMachineIdempotentMutation(fullMethod string) bool {
	switch fullMethod {
	case machinev1.MachineCommerceService_CreateOrder_FullMethodName,
		machinev1.MachineCommerceService_CreatePaymentSession_FullMethodName,
		machinev1.MachineCommerceService_AttachPaymentResult_FullMethodName,
		machinev1.MachineCommerceService_ConfirmCashPayment_FullMethodName,
		machinev1.MachineCommerceService_CreateCashCheckout_FullMethodName,
		machinev1.MachineCommerceService_StartVend_FullMethodName,
		machinev1.MachineCommerceService_ConfirmVendSuccess_FullMethodName,
		machinev1.MachineCommerceService_ReportVendSuccess_FullMethodName,
		machinev1.MachineCommerceService_ReportVendFailure_FullMethodName,
		machinev1.MachineCommerceService_CancelOrder_FullMethodName,
		machinev1.MachineSaleService_CreateSale_FullMethodName,
		machinev1.MachineSaleService_AttachPayment_FullMethodName,
		machinev1.MachineSaleService_ConfirmCashReceived_FullMethodName,
		machinev1.MachineSaleService_StartVend_FullMethodName,
		machinev1.MachineSaleService_CompleteVend_FullMethodName,
		machinev1.MachineSaleService_FailVend_FullMethodName,
		machinev1.MachineSaleService_CancelSale_FullMethodName,
		machinev1.MachineInventoryService_PushInventoryDelta_FullMethodName,
		machinev1.MachineInventoryService_SubmitStockSnapshot_FullMethodName,
		machinev1.MachineInventoryService_SubmitFillResult_FullMethodName,
		machinev1.MachineInventoryService_SubmitFillReport_FullMethodName,
		machinev1.MachineInventoryService_SubmitRestock_FullMethodName,
		machinev1.MachineInventoryService_SubmitInventoryAdjustment_FullMethodName,
		machinev1.MachineInventoryService_SubmitStockAdjustment_FullMethodName,
		machinev1.MachineOperatorService_SubmitFillReport_FullMethodName,
		machinev1.MachineOperatorService_SubmitStockAdjustment_FullMethodName,
		machinev1.MachineOperatorService_HeartbeatOperatorSession_FullMethodName,
		machinev1.MachineTelemetryService_CheckIn_FullMethodName,
		machinev1.MachineTelemetryService_SubmitTelemetryBatch_FullMethodName,
		machinev1.MachineTelemetryService_PushTelemetryBatch_FullMethodName,
		machinev1.MachineTelemetryService_PushCriticalEvent_FullMethodName,
		machinev1.MachineTelemetryService_ReconcileEvents_FullMethodName,
		machinev1.MachineOfflineSyncService_PushOfflineEvents_FullMethodName,
		machinev1.MachineBootstrapService_CheckIn_FullMethodName,
		machinev1.MachineBootstrapService_AckConfigVersion_FullMethodName,
		machinev1.MachineCatalogService_AckCatalogVersion_FullMethodName,
		machinev1.MachineInventoryService_AckInventorySync_FullMethodName,
		machinev1.MachineMediaService_AckMediaVersion_FullMethodName,
		machinev1.MachineCommandService_ReportUpdateStatus_FullMethodName,
		machinev1.MachineCommandService_ReportDiagnosticBundleResult_FullMethodName:
		return true
	default:
		return false
	}
}

func newMachineMutationResponse(fullMethod string) proto.Message {
	canon := canonicalMachineMutationOperation(fullMethod)
	switch canon {
	case machinev1.MachineCommerceService_CreateOrder_FullMethodName:
		return &machinev1.CreateOrderResponse{}
	case machinev1.MachineCommerceService_CreatePaymentSession_FullMethodName:
		return &machinev1.CreatePaymentSessionResponse{}
	case machinev1.MachineCommerceService_ConfirmCashPayment_FullMethodName:
		return &machinev1.ConfirmCashPaymentResponse{}
	case machinev1.MachineCommerceService_StartVend_FullMethodName:
		return &machinev1.StartVendResponse{}
	case machinev1.MachineCommerceService_ConfirmVendSuccess_FullMethodName:
		return &machinev1.ConfirmVendSuccessResponse{}
	case machinev1.MachineCommerceService_ReportVendSuccess_FullMethodName:
		return &machinev1.ReportVendSuccessResponse{}
	case machinev1.MachineCommerceService_ReportVendFailure_FullMethodName:
		return &machinev1.ReportVendFailureResponse{}
	case machinev1.MachineCommerceService_CancelOrder_FullMethodName:
		return &machinev1.CancelOrderResponse{}
	case machinev1.MachineSaleService_CreateSale_FullMethodName:
		return &machinev1.CreateSaleResponse{}
	case machinev1.MachineSaleService_AttachPayment_FullMethodName:
		return &machinev1.AttachPaymentResponse{}
	case machinev1.MachineSaleService_ConfirmCashReceived_FullMethodName:
		return &machinev1.ConfirmCashReceivedResponse{}
	case machinev1.MachineSaleService_StartVend_FullMethodName:
		return &machinev1.StartVendResponse{}
	case machinev1.MachineSaleService_CompleteVend_FullMethodName:
		return &machinev1.ConfirmVendSuccessResponse{}
	case machinev1.MachineSaleService_FailVend_FullMethodName:
		return &machinev1.ReportVendFailureResponse{}
	case machinev1.MachineSaleService_CancelSale_FullMethodName:
		return &machinev1.CancelOrderResponse{}
	case machinev1.MachineInventoryService_PushInventoryDelta_FullMethodName:
		return &machinev1.ReportInventoryDeltaResponse{}
	case machinev1.MachineInventoryService_SubmitStockSnapshot_FullMethodName:
		return &machinev1.SubmitStockSnapshotResponse{}
	case machinev1.MachineInventoryService_SubmitFillResult_FullMethodName:
		return &machinev1.SubmitFillResultResponse{}
	case machinev1.MachineInventoryService_SubmitRestock_FullMethodName:
		return &machinev1.SubmitRestockResponse{}
	case machinev1.MachineInventoryService_SubmitInventoryAdjustment_FullMethodName:
		return &machinev1.SubmitInventoryAdjustmentResponse{}
	case machinev1.MachineOperatorService_SubmitFillReport_FullMethodName:
		return &machinev1.SubmitFillReportResponse{}
	case machinev1.MachineOperatorService_SubmitStockAdjustment_FullMethodName:
		return &machinev1.SubmitStockAdjustmentResponse{}
	case machinev1.MachineOperatorService_HeartbeatOperatorSession_FullMethodName:
		return &machinev1.HeartbeatOperatorSessionResponse{}
	case machinev1.MachineTelemetryService_CheckIn_FullMethodName:
		return &machinev1.CheckInResponse{}
	case machinev1.MachineTelemetryService_SubmitTelemetryBatch_FullMethodName:
		return &machinev1.SubmitTelemetryBatchResponse{}
	case machinev1.MachineTelemetryService_PushTelemetryBatch_FullMethodName:
		return &machinev1.PushTelemetryBatchResponse{}
	case machinev1.MachineTelemetryService_PushCriticalEvent_FullMethodName:
		return &machinev1.PushCriticalEventResponse{}
	case machinev1.MachineTelemetryService_ReconcileEvents_FullMethodName:
		return &machinev1.ReconcileEventsResponse{}
	case machinev1.MachineOfflineSyncService_PushOfflineEvents_FullMethodName:
		return &machinev1.SyncOfflineEventsResponse{}
	case machinev1.MachineBootstrapService_CheckIn_FullMethodName:
		return &machinev1.MachineBootstrapServiceCheckInResponse{}
	case machinev1.MachineBootstrapService_AckConfigVersion_FullMethodName:
		return &machinev1.AckConfigVersionResponse{}
	case machinev1.MachineCatalogService_AckCatalogVersion_FullMethodName:
		return &machinev1.AckCatalogVersionResponse{}
	case machinev1.MachineInventoryService_AckInventorySync_FullMethodName:
		return &machinev1.AckInventorySyncResponse{}
	case machinev1.MachineMediaService_AckMediaVersion_FullMethodName:
		return &machinev1.AckMediaVersionResponse{}
	case machinev1.MachineCommandService_ReportUpdateStatus_FullMethodName:
		return &machinev1.ReportUpdateStatusResponse{}
	case machinev1.MachineCommandService_ReportDiagnosticBundleResult_FullMethodName:
		return &machinev1.ReportDiagnosticBundleResultResponse{}
	default:
		panic(fmt.Sprintf("missing machine mutation response type for %s (canon=%s)", fullMethod, canon))
	}
}
