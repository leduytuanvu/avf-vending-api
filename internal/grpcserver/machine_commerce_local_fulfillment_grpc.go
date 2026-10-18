package grpcserver

import (
	"context"
	"strings"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *machineCommerceServer) CommitLocalFulfillment(ctx context.Context, req *machinev1.CommitLocalFulfillmentRequest) (*machinev1.CommitLocalFulfillmentResponse, error) {
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
	clientOrderID := strings.TrimSpace(req.GetClientOrderId())
	if clientOrderID == "" {
		return nil, status.Error(codes.InvalidArgument, "client_order_id required")
	}
	quoteRaw := strings.TrimSpace(req.GetQuoteId())
	if quoteRaw == "" {
		return nil, status.Error(codes.InvalidArgument, "quote_id required for commit")
	}
	quoteID, err := uuid.Parse(quoteRaw)
	if err != nil || quoteID == uuid.Nil {
		return nil, status.Error(codes.InvalidArgument, "invalid quote_id")
	}
	if req.GetSealedAtMs() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "sealed_at_ms required")
	}
	out, err := svc.CreateOrderFromQuote(ctx, appcommerce.CreateOrderFromQuoteInput{
		MachineID:      claims.MachineID,
		QuoteID:        quoteID,
		PaymentMethod:  "local_fulfillment_commit",
		IdempotencyKey: wctx.IdempotencyKey,
	})
	if err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	if !out.Replay {
		s.auditCommerce(ctx, claims, compliance.ActionMachineCommerceOrderCreated, map[string]any{
			"order_id":         out.OrderID.String(),
			"quote_id":         quoteID.String(),
			"client_order_id":  clientOrderID,
			"idempotency_key":  wctx.IdempotencyKey,
			"terminal_status":  strings.TrimSpace(req.GetTerminalStatus()),
			"terminal_desc":    strings.TrimSpace(req.GetTerminalDescription()),
			"vend_line_count":  len(req.GetLines()),
		})
	}
	return &machinev1.CommitLocalFulfillmentResponse{
		Replay:        out.Replay,
		ServerOrderId: out.OrderID.String(),
		OrderStatus:   out.OrderStatus,
	}, nil
}
