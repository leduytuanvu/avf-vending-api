package grpcserver

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/platform/observability/productionmetrics"
	platformpayments "github.com/avf/avf-vending-api/internal/platform/payments"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type machineCommerceServer struct {
	machinev1.UnimplementedMachineCommerceServiceServer
	deps MachineGRPCServicesDeps
}

func machinePrincipalFromAccessClaims(c plauth.MachineAccessClaims) plauth.Principal {
	return plauth.Principal{
		Subject:        "machine:" + c.MachineID.String(),
		Roles:          []string{plauth.RoleMachine},
		OrganizationID: c.OrganizationID,
		SiteID:         c.SiteID,
		MachineIDs:     []uuid.UUID{c.MachineID},
		JWTType:        plauth.JWTClaimTypeMachine,
	}
}

func (d MachineGRPCServicesDeps) machineOrderCheckoutMaxAge() time.Duration {
	if d.Config == nil {
		return 30 * time.Minute
	}
	age := d.Config.Commerce.MachineOrderCheckoutMaxAge
	if age <= 0 {
		return 30 * time.Minute
	}
	return age
}

func checkMachineOrderCheckoutWindow(o domaincommerce.Order, maxAge time.Duration) error {
	if maxAge <= 0 {
		return nil
	}
	if time.Now().UTC().After(o.CreatedAt.UTC().Add(maxAge)) {
		return status.Error(codes.FailedPrecondition, "order checkout window expired")
	}
	return nil
}

func orderStatusTerminal(st string) bool {
	switch strings.ToLower(strings.TrimSpace(st)) {
	case "completed", "failed", "cancelled":
		return true
	default:
		return false
	}
}

func mapCommercePaymentSessionErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, platformpayments.ErrProviderKeyMismatch):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, platformpayments.ErrSandboxProviderInProduction):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, platformpayments.ErrPaymentProviderRequired):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, platformpayments.ErrUnknownProvider):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, platformpayments.ErrLiveProviderNotWired):
		return status.Error(codes.Unavailable, err.Error())
	case errors.Is(err, platformpayments.ErrInvalidCardSessionProvider):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return mapCommerceGRPCErr(err)
	}
}

func mapCommerceGRPCErr(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := status.FromError(err); ok {
		return err
	}
	switch {
	case errors.Is(err, appcommerce.ErrInvalidArgument):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, appcommerce.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, appcommerce.ErrOrgMismatch):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, appcommerce.ErrIllegalTransition):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, appcommerce.ErrPaymentNotSettled):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, appcommerce.ErrCancelNotAllowed):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, appcommerce.ErrNotConfigured):
		return status.Error(codes.Unavailable, err.Error())
	case errors.Is(err, appcommerce.ErrIdempotencyPayloadConflict):
		return status.Error(codes.Aborted, err.Error())
	default:
		if strings.Contains(err.Error(), "insufficient stock") {
			return status.Error(codes.ResourceExhausted, err.Error())
		}
		return status.Error(codes.Internal, err.Error())
	}
}

func (s *machineCommerceServer) requireCommerce(ctx context.Context) (plauth.MachineAccessClaims, appcommerce.Orchestrator, *postgres.Store, error) {
	if s.deps.Commerce == nil || s.deps.TelemetryStore == nil {
		return plauth.MachineAccessClaims{}, nil, nil, status.Error(codes.Unavailable, "commerce not configured")
	}
	claims, ok := plauth.MachineAccessClaimsFromContext(ctx)
	if !ok {
		return plauth.MachineAccessClaims{}, nil, nil, status.Error(codes.Unauthenticated, "missing machine credentials")
	}
	q := db.New(s.deps.Pool)
	if err := machineRuntimeInventoryGate(ctx, q, claims); err != nil {
		return plauth.MachineAccessClaims{}, nil, nil, err
	}
	return claims, s.deps.Commerce, s.deps.TelemetryStore, nil
}

func (s *machineCommerceServer) auditCommerce(ctx context.Context, claims plauth.MachineAccessClaims, action string, meta map[string]any) {
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
		ResourceType:   "commerce.order",
		ResourceID:     ptrMetaOrderID(meta),
		Metadata:       md,
	})
}

func ptrMetaOrderID(meta map[string]any) *string {
	if meta == nil {
		return nil
	}
	v, ok := meta["order_id"].(string)
	if !ok || strings.TrimSpace(v) == "" {
		return nil
	}
	return &v
}

func parseSlotProto(sel *machinev1.SlotSelection) (slotID *uuid.UUID, cab, slot string, slotIdx *int32, err error) {
	if sel == nil {
		return nil, "", "", nil, status.Error(codes.InvalidArgument, "slot selection required")
	}
	cab = strings.TrimSpace(sel.GetCabinetCode())
	slot = strings.TrimSpace(sel.GetSlotCode())
	if sid := strings.TrimSpace(sel.GetSlotId()); sid != "" {
		u, perr := uuid.Parse(sid)
		if perr != nil || u == uuid.Nil {
			return nil, "", "", nil, status.Error(codes.InvalidArgument, "invalid slot_id")
		}
		slotID = &u
	}
	if sel.SlotIndex != nil {
		i := *sel.SlotIndex
		if i < 0 {
			return nil, "", "", nil, status.Error(codes.InvalidArgument, "slot_index must be non-negative")
		}
		slotIdx = &i
	}
	hasSlotID := slotID != nil
	hasCodes := cab != "" && slot != ""
	hasIdx := slotIdx != nil
	if !hasSlotID && !hasCodes && !hasIdx {
		return nil, "", "", nil, status.Error(codes.InvalidArgument, "set slot_id, cabinet_code+slot_code, or slot_index")
	}
	return slotID, cab, slot, slotIdx, nil
}

func validateReplayCreateOrder(claims plauth.MachineAccessClaims, productID uuid.UUID, slotID *uuid.UUID, cab, slot string, slotIdx *int32, out appcommerce.CreateOrderResult) error {
	if out.Order.MachineID != claims.MachineID {
		return appcommerce.ErrIdempotencyPayloadConflict
	}
	if out.Vend.ProductID != productID {
		return appcommerce.ErrIdempotencyPayloadConflict
	}
	switch {
	case slotID != nil:
		if out.SaleLine.SlotConfigID != *slotID {
			return appcommerce.ErrIdempotencyPayloadConflict
		}
	case cab != "" && slot != "":
		if !strings.EqualFold(strings.TrimSpace(out.SaleLine.CabinetCode), cab) || !strings.EqualFold(strings.TrimSpace(out.SaleLine.SlotCode), slot) {
			return appcommerce.ErrIdempotencyPayloadConflict
		}
	case slotIdx != nil:
		if out.SaleLine.SlotIndex != *slotIdx {
			return appcommerce.ErrIdempotencyPayloadConflict
		}
	}
	return nil
}

func (s *machineCommerceServer) CreateOrder(ctx context.Context, req *machinev1.CreateOrderRequest) (*machinev1.CreateOrderResponse, error) {
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
	productID, perr := uuid.Parse(strings.TrimSpace(req.GetProductId()))
	if perr != nil || productID == uuid.Nil {
		return nil, status.Error(codes.InvalidArgument, "invalid product_id")
	}
	slotID, cab, sc, slotIdx, err := parseSlotProto(req.GetSlot())
	if err != nil {
		return nil, err
	}
	cur := strings.ToUpper(strings.TrimSpace(req.GetCurrency()))
	if len(cur) != 3 {
		return nil, status.Error(codes.InvalidArgument, "currency must be a 3-letter ISO code")
	}

	out, err := svc.CreateOrder(ctx, appcommerce.CreateOrderInput{
		OrganizationID: claims.OrganizationID,
		MachineID:      claims.MachineID,
		ProductID:      productID,
		SlotID:         slotID,
		CabinetCode:    cab,
		SlotCode:       sc,
		SlotIndex:      slotIdx,
		Currency:       cur,
		IdempotencyKey: wctx.IdempotencyKey,
	})
	if err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	if out.Replay {
		if err := validateReplayCreateOrder(claims, productID, slotID, cab, sc, slotIdx, out); err != nil {
			return nil, mapCommerceGRPCErr(err)
		}
	} else {
		s.auditCommerce(ctx, claims, compliance.ActionMachineCommerceOrderCreated, map[string]any{
			"order_id":        out.Order.ID.String(),
			"vend_session_id": out.Vend.ID.String(),
			"idempotency_key": wctx.IdempotencyKey,
			"client_event_id": wctx.ClientEventID,
			"product_id":      productID.String(),
		})
		productionmetrics.RecordOrderCreated("grpc_machine")
	}

	sid := ""
	if out.SaleLine.SlotConfigID != uuid.Nil {
		sid = out.SaleLine.SlotConfigID.String()
	}
	return &machinev1.CreateOrderResponse{
		Replay:        out.Replay,
		OrderId:       out.Order.ID.String(),
		VendSessionId: out.Vend.ID.String(),
		OrderStatus:   out.Order.Status,
		VendState:     out.Vend.State,
		SlotId:        sid,
		CabinetCode:   out.SaleLine.CabinetCode,
		SlotCode:      out.SaleLine.SlotCode,
		SlotIndex:     out.SaleLine.SlotIndex,
		SubtotalMinor: out.Order.SubtotalMinor,
		TaxMinor:      out.Order.TaxMinor,
		TotalMinor:    out.Order.TotalMinor,
		PriceMinor:    out.SaleLine.PriceMinor,
	}, nil
}

func (s *machineCommerceServer) CreatePaymentSession(ctx context.Context, req *machinev1.CreatePaymentSessionRequest) (*machinev1.CreatePaymentSessionResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	wctx, err := parseMachineMutationContext(ctx, req.GetContext())
	if err != nil {
		return nil, err
	}
	claims, svc, store, err := s.requireCommerce(ctx)
	if err != nil {
		return nil, err
	}
	orderID, err := uuid.Parse(strings.TrimSpace(req.GetOrderId()))
	if err != nil || orderID == uuid.Nil {
		return nil, status.Error(codes.InvalidArgument, "invalid order_id")
	}
	principal := machinePrincipalFromAccessClaims(claims)
	if err := svc.EnsureCommerceCallerOrderAccess(ctx, claims.OrganizationID, orderID, principal); err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	o, err := store.GetOrderByID(ctx, orderID)
	if err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	if err := checkMachineOrderCheckoutWindow(o, s.deps.machineOrderCheckoutMaxAge()); err != nil {
		return nil, err
	}
	if orderStatusTerminal(o.Status) {
		return nil, status.Error(codes.FailedPrecondition, "order is terminal")
	}
	if o.MachineID != claims.MachineID {
		return nil, status.Error(codes.PermissionDenied, "order machine mismatch")
	}

	topic := strings.TrimSpace(s.deps.Config.Commerce.PaymentOutboxTopic)
	evType := strings.TrimSpace(s.deps.Config.Commerce.PaymentOutboxEventType)
	aggType := strings.TrimSpace(s.deps.Config.Commerce.PaymentOutboxAggregateType)
	if topic == "" || evType == "" || aggType == "" {
		return nil, status.Error(codes.Unavailable, "commerce outbox not configured")
	}

	payState := strings.TrimSpace(req.GetPaymentState())
	amt := req.GetAmountMinor()
	cur := strings.ToUpper(strings.TrimSpace(req.GetCurrency()))
	if cur == "" || len(cur) != 3 {
		return nil, status.Error(codes.InvalidArgument, "currency must be a 3-letter ISO code")
	}
	if amt != o.TotalMinor {
		return nil, status.Error(codes.InvalidArgument, "amount_minor must match order total")
	}
	if strings.ToUpper(strings.TrimSpace(o.Currency)) != cur {
		return nil, status.Error(codes.InvalidArgument, "currency must match order")
	}
	_ = payState // vending clients cannot choose non-created PSP states; validated again in app layer

	res, err := svc.CreateMachinePaymentSession(ctx, appcommerce.CreateMachinePaymentSessionInput{
		OrganizationID:  claims.OrganizationID,
		OrderID:         orderID,
		MachineID:       claims.MachineID,
		IdempotencyKey:  wctx.IdempotencyKey,
		ClientProvider:  strings.TrimSpace(req.GetProvider()),
		ClientPayState:  strings.TrimSpace(req.GetPaymentState()),
		AmountMinor:     amt,
		Currency:        cur,
		AppEnv:          s.deps.Config.AppEnv,
		OutboxTopic:     topic,
		OutboxEventType: evType,
		OutboxAggregate: aggType,
	})
	if err != nil {
		return nil, mapCommercePaymentSessionErr(err)
	}
	if !res.Replay {
		s.auditCommerce(ctx, claims, compliance.ActionMachineCommercePaymentSessionCreated, map[string]any{
			"order_id":        orderID.String(),
			"payment_id":      res.Payment.ID.String(),
			"idempotency_key": wctx.IdempotencyKey,
			"provider":        res.ProviderKey,
			"payment_state":   "created",
		})
	}

	return &machinev1.CreatePaymentSessionResponse{
		Replay:         res.Replay,
		PaymentId:      res.Payment.ID.String(),
		PaymentState:   res.Payment.State,
		OutboxEventId:  res.Outbox.ID,
		QrPayloadOrUrl: res.QRPayloadOrURL,
	}, nil
}

func (s *machineCommerceServer) AttachPaymentResult(ctx context.Context, req *machinev1.CreatePaymentSessionRequest) (*machinev1.CreatePaymentSessionResponse, error) {
	return s.CreatePaymentSession(ctx, req)
}

func (s *machineCommerceServer) ConfirmCashPayment(ctx context.Context, req *machinev1.ConfirmCashPaymentRequest) (*machinev1.ConfirmCashPaymentResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	wctx, err := parseMachineMutationContext(ctx, req.GetContext())
	if err != nil {
		return nil, err
	}
	claims, svc, store, err := s.requireCommerce(ctx)
	if err != nil {
		return nil, err
	}
	orderID, err := uuid.Parse(strings.TrimSpace(req.GetOrderId()))
	if err != nil || orderID == uuid.Nil {
		return nil, status.Error(codes.InvalidArgument, "invalid order_id")
	}
	principal := machinePrincipalFromAccessClaims(claims)
	if err := svc.EnsureCommerceCallerOrderAccess(ctx, claims.OrganizationID, orderID, principal); err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	o, err := store.GetOrderByID(ctx, orderID)
	if err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	if err := checkMachineOrderCheckoutWindow(o, s.deps.machineOrderCheckoutMaxAge()); err != nil {
		return nil, err
	}
	if orderStatusTerminal(o.Status) {
		return nil, status.Error(codes.FailedPrecondition, "order is terminal")
	}
	if o.MachineID != claims.MachineID {
		return nil, status.Error(codes.PermissionDenied, "order machine mismatch")
	}

	topic := strings.TrimSpace(s.deps.Config.Commerce.PaymentOutboxTopic)
	evType := strings.TrimSpace(s.deps.Config.Commerce.PaymentOutboxEventType)
	aggType := strings.TrimSpace(s.deps.Config.Commerce.PaymentOutboxAggregateType)
	if topic == "" || evType == "" || aggType == "" {
		return nil, status.Error(codes.Unavailable, "commerce outbox not configured")
	}

	idem := wctx.IdempotencyKey
	payKey := idem + ":cash:payment"
	outboxIdem := idem + ":cash:payment:outbox:" + orderID.String()

	payRes, err := svc.StartPaymentWithOutbox(ctx, appcommerce.StartPaymentInput{
		OrganizationID:       claims.OrganizationID,
		OrderID:              orderID,
		Provider:             "cash",
		PaymentState:         "captured",
		AmountMinor:          o.TotalMinor,
		Currency:             o.Currency,
		IdempotencyKey:       payKey,
		OutboxTopic:          topic,
		OutboxEventType:      evType,
		OutboxPayload:        []byte(`{"source":"machine_grpc_cash"}`),
		OutboxAggregateType:  aggType,
		OutboxAggregateID:    orderID,
		OutboxIdempotencyKey: outboxIdem,
	})
	if err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	if payRes.Replay {
		if payRes.Payment.AmountMinor != o.TotalMinor ||
			strings.ToUpper(strings.TrimSpace(payRes.Payment.Currency)) != strings.ToUpper(strings.TrimSpace(o.Currency)) ||
			payRes.Payment.State != "captured" {
			return nil, mapCommerceGRPCErr(appcommerce.ErrIdempotencyPayloadConflict)
		}
	}

	paid, err := svc.MarkOrderPaidAfterPaymentCapture(ctx, claims.OrganizationID, orderID)
	if err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	replay := payRes.Replay
	if !payRes.Replay {
		s.auditCommerce(ctx, claims, compliance.ActionMachineCommerceCashPaymentConfirmed, map[string]any{
			"order_id":        orderID.String(),
			"payment_id":      payRes.Payment.ID.String(),
			"idempotency_key": idem,
		})
	}
	return &machinev1.ConfirmCashPaymentResponse{
		Replay:       replay,
		PaymentId:    payRes.Payment.ID.String(),
		OrderStatus:  paid.Status,
		PaymentState: payRes.Payment.State,
	}, nil
}

func (s *machineCommerceServer) CreateCashCheckout(ctx context.Context, req *machinev1.ConfirmCashPaymentRequest) (*machinev1.ConfirmCashPaymentResponse, error) {
	return s.ConfirmCashPayment(ctx, req)
}

func (s *machineCommerceServer) getStatus(ctx context.Context, claims plauth.MachineAccessClaims, svc appcommerce.Orchestrator, orderID uuid.UUID, slotIndex int32) (appcommerce.CheckoutStatusView, error) {
	principal := machinePrincipalFromAccessClaims(claims)
	if err := svc.EnsureCommerceCallerOrderAccess(ctx, claims.OrganizationID, orderID, principal); err != nil {
		return appcommerce.CheckoutStatusView{}, err
	}
	return svc.GetCheckoutStatus(ctx, claims.OrganizationID, orderID, slotIndex)
}

func (s *machineCommerceServer) GetOrder(ctx context.Context, req *machinev1.GetOrderRequest) (*machinev1.GetOrderResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	claims, svc, _, err := s.requireCommerce(ctx)
	if err != nil {
		return nil, err
	}
	orderID, err := uuid.Parse(strings.TrimSpace(req.GetOrderId()))
	if err != nil || orderID == uuid.Nil {
		return nil, status.Error(codes.InvalidArgument, "invalid order_id")
	}
	st, err := s.getStatus(ctx, claims, svc, orderID, req.GetSlotIndex())
	if err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	return checkoutViewToGetOrderResponse(st), nil
}

func checkoutViewToGetOrderResponse(st appcommerce.CheckoutStatusView) *machinev1.GetOrderResponse {
	out := &machinev1.GetOrderResponse{
		OrderId:        st.Order.ID.String(),
		OrderStatus:    st.Order.Status,
		Currency:       st.Order.Currency,
		SubtotalMinor:  st.Order.SubtotalMinor,
		TaxMinor:       st.Order.TaxMinor,
		TotalMinor:     st.Order.TotalMinor,
		OrderCreatedAt: timestamppb.New(st.Order.CreatedAt.UTC()),
		VendState:      st.Vend.State,
		VendSlotIndex:  st.Vend.SlotIndex,
		ProductId:      st.Vend.ProductID.String(),
		PaymentPresent: st.PaymentPresent,
	}
	if st.PaymentPresent {
		out.PaymentId = st.Payment.ID.String()
		out.PaymentProvider = st.Payment.Provider
		out.PaymentState = st.Payment.State
	}
	return out
}

func (s *machineCommerceServer) GetOrderStatus(ctx context.Context, req *machinev1.GetOrderStatusRequest) (*machinev1.GetOrderStatusResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	claims, svc, _, err := s.requireCommerce(ctx)
	if err != nil {
		return nil, err
	}
	orderID, err := uuid.Parse(strings.TrimSpace(req.GetOrderId()))
	if err != nil || orderID == uuid.Nil {
		return nil, status.Error(codes.InvalidArgument, "invalid order_id")
	}
	st, err := s.getStatus(ctx, claims, svc, orderID, req.GetSlotIndex())
	if err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	resp := &machinev1.GetOrderStatusResponse{
		OrderId:        st.Order.ID.String(),
		OrderStatus:    st.Order.Status,
		VendState:      st.Vend.State,
		PaymentPresent: st.PaymentPresent,
	}
	if st.PaymentPresent {
		resp.PaymentState = st.Payment.State
	}
	return resp, nil
}

func (s *machineCommerceServer) StartVend(ctx context.Context, req *machinev1.StartVendRequest) (*machinev1.StartVendResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	wctx, err := parseMachineMutationContext(ctx, req.GetContext())
	if err != nil {
		return nil, err
	}
	claims, svc, store, err := s.requireCommerce(ctx)
	if err != nil {
		return nil, err
	}
	orderID, err := uuid.Parse(strings.TrimSpace(req.GetOrderId()))
	if err != nil || orderID == uuid.Nil {
		return nil, status.Error(codes.InvalidArgument, "invalid order_id")
	}
	slotIndex := req.GetSlotIndex()
	principal := machinePrincipalFromAccessClaims(claims)
	if err := svc.EnsureCommerceCallerOrderAccess(ctx, claims.OrganizationID, orderID, principal); err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	o, err := store.GetOrderByID(ctx, orderID)
	if err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	if err := checkMachineOrderCheckoutWindow(o, s.deps.machineOrderCheckoutMaxAge()); err != nil {
		return nil, err
	}
	if orderStatusTerminal(o.Status) {
		return nil, status.Error(codes.FailedPrecondition, "order is terminal")
	}

	st, err := svc.GetCheckoutStatus(ctx, claims.OrganizationID, orderID, slotIndex)
	if err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	if !st.PaymentPresent || st.Payment.State != "captured" {
		return nil, status.Error(codes.FailedPrecondition, "payment not captured")
	}
	if st.Order.Status != "paid" && st.Order.Status != "vending" {
		return nil, status.Error(codes.FailedPrecondition, "order not paid")
	}
	if st.Vend.State == "in_progress" {
		return &machinev1.StartVendResponse{Replay: true, VendState: "in_progress", SlotIndex: slotIndex}, nil
	}
	if st.Vend.State != "pending" {
		return nil, status.Error(codes.FailedPrecondition, "vend not startable")
	}

	v, err := svc.AdvanceVend(ctx, appcommerce.AdvanceVendInput{
		OrganizationID: claims.OrganizationID,
		OrderID:        orderID,
		SlotIndex:      slotIndex,
		ToState:        "in_progress",
	})
	if err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	s.auditCommerce(ctx, claims, compliance.ActionMachineCommerceVendStarted, map[string]any{
		"order_id":        orderID.String(),
		"slot_index":      slotIndex,
		"idempotency_key": wctx.IdempotencyKey,
	})
	return &machinev1.StartVendResponse{Replay: false, VendState: v.State, SlotIndex: v.SlotIndex}, nil
}

func (s *machineCommerceServer) ReportVendSuccess(ctx context.Context, req *machinev1.ReportVendSuccessRequest) (*machinev1.ReportVendSuccessResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	wctx, err := parseMachineMutationContext(ctx, req.GetContext())
	if err != nil {
		return nil, err
	}
	claims, svc, store, err := s.requireCommerce(ctx)
	if err != nil {
		return nil, err
	}
	orderID, err := uuid.Parse(strings.TrimSpace(req.GetOrderId()))
	if err != nil || orderID == uuid.Nil {
		return nil, status.Error(codes.InvalidArgument, "invalid order_id")
	}
	slotIndex := req.GetSlotIndex()
	var corr *uuid.UUID
	if cid := strings.TrimSpace(req.GetCorrelationId()); cid != "" {
		u, perr := uuid.Parse(cid)
		if perr != nil || u == uuid.Nil {
			return nil, status.Error(codes.InvalidArgument, "invalid correlation_id")
		}
		corr = &u
	}

	return s.confirmVendSuccess(ctx, claims, svc, store, wctx, orderID, slotIndex, corr)
}

func (s *machineCommerceServer) ConfirmVendSuccess(ctx context.Context, req *machinev1.ConfirmVendSuccessRequest) (*machinev1.ConfirmVendSuccessResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	wctx, err := parseMachineMutationContext(ctx, req.GetContext())
	if err != nil {
		return nil, err
	}
	claims, svc, store, err := s.requireCommerce(ctx)
	if err != nil {
		return nil, err
	}
	orderID, err := uuid.Parse(strings.TrimSpace(req.GetOrderId()))
	if err != nil || orderID == uuid.Nil {
		return nil, status.Error(codes.InvalidArgument, "invalid order_id")
	}
	slotIndex := req.GetSlotIndex()
	var corr *uuid.UUID
	if cid := strings.TrimSpace(req.GetCorrelationId()); cid != "" {
		u, perr := uuid.Parse(cid)
		if perr != nil || u == uuid.Nil {
			return nil, status.Error(codes.InvalidArgument, "invalid correlation_id")
		}
		corr = &u
	}

	out, err := s.confirmVendSuccess(ctx, claims, svc, store, wctx, orderID, slotIndex, corr)
	if err != nil {
		return nil, err
	}
	return &machinev1.ConfirmVendSuccessResponse{
		Replay:          out.GetReplay(),
		InventoryReplay: out.GetInventoryReplay(),
		OrderId:         out.GetOrderId(),
		OrderStatus:     out.GetOrderStatus(),
		VendState:       out.GetVendState(),
	}, nil
}

func (s *machineCommerceServer) confirmVendSuccess(ctx context.Context, claims plauth.MachineAccessClaims, svc appcommerce.Orchestrator, store *postgres.Store, wctx machineMutationContext, orderID uuid.UUID, slotIndex int32, corr *uuid.UUID) (*machinev1.ReportVendSuccessResponse, error) {
	principal := machinePrincipalFromAccessClaims(claims)
	if err := svc.EnsureCommerceCallerOrderAccess(ctx, claims.OrganizationID, orderID, principal); err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	o, err := store.GetOrderByID(ctx, orderID)
	if err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	if o.MachineID != claims.MachineID {
		return nil, status.Error(codes.PermissionDenied, "order machine mismatch")
	}
	if err := checkMachineOrderCheckoutWindow(o, s.deps.machineOrderCheckoutMaxAge()); err != nil {
		return nil, err
	}

	if corr != nil {
		_ = store.TouchVendSessionCorrelation(ctx, orderID, slotIndex, *corr)
	}
	if _, err := svc.GetCheckoutStatus(ctx, claims.OrganizationID, orderID, slotIndex); err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	if err := svc.EnsureVendInProgressForPaidOrder(ctx, claims.OrganizationID, orderID, slotIndex); err != nil {
		return nil, mapCommerceGRPCErr(err)
	}

	fout, err := svc.FinalizeOrderAfterVend(ctx, appcommerce.FinalizeAfterVendInput{
		OrganizationID:            claims.OrganizationID,
		OrderID:                   orderID,
		SlotIndex:                 slotIndex,
		TerminalVendState:         "success",
		FailureReason:             nil,
		ClientWriteIdempotencyKey: strings.TrimSpace(wctx.IdempotencyKey),
		CorrelationID:             corr,
	})
	if err != nil {
		return nil, mapCommerceGRPCErr(err)
	}

	fullReplay := fout.OrderVendReplay && fout.InventoryReplay

	if !fullReplay {
		s.auditCommerce(ctx, claims, compliance.ActionMachineCommerceVendSuccess, map[string]any{
			"order_id":         orderID.String(),
			"idempotency_key":  wctx.IdempotencyKey,
			"inventory_replay": fout.InventoryReplay,
			"finalize_replay":  fout.OrderVendReplay,
		})
	}

	return &machinev1.ReportVendSuccessResponse{
		Replay:          fullReplay,
		InventoryReplay: fout.InventoryReplay,
		OrderId:         fout.Order.ID.String(),
		OrderStatus:     fout.Order.Status,
		VendState:       fout.Vend.State,
	}, nil
}

func (s *machineCommerceServer) ReportVendFailure(ctx context.Context, req *machinev1.ReportVendFailureRequest) (*machinev1.ReportVendFailureResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	wctx, err := parseMachineMutationContext(ctx, req.GetContext())
	if err != nil {
		return nil, err
	}
	claims, svc, store, err := s.requireCommerce(ctx)
	if err != nil {
		return nil, err
	}
	orderID, err := uuid.Parse(strings.TrimSpace(req.GetOrderId()))
	if err != nil || orderID == uuid.Nil {
		return nil, status.Error(codes.InvalidArgument, "invalid order_id")
	}
	slotIndex := req.GetSlotIndex()
	reason := strings.TrimSpace(req.GetFailureReason())
	var corr *uuid.UUID
	if cid := strings.TrimSpace(req.GetCorrelationId()); cid != "" {
		u, perr := uuid.Parse(cid)
		if perr != nil || u == uuid.Nil {
			return nil, status.Error(codes.InvalidArgument, "invalid correlation_id")
		}
		corr = &u
	}

	principal := machinePrincipalFromAccessClaims(claims)
	if err := svc.EnsureCommerceCallerOrderAccess(ctx, claims.OrganizationID, orderID, principal); err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	o, err := store.GetOrderByID(ctx, orderID)
	if err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	if o.MachineID != claims.MachineID {
		return nil, status.Error(codes.PermissionDenied, "order machine mismatch")
	}
	if err := checkMachineOrderCheckoutWindow(o, s.deps.machineOrderCheckoutMaxAge()); err != nil {
		return nil, err
	}

	if corr != nil {
		_ = store.TouchVendSessionCorrelation(ctx, orderID, slotIndex, *corr)
	}
	st, err := svc.GetCheckoutStatus(ctx, claims.OrganizationID, orderID, slotIndex)
	if err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	if err := svc.EnsureVendInProgressForPaidOrder(ctx, claims.OrganizationID, orderID, slotIndex); err != nil {
		return nil, mapCommerceGRPCErr(err)
	}

	vendReplay := st.Vend.State == "failed"
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}
	fout, err := svc.FinalizeOrderAfterVend(ctx, appcommerce.FinalizeAfterVendInput{
		OrganizationID:    claims.OrganizationID,
		OrderID:           orderID,
		SlotIndex:         slotIndex,
		TerminalVendState: "failed",
		FailureReason:     reasonPtr,
	})
	if err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	vendReplay = vendReplay || fout.OrderVendReplay

	st2, _ := svc.GetCheckoutStatus(ctx, claims.OrganizationID, orderID, slotIndex)
	resp := &machinev1.ReportVendFailureResponse{
		Replay:      vendReplay,
		OrderId:     fout.Order.ID.String(),
		OrderStatus: fout.Order.Status,
		VendState:   fout.Vend.State,
	}
	if st2.PaymentPresent && strings.EqualFold(st2.Payment.Provider, "cash") && st2.Payment.State == "captured" {
		resp.LocalCashRefundRequired = true
	} else if st2.PaymentPresent && st2.Payment.State == "captured" {
		resp.RefundRequired = true
	}

	if !vendReplay {
		s.auditCommerce(ctx, claims, compliance.ActionMachineCommerceVendFailure, map[string]any{
			"order_id":        orderID.String(),
			"idempotency_key": wctx.IdempotencyKey,
			"failure_reason":  reason,
		})
	}
	return resp, nil
}

func (s *machineCommerceServer) CancelOrder(ctx context.Context, req *machinev1.CancelOrderRequest) (*machinev1.CancelOrderResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	wctx, err := parseMachineMutationContext(ctx, req.GetContext())
	if err != nil {
		return nil, err
	}
	claims, svc, store, err := s.requireCommerce(ctx)
	if err != nil {
		return nil, err
	}
	orderID, err := uuid.Parse(strings.TrimSpace(req.GetOrderId()))
	if err != nil || orderID == uuid.Nil {
		return nil, status.Error(codes.InvalidArgument, "invalid order_id")
	}
	principal := machinePrincipalFromAccessClaims(claims)
	if err := svc.EnsureCommerceCallerOrderAccess(ctx, claims.OrganizationID, orderID, principal); err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	o, err := store.GetOrderByID(ctx, orderID)
	if err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	if o.MachineID != claims.MachineID {
		return nil, status.Error(codes.PermissionDenied, "order machine mismatch")
	}
	if strings.EqualFold(strings.TrimSpace(o.Status), "cancelled") {
		return &machinev1.CancelOrderResponse{Replay: true, OrderId: o.ID.String(), OrderStatus: o.Status}, nil
	}
	if orderStatusTerminal(o.Status) {
		return nil, status.Error(codes.FailedPrecondition, "order is terminal")
	}

	reason := strings.TrimSpace(req.GetReason())
	o2, err := svc.CancelOrder(ctx, claims.OrganizationID, orderID, reason)
	if err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	s.auditCommerce(ctx, claims, compliance.ActionMachineCommerceOrderCancelled, map[string]any{
		"order_id":        orderID.String(),
		"idempotency_key": wctx.IdempotencyKey,
		"reason":          reason,
	})
	return &machinev1.CancelOrderResponse{Replay: false, OrderId: o2.ID.String(), OrderStatus: o2.Status}, nil
}
