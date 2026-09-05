package grpcserver

import (
	"context"
	"strings"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/platform/observability/productionmetrics"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *machineCommerceServer) CreateQuote(ctx context.Context, req *machinev1.CreateQuoteRequest) (*machinev1.CreateQuoteResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	wctx, err := parseMachineMutationContext(ctx, req.GetContext())
	if err != nil {
		return nil, err
	}
	claims, svc, _, err := s.requireCommerce(ctx)
	if err != nil {
		return nil, err
	}
	if mid := strings.TrimSpace(req.GetMachineId()); mid != "" {
		parsed, perr := uuid.Parse(mid)
		if perr != nil || parsed != claims.MachineID {
			return nil, status.Error(codes.PermissionDenied, "machine_id does not match token")
		}
	}
	cur := strings.ToUpper(strings.TrimSpace(req.GetCurrency()))
	if len(cur) != 3 {
		return nil, status.Error(codes.InvalidArgument, "currency must be a 3-letter ISO code")
	}
	if len(req.GetLines()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "lines required")
	}
	var lines []appcommerce.QuoteLineInput
	for _, ln := range req.GetLines() {
		productID, perr := uuid.Parse(strings.TrimSpace(ln.GetProductId()))
		if perr != nil || productID == uuid.Nil {
			return nil, status.Error(codes.InvalidArgument, "invalid product_id")
		}
		slotID, cab, sc, slotIdx, serr := parseSlotProto(ln.GetSlot())
		if serr != nil {
			return nil, serr
		}
		qty := ln.GetQuantity()
		if qty <= 0 {
			qty = 1
		}
		lines = append(lines, appcommerce.QuoteLineInput{
			ProductID:   productID,
			SlotID:      slotID,
			CabinetCode: cab,
			SlotCode:    sc,
			SlotIndex:   slotIdx,
			Quantity:    qty,
		})
	}
	var pricingSnap *appcommerce.MachinePricingSnapshotInput
	if req.GetPricingSnapshot() != nil {
		snap, perr := appcommerce.MachinePricingSnapshotFromProto(req.GetPricingSnapshot())
		if perr != nil {
			return nil, status.Error(codes.InvalidArgument, perr.Error())
		}
		pricingSnap = &snap
	}
	out, err := svc.CreateQuote(ctx, appcommerce.CreateQuoteInput{
		MachineID:       claims.MachineID,
		Currency:        cur,
		PaymentMethod:   strings.TrimSpace(req.GetPaymentMethod()),
		Lines:           lines,
		IdempotencyKey:  wctx.IdempotencyKey,
		PricingSnapshot: pricingSnap,
	})
	if err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	if !out.Replay {
		s.auditCommerce(ctx, claims, compliance.ActionMachineCommerceOrderCreated, map[string]any{
			"quote_id":        out.QuoteID.String(),
			"idempotency_key": wctx.IdempotencyKey,
			"line_count":      len(out.Lines),
		})
		productionmetrics.RecordOrderCreated("grpc_machine_quote")
	}
	respLines := make([]*machinev1.QuoteLineResponse, 0, len(out.Lines))
	for _, l := range out.Lines {
		respLines = append(respLines, &machinev1.QuoteLineResponse{
			LineSequence:       l.LineSequence,
			ProductId:          l.ProductID.String(),
			CabinetCode:        l.CabinetCode,
			SlotCode:           l.SlotCode,
			SlotIndex:          l.SlotIndex,
			Quantity:           l.Quantity,
			UnitPriceMinor:     l.UnitPriceMinor,
			LineSubtotalMinor:  l.LineSubtotalMinor,
			PricingFingerprint: l.PricingFingerprint,
		})
	}
	zap.L().Info("CreateQuote ok", zap.String("quote_id", out.QuoteID.String()), zap.Bool("replay", out.Replay))
	return &machinev1.CreateQuoteResponse{
		Replay:                      out.Replay,
		QuoteId:                     out.QuoteID.String(),
		Currency:                    out.Currency,
		PaymentMethod:               out.PaymentMethod,
		SubtotalMinor:               out.SubtotalMinor,
		DiscountMinor:               out.DiscountMinor,
		PayableMinor:                out.PayableMinor,
		ExpiresAt:                   timestamppb.New(out.ExpiresAt),
		Lines:                       respLines,
		PricingSource:               out.PricingSource,
		ServerReferencePayableMinor: out.ServerReferencePayableMinor,
	}, nil
}

func (s *machineCommerceServer) CreateOrderFromQuote(ctx context.Context, req *machinev1.CreateOrderFromQuoteRequest) (*machinev1.CreateOrderFromQuoteResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	wctx, err := parseMachineMutationContext(ctx, req.GetContext())
	if err != nil {
		return nil, err
	}
	claims, svc, _, err := s.requireCommerce(ctx)
	if err != nil {
		return nil, err
	}
	if mid := strings.TrimSpace(req.GetMachineId()); mid != "" {
		parsed, perr := uuid.Parse(mid)
		if perr != nil || parsed != claims.MachineID {
			return nil, status.Error(codes.PermissionDenied, "machine_id does not match token")
		}
	}
	quoteID, err := uuid.Parse(strings.TrimSpace(req.GetQuoteId()))
	if err != nil || quoteID == uuid.Nil {
		return nil, status.Error(codes.InvalidArgument, "invalid quote_id")
	}
	simMeta := parseSimulationContext(req.GetSimulation())
	if err := validateSimulationCommerce(claims.MachineID, simMeta, s.deps.Config.AppEnv); err != nil {
		return nil, err
	}
	var orderPricingSnap *appcommerce.MachinePricingSnapshotInput
	if req.GetPricingSnapshot() != nil {
		snap, perr := appcommerce.MachinePricingSnapshotFromProto(req.GetPricingSnapshot())
		if perr != nil {
			return nil, status.Error(codes.InvalidArgument, perr.Error())
		}
		orderPricingSnap = &snap
	}
	out, err := svc.CreateOrderFromQuote(ctx, appcommerce.CreateOrderFromQuoteInput{
		MachineID:          claims.MachineID,
		QuoteID:            quoteID,
		PaymentMethod:      strings.TrimSpace(req.GetPaymentMethod()),
		IdempotencyKey:     wctx.IdempotencyKey,
		Simulated:          simMeta.Simulated,
		SimulationRunID:    simMeta.SimulationRunID,
		SimulationScenario: simMeta.SimulationScenario,
		FakeBill:           simMeta.FakeBill,
		FakeBoard:          simMeta.FakeBoard,
		PricingSnapshot:    orderPricingSnap,
	})
	if err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	if !out.Replay {
		s.auditCommerce(ctx, claims, compliance.ActionMachineCommerceOrderCreated, map[string]any{
			"order_id":        out.OrderID.String(),
			"quote_id":        quoteID.String(),
			"idempotency_key": wctx.IdempotencyKey,
			"vend_line_count": len(out.Lines),
		})
		productionmetrics.RecordOrderCreated("grpc_machine_order_from_quote")
	}
	respLines := make([]*machinev1.OrderVendLineResponse, 0, len(out.Lines))
	for _, l := range out.Lines {
		respLines = append(respLines, &machinev1.OrderVendLineResponse{
			VendSessionId: l.VendSessionID.String(),
			LineSequence:  l.LineSequence,
			SlotIndex:     l.SlotIndex,
			ProductId:     l.ProductID.String(),
			CabinetCode:   l.CabinetCode,
			SlotCode:      l.SlotCode,
			VendState:     l.VendState,
		})
	}
	return &machinev1.CreateOrderFromQuoteResponse{
		Replay:                    out.Replay,
		OrderId:                   out.OrderID.String(),
		OrderStatus:               out.OrderStatus,
		Currency:                  out.Currency,
		SubtotalMinor:             out.SubtotalMinor,
		TaxMinor:                  out.TaxMinor,
		TotalMinor:                out.TotalMinor,
		Lines:                     respLines,
		PricingSource:             out.PricingSource,
		ServerReferenceTotalMinor: out.ServerReferenceTotalMinor,
	}, nil
}
