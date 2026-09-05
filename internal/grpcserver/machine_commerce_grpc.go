package grpcserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/observability"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/platform/observability/productionmetrics"
	platformpayments "github.com/avf/avf-vending-api/internal/platform/payments"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type machineCommerceServer struct {
	machinev1.UnimplementedMachineCommerceServiceServer
	deps MachineGRPCServicesDeps
}

func (s *machineCommerceServer) machineFeatureFlags(ctx context.Context, machineID uuid.UUID) map[string]bool {
	if s.deps.FeatureFlags == nil || machineID == uuid.Nil {
		return nil
	}
	rh, err := s.deps.FeatureFlags.RuntimeHintsForMachine(ctx, machineID)
	if err != nil || rh == nil {
		return nil
	}
	return rh.FeatureFlags
}

func (s *machineCommerceServer) machinePaymentMethods(ctx context.Context, machineID uuid.UUID) platformpayments.MachinePaymentMethodsView {
	flags := s.machineFeatureFlags(ctx, machineID)
	if s.deps.PaymentRuntime != nil {
		return s.deps.PaymentRuntime.ResolveMachinePaymentMethodsForMachine(s.deps.Config, flags)
	}
	return platformpayments.ResolveMachinePaymentMethods(s.deps.Config, nil, flags)
}

func mapPaymentMethodsProto(m platformpayments.MachinePaymentMethodsView) *machinev1.PaymentMethodsConfig {
	if m.PaymentMode == "" {
		return nil
	}
	providers := make([]*machinev1.PaymentProviderCapability, 0, len(m.Providers))
	for _, p := range m.Providers {
		providers = append(providers, &machinev1.PaymentProviderCapability{
			Key:               p.Key,
			Enabled:           p.Enabled,
			Status:            p.Status,
			Ready:             p.Ready,
			SessionCreatable:  p.SessionCreatable,
			UnavailableReason: p.UnavailableReason,
		})
	}
	return &machinev1.PaymentMethodsConfig{
		CashEnabled:             m.CashEnabled,
		QrCardEnabled:           m.QRCardEnabled,
		PaymentMode:             m.PaymentMode,
		CardQrProviderKey:       m.CardQRProviderKey,
		CardQrProviderStatus:    m.CardQRProviderStatus,
		QrCardUnavailableReason: m.QRCardUnavailableReason,
		Providers:               providers,
	}
}

func machineExternalCode(ctx context.Context, deps MachineGRPCServicesDeps, machineID uuid.UUID) string {
	if deps.Pool == nil || machineID == uuid.Nil {
		return ""
	}
	m, err := db.New(deps.Pool).GetMachineByID(ctx, machineID)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(m.Code)
}

func machinePrincipalFromAccessClaims(c plauth.MachineAccessClaims) plauth.Principal {
	return plauth.Principal{
		Subject:    "machine:" + c.MachineID.String(),
		Roles:      []string{plauth.RoleMachine},
		SiteID:     c.SiteID,
		MachineIDs: []uuid.UUID{c.MachineID},
		JWTType:    plauth.JWTClaimTypeMachine,
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
		return status.Error(codes.FailedPrecondition, "provider_unavailable")
	case errors.Is(err, platformpayments.ErrProviderUnavailable):
		return status.Error(codes.FailedPrecondition, "provider_unavailable")
	case errors.Is(err, platformpayments.ErrInvalidCardSessionProvider):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		if mapped := mapCommercePersistenceErr(err); mapped != nil {
			return mapped
		}
		if msg := err.Error(); strings.Contains(msg, "momo create failed") || strings.Contains(msg, "empty provider_reference") {
			return status.Error(codes.FailedPrecondition, "provider_rejected")
		}
		if strings.Contains(err.Error(), "momo create:") || strings.Contains(err.Error(), "timeout") {
			return status.Error(codes.Unavailable, "provider_timeout")
		}
		return mapCommerceGRPCErr(err)
	}
}

func mapCommercePersistenceErr(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return nil
	}
	switch pgErr.Code {
	case "22P02", "22023":
		return status.Error(codes.Internal, "payment_session_persistence_failed")
	case "23505":
		return status.Error(codes.FailedPrecondition, "payment_conflict")
	default:
		if strings.HasPrefix(pgErr.Code, "08") || pgErr.Code == "57P03" {
			return status.Error(codes.Unavailable, "payment_backend_unavailable")
		}
	}
	return nil
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
	case errors.Is(err, domaincommerce.ErrVendEvidenceRequired):
		return status.Error(codes.FailedPrecondition, vendHardwareEvidenceRequiredMsg)
	case errors.Is(err, domaincommerce.ErrVendEvidenceInvalid):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		if mapped := mapCommercePersistenceErr(err); mapped != nil {
			return mapped
		}
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
		ActorType:    compliance.ActorMachine,
		ActorID:      &mid,
		Action:       action,
		ResourceType: "commerce.order",
		ResourceID:   ptrMetaOrderID(meta),
		Metadata:     md,
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

func validateReplayCreateOrder(claims plauth.MachineAccessClaims, productID uuid.UUID, slotID *uuid.UUID, cab, slot string, slotIdx *int32, pricingSnapshot *appcommerce.MachinePricingSnapshotInput, out appcommerce.CreateOrderResult) error {
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
	if pricingSnapshot != nil {
		if err := appcommerce.ValidateReplayPricingSnapshot(pricingSnapshot, out.Order); err != nil {
			return err
		}
	}
	return nil
}

func (s *machineCommerceServer) CreateOrder(ctx context.Context, req *machinev1.CreateOrderRequest) (*machinev1.CreateOrderResponse, error) {
	trace := newCreateOrderTrace(zap.L())
	trace.checkpoint(ctx, "create_order.enter")
	ctx, cancel := withCreateOrderDeadline(ctx)
	defer cancel()

	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	wctx, err := parseMachineMutationContext(ctx, req.GetContext())
	if err != nil {
		return nil, err
	}
	trace.checkpoint(ctx, "parse_context.done", zap.String("idempotency_key_hash", idempotencyKeyHash(wctx.IdempotencyKey)))
	claims, svc, _, err := s.requireCommerce(ctx)
	if err != nil {
		return nil, err
	}
	trace.checkpoint(ctx, "require_commerce.done", zap.String("machine_id", claims.MachineID.String()))
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
	trace.checkpoint(ctx, "product_parse.done", zap.String("product_id", productID.String()))
	slotID, cab, sc, slotIdx, err := parseSlotProto(req.GetSlot())
	if err != nil {
		return nil, err
	}
	trace.checkpoint(ctx, "slot_parse.done", zap.Int32("slot_index", slotIdxValue(slotIdx)))
	cur := strings.ToUpper(strings.TrimSpace(req.GetCurrency()))
	if len(cur) != 3 {
		return nil, status.Error(codes.InvalidArgument, "currency must be a 3-letter ISO code")
	}

	simMeta := parseSimulationContext(req.GetSimulation())
	if err := validateSimulationCommerce(claims.MachineID, simMeta, s.deps.Config.AppEnv); err != nil {
		return nil, err
	}
	trace.checkpoint(ctx, "simulation_validate.done")

	var pricingSnapshot *appcommerce.MachinePricingSnapshotInput
	if req.GetPricingSnapshot() != nil {
		snap, err := appcommerce.MachinePricingSnapshotFromProto(req.GetPricingSnapshot())
		if err != nil {
			return nil, mapCommerceGRPCErr(err)
		}
		pricingSnapshot = &snap
	}

	trace.checkpoint(ctx, "service_create_order.start")
	out, err := svc.CreateOrder(ctx, appcommerce.CreateOrderInput{
		MachineID:          claims.MachineID,
		ProductID:          productID,
		SlotID:             slotID,
		CabinetCode:        cab,
		SlotCode:           sc,
		SlotIndex:          slotIdx,
		Currency:           cur,
		IdempotencyKey:     wctx.IdempotencyKey,
		Simulated:          simMeta.Simulated,
		SimulationRunID:    simMeta.SimulationRunID,
		SimulationScenario: simMeta.SimulationScenario,
		FakeBill:           simMeta.FakeBill,
		FakeBoard:          simMeta.FakeBoard,
		PricingSnapshot:    pricingSnapshot,
	})
	if err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	trace.checkpoint(ctx, "service_create_order.done", zap.String("order_id", out.Order.ID.String()))
	if out.Replay {
		if err := validateReplayCreateOrder(claims, productID, slotID, cab, sc, slotIdx, pricingSnapshot, out); err != nil {
			return nil, mapCommerceGRPCErr(err)
		}
	} else {
		trace.checkpoint(ctx, "audit.start")
		s.auditCommerce(ctx, claims, compliance.ActionMachineCommerceOrderCreated, map[string]any{
			"order_id":        out.Order.ID.String(),
			"vend_session_id": out.Vend.ID.String(),
			"idempotency_key": wctx.IdempotencyKey,
			"client_event_id": wctx.ClientEventID,
			"product_id":      productID.String(),
		})
		trace.checkpoint(ctx, "audit.done")
		productionmetrics.RecordOrderCreated("grpc_machine")
	}

	sid := ""
	if out.SaleLine.SlotConfigID != uuid.Nil {
		sid = out.SaleLine.SlotConfigID.String()
	}
	trace.checkpoint(ctx, "response.build")
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
	started := time.Now()
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	wctx, err := parseMachineMutationContext(ctx, req.GetContext())
	if err != nil {
		return nil, err
	}
	provider := strings.TrimSpace(req.GetProvider())
	log := observability.LoggerFromContext(ctx, zap.NewNop())
	log.Info("CREATE_PAYMENT_SESSION_START",
		zap.String("order_id", strings.TrimSpace(req.GetOrderId())),
		zap.String("provider", provider),
		zap.String("idempotency_key_fingerprint", idempotencyKeyFingerprint(wctx.IdempotencyKey)),
	)
	claims, svc, store, err := s.requireCommerce(ctx)
	if err != nil {
		return nil, err
	}
	orderID, err := uuid.Parse(strings.TrimSpace(req.GetOrderId()))
	if err != nil || orderID == uuid.Nil {
		return nil, status.Error(codes.InvalidArgument, "invalid order_id")
	}
	principal := machinePrincipalFromAccessClaims(claims)
	if err := svc.EnsureCommerceCallerOrderAccess(ctx, uuid.Nil, orderID, principal); err != nil {
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
	if methods := s.machinePaymentMethods(ctx, claims.MachineID); !methods.QRCardEnabled {
		return nil, status.Error(codes.FailedPrecondition, "provider_unavailable")
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
		OrderID:             orderID,
		MachineID:           claims.MachineID,
		IdempotencyKey:      wctx.IdempotencyKey,
		ClientProvider:      strings.TrimSpace(req.GetProvider()),
		ClientPayState:      strings.TrimSpace(req.GetPaymentState()),
		AmountMinor:         amt,
		Currency:            cur,
		AppEnv:              s.deps.Config.AppEnv,
		OutboxTopic:         topic,
		OutboxEventType:     evType,
		OutboxAggregate:     aggType,
		MachineExternalCode: machineExternalCode(ctx, s.deps, claims.MachineID),
	})
	if err != nil {
		if st, ok := status.FromError(mapCommercePaymentSessionErr(err)); ok {
			log.Error("CREATE_PAYMENT_SESSION_DB_ERROR",
				zap.String("order_id", orderID.String()),
				zap.String("provider", provider),
				zap.String("grpc_code", st.Code().String()),
				zap.String("reason", st.Message()),
				zap.Error(err),
			)
		}
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

	log.Info("CREATE_PAYMENT_SESSION_SUCCESS",
		zap.String("order_id", orderID.String()),
		zap.String("payment_id", res.Payment.ID.String()),
		zap.String("provider", res.ProviderKey),
		zap.Bool("replay", res.Replay),
		zap.Int64("duration_ms", time.Since(started).Milliseconds()),
	)

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
	if err := svc.EnsureCommerceCallerOrderAccess(ctx, uuid.Nil, orderID, principal); err != nil {
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
	if methods := s.machinePaymentMethods(ctx, claims.MachineID); !methods.CashEnabled {
		return nil, status.Error(codes.FailedPrecondition, "cash_payment_disabled")
	}

	topic := strings.TrimSpace(s.deps.Config.Commerce.PaymentOutboxTopic)
	evType := strings.TrimSpace(s.deps.Config.Commerce.PaymentOutboxEventType)
	aggType := strings.TrimSpace(s.deps.Config.Commerce.PaymentOutboxAggregateType)
	if topic == "" || evType == "" || aggType == "" {
		return nil, status.Error(codes.Unavailable, "commerce outbox not configured")
	}

	simMeta := parseSimulationContext(req.GetSimulation())
	if simMeta.Simulated {
		if err := validateSimulationCommerce(claims.MachineID, simMeta, s.deps.Config.AppEnv); err != nil {
			return nil, err
		}
	} else {
		simMeta = simulationMetaFromOrder(o.Simulated, o.SimulationRunID, o.SimulationScenario, o.FakeBill, o.FakeBoard)
	}

	var consentedAt *time.Time
	if req.GetConsentedAt() != nil {
		t := req.GetConsentedAt().AsTime().UTC()
		consentedAt = &t
	}
	acceptance := make([]appcommerce.CashAcceptanceEventInput, 0, len(req.GetCashAcceptanceEvents()))
	for _, ev := range req.GetCashAcceptanceEvents() {
		at := time.Now().UTC()
		if ev.GetAcceptedAt() != nil {
			at = ev.GetAcceptedAt().AsTime().UTC()
		}
		acceptance = append(acceptance, appcommerce.CashAcceptanceEventInput{
			DeviceEventID:     strings.TrimSpace(ev.GetDeviceEventId()),
			DenominationMinor: ev.GetDenominationMinor(),
			CreditSource:      strings.TrimSpace(ev.GetCreditSource()),
			AcceptedAt:        at,
		})
	}

	res, err := svc.ConfirmCashPayment(ctx, appcommerce.ConfirmCashPaymentInput{
		OrderID:                orderID,
		MachineID:              claims.MachineID,
		IdempotencyKey:         wctx.IdempotencyKey,
		GrossAcceptedMinor:     req.GetGrossAcceptedMinor(),
		AllocatedMinor:         req.GetAllocatedMinor(),
		PreOrderCreditMinor:    req.GetPreOrderCreditMinor(),
		PostOrderInsertedMinor: req.GetPostOrderInsertedMinor(),
		ChangeDueMinor:         req.GetChangeDueMinor(),
		ChangeDispensedMinor:   req.GetChangeDispensedMinor(),
		ChangeOutcome:          req.GetChangeOutcome(),
		ConsentSource:          req.GetConsentSource(),
		ConsentedAt:            consentedAt,
		Currency:               req.GetCurrency(),
		AcceptanceEvents:       acceptance,
		Simulated:              simMeta.Simulated,
		SimulationRunID:        simMeta.SimulationRunID,
		SimulationScenario:     simMeta.SimulationScenario,
		FakeBill:               simMeta.FakeBill,
		FakeBoard:              simMeta.FakeBoard,
		OutboxTopic:            topic,
		OutboxEventType:        evType,
		OutboxAggregateType:    aggType,
	})
	if err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	if !res.Replay {
		s.auditCommerce(ctx, claims, compliance.ActionMachineCommerceCashPaymentConfirmed, map[string]any{
			"order_id":        orderID.String(),
			"payment_id":      res.Payment.ID.String(),
			"idempotency_key": wctx.IdempotencyKey,
			"consent_source":  req.GetConsentSource(),
		})
	}
	return &machinev1.ConfirmCashPaymentResponse{
		Replay:       res.Replay,
		PaymentId:    res.Payment.ID.String(),
		OrderStatus:  res.Order.Status,
		PaymentState: res.Payment.State,
	}, nil
}

func (s *machineCommerceServer) CancelPaymentSession(ctx context.Context, req *machinev1.CancelPaymentSessionRequest) (*machinev1.CancelPaymentSessionResponse, error) {
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
	if err := svc.EnsureCommerceCallerOrderAccess(ctx, uuid.Nil, orderID, principal); err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	o, err := store.GetOrderByID(ctx, orderID)
	if err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	if o.MachineID != claims.MachineID {
		return nil, status.Error(codes.PermissionDenied, "order machine mismatch")
	}
	res, err := svc.CancelPaymentSession(ctx, appcommerce.CancelPaymentSessionInput{
		OrderID:        orderID,
		MachineID:      claims.MachineID,
		IdempotencyKey: wctx.IdempotencyKey,
		Reason:         strings.TrimSpace(req.GetReason()),
	})
	if err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	out := &machinev1.CancelPaymentSessionResponse{
		Replay:      res.Replay,
		OrderId:     res.Order.ID.String(),
		OrderStatus: res.Order.Status,
	}
	if res.PaymentFound {
		out.PaymentId = res.Payment.ID.String()
		out.PaymentState = res.Payment.State
	}
	return out, nil
}

func (s *machineCommerceServer) CreateCashCheckout(ctx context.Context, req *machinev1.ConfirmCashPaymentRequest) (*machinev1.ConfirmCashPaymentResponse, error) {
	return s.ConfirmCashPayment(ctx, req)
}

func (s *machineCommerceServer) getStatus(ctx context.Context, claims plauth.MachineAccessClaims, svc appcommerce.Orchestrator, orderID uuid.UUID, slotIndex int32) (appcommerce.CheckoutStatusView, error) {
	principal := machinePrincipalFromAccessClaims(claims)
	if err := svc.EnsureCommerceCallerOrderAccess(ctx, uuid.Nil, orderID, principal); err != nil {
		return appcommerce.CheckoutStatusView{}, err
	}
	return svc.GetOrderStatusView(ctx, uuid.Nil, orderID, slotIndex, 0)
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
	started := time.Now()
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
	slotIndex := req.GetSlotIndex()
	log := observability.LoggerFromContext(ctx, zap.NewNop())
	log.Info("GET_ORDER_STATUS_START",
		zap.String("order_id", orderID.String()),
		zap.String("machine_id", claims.MachineID.String()),
		zap.Int32("slot_index", slotIndex),
	)
	st, err := s.getStatus(ctx, claims, svc, orderID, slotIndex)
	if err != nil {
		if errors.Is(err, appcommerce.ErrNotFound) {
			log.Info("GET_ORDER_STATUS_NOT_FOUND",
				zap.String("order_id", orderID.String()),
				zap.String("machine_id", claims.MachineID.String()),
				zap.Int32("slot_index", slotIndex),
				zap.Int64("duration_ms", time.Since(started).Milliseconds()),
			)
		} else {
			log.Warn("GET_ORDER_STATUS_ERROR",
				zap.String("order_id", orderID.String()),
				zap.String("machine_id", claims.MachineID.String()),
				zap.Int32("slot_index", slotIndex),
				zap.Int64("duration_ms", time.Since(started).Milliseconds()),
				zap.Error(err),
			)
		}
		return nil, mapCommerceGRPCErr(err)
	}
	if st.Vend.ID != uuid.Nil && st.Vend.SlotIndex != slotIndex {
		log.Info("GET_ORDER_STATUS_FALLBACK",
			zap.String("order_id", orderID.String()),
			zap.String("machine_id", claims.MachineID.String()),
			zap.Int32("slot_index", slotIndex),
			zap.String("from", "slot_index"),
			zap.String("to", "order_line"),
			zap.String("reason", "vend_slot_not_matched"),
			zap.Int32("resolved_slot_index", st.Vend.SlotIndex),
		)
	}
	log.Info("GET_ORDER_STATUS_LOOKUP",
		zap.String("order_id", orderID.String()),
		zap.String("machine_id", claims.MachineID.String()),
		zap.Int32("slot_index", slotIndex),
		zap.String("lookup_source", "primary"),
		zap.Bool("order_found", true),
		zap.Bool("payment_found", st.PaymentPresent),
		zap.String("order_state", st.Order.Status),
		zap.String("payment_state", st.Payment.State),
	)
	// Accelerate pending QR payments via provider query (ZaloPay-style); IPN/MQTT remain primary.
	if st.PaymentPresent {
		ps := strings.ToLower(strings.TrimSpace(st.Payment.State))
		if ps == "created" || ps == "authorized" || ps == "pending" {
			svc.RefreshPendingPaymentFromProvider(ctx, uuid.Nil, orderID, machineExternalCode(ctx, s.deps, claims.MachineID))
			st, err = s.getStatus(ctx, claims, svc, orderID, slotIndex)
			if err != nil {
				return nil, mapCommerceGRPCErr(err)
			}
		}
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
	log.Info("GET_ORDER_STATUS_SUCCESS",
		zap.String("order_id", orderID.String()),
		zap.String("machine_id", claims.MachineID.String()),
		zap.Int32("slot_index", slotIndex),
		zap.Int64("duration_ms", time.Since(started).Milliseconds()),
		zap.String("order_state", st.Order.Status),
		zap.String("payment_state", resp.GetPaymentState()),
		zap.Bool("payment_present", st.PaymentPresent),
	)
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
	lineSequence := req.GetLineSequence()
	principal := machinePrincipalFromAccessClaims(claims)
	if err := svc.EnsureCommerceCallerOrderAccess(ctx, uuid.Nil, orderID, principal); err != nil {
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

	var st appcommerce.CheckoutStatusView
	if lineSequence > 0 {
		st, err = svc.GetCheckoutStatusByLineSequence(ctx, uuid.Nil, orderID, lineSequence)
	} else {
		st, err = svc.GetCheckoutStatus(ctx, uuid.Nil, orderID, slotIndex)
	}
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
		respSlot := slotIndex
		if lineSequence > 0 {
			respSlot = st.Vend.SlotIndex
		}
		return &machinev1.StartVendResponse{Replay: true, VendState: "in_progress", SlotIndex: respSlot}, nil
	}
	if st.Vend.State != "pending" {
		return nil, status.Error(codes.FailedPrecondition, "vend not startable")
	}

	v, err := svc.AdvanceVend(ctx, appcommerce.AdvanceVendInput{
		OrderID:      orderID,
		SlotIndex:    st.Vend.SlotIndex,
		LineSequence: lineSequence,
		ToState:      "in_progress",
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

	return s.confirmVendSuccess(ctx, claims, svc, store, wctx, orderID, slotIndex, corr, req.GetEvidence())
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

	out, err := s.confirmVendSuccess(ctx, claims, svc, store, wctx, orderID, slotIndex, corr, req.GetEvidence())
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

func (s *machineCommerceServer) confirmVendSuccess(ctx context.Context, claims plauth.MachineAccessClaims, svc appcommerce.Orchestrator, store *postgres.Store, wctx machineMutationContext, orderID uuid.UUID, slotIndex int32, corr *uuid.UUID, protoEvidence *machinev1.VendHardwareEvidence) (*machinev1.ReportVendSuccessResponse, error) {
	principal := machinePrincipalFromAccessClaims(claims)
	if err := svc.EnsureCommerceCallerOrderAccess(ctx, uuid.Nil, orderID, principal); err != nil {
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
	st, err := svc.GetCheckoutStatus(ctx, uuid.Nil, orderID, slotIndex)
	if err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	if st.Vend.State != "in_progress" {
		return nil, status.Error(codes.FailedPrecondition, "vend not in progress")
	}

	cashFlow := st.PaymentPresent && strings.EqualFold(strings.TrimSpace(st.Payment.Provider), "cash")
	requireEvidence := commerceRequireVendHardwareEvidence(s.deps, claims.MachineID)
	// Authoritative authorized cash amount for reconcile against self-attested bill_final evidence.
	authorizedAmountMinor := int64(0)
	if cashFlow {
		authorizedAmountMinor = st.Payment.AmountMinor
	}
	evidence, verificationStatus, err := resolveVendEvidenceFromRequest(protoEvidence, corr, cashFlow, requireEvidence, true, authorizedAmountMinor)
	if err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	if evidence != nil && corr == nil {
		c := evidence.CorrelationID
		corr = &c
		_ = store.TouchVendSessionCorrelation(ctx, orderID, slotIndex, evidence.CorrelationID)
	}

	outboxTopic, outboxSucceeded, _, outboxRecon, outboxAgg := machineCommerceVendOutboxConfig(s.deps)
	idemKey := strings.TrimSpace(wctx.IdempotencyKey)
	outboxIdem := idemKey + ":vend:success:" + orderID.String()

	fout, err := svc.FinalizeOrderAfterVend(ctx, appcommerce.FinalizeAfterVendInput{
		OrderID:                   orderID,
		SlotIndex:                 slotIndex,
		TerminalVendState:         "success",
		FailureReason:             nil,
		ClientWriteIdempotencyKey: idemKey,
		CorrelationID:             corr,
		Evidence:                  evidence,
		VerificationStatus:        verificationStatus,
		OutboxTopic:               outboxTopic,
		OutboxEventType:           outboxSucceeded,
		OutboxAggregateType:       outboxAgg,
		OutboxIdempotencyKey:      outboxIdem,
		ReconciliationEventType:   outboxRecon,
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
	lineSequence := req.GetLineSequence()
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
	if err := svc.EnsureCommerceCallerOrderAccess(ctx, uuid.Nil, orderID, principal); err != nil {
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
		if lineSequence > 0 {
			stByLine, lineErr := svc.GetCheckoutStatusByLineSequence(ctx, uuid.Nil, orderID, lineSequence)
			if lineErr == nil {
				_ = store.TouchVendSessionCorrelation(ctx, orderID, stByLine.Vend.SlotIndex, *corr)
			}
		} else {
			_ = store.TouchVendSessionCorrelation(ctx, orderID, slotIndex, *corr)
		}
	}
	var st appcommerce.CheckoutStatusView
	if lineSequence > 0 {
		st, err = svc.GetCheckoutStatusByLineSequence(ctx, uuid.Nil, orderID, lineSequence)
	} else {
		st, err = svc.GetCheckoutStatus(ctx, uuid.Nil, orderID, slotIndex)
	}
	if err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	resolvedSlotIndex := st.Vend.SlotIndex
	if lineSequence > 0 {
		slotIndex = resolvedSlotIndex
	}
	if st.Vend.State != "in_progress" {
		return nil, status.Error(codes.FailedPrecondition, "vend not in progress")
	}

	// Persist failure-path hardware evidence the same way as success, BUT a failure report must always
	// finalize/refund: missing or invalid evidence (even when COMMERCE_REQUIRE_VEND_HARDWARE_EVIDENCE is
	// ON) is ACCEPTED and persisted as hardware_unverified rather than rejected. requireSuccessEvidence
	// is false here because a failed vend legitimately has no optical-drop/bill_final to attest.
	cashFlow := st.PaymentPresent && strings.EqualFold(strings.TrimSpace(st.Payment.Provider), "cash")
	requireEvidence := commerceRequireVendHardwareEvidence(s.deps, claims.MachineID)
	authorizedAmountMinor := int64(0)
	if cashFlow {
		authorizedAmountMinor = st.Payment.AmountMinor
	}
	evidence, verificationStatus, evErr := resolveVendEvidenceFromRequest(req.GetEvidence(), corr, cashFlow, requireEvidence, false, authorizedAmountMinor)
	if evErr != nil {
		// Never reject a failure report for evidence reasons; degrade to hardware_unverified so the
		// order still reconciles/refunds deterministically.
		evidence = nil
		verificationStatus = domaincommerce.VerificationHardwareUnverified
	}
	if evidence != nil && corr == nil {
		c := evidence.CorrelationID
		corr = &c
		_ = store.TouchVendSessionCorrelation(ctx, orderID, resolvedSlotIndex, evidence.CorrelationID)
	}

	outboxTopic, _, outboxFailed, outboxRecon, outboxAgg := machineCommerceVendOutboxConfig(s.deps)
	idemKey := strings.TrimSpace(wctx.IdempotencyKey)
	outboxIdem := idemKey + ":vend:failure:" + orderID.String()

	vendReplay := st.Vend.State == "failed"
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}
	fout, err := svc.FinalizeOrderAfterVend(ctx, appcommerce.FinalizeAfterVendInput{
		OrderID:                   orderID,
		SlotIndex:                 resolvedSlotIndex,
		LineSequence:              lineSequence,
		TerminalVendState:         "failed",
		FailureReason:             reasonPtr,
		ClientWriteIdempotencyKey: idemKey,
		CorrelationID:             corr,
		Evidence:                  evidence,
		VerificationStatus:        verificationStatus,
		OutboxTopic:               outboxTopic,
		OutboxEventType:           outboxFailed,
		OutboxAggregateType:       outboxAgg,
		OutboxIdempotencyKey:      outboxIdem,
		ReconciliationEventType:   outboxRecon,
	})
	if err != nil {
		return nil, mapCommerceGRPCErr(err)
	}
	vendReplay = vendReplay || fout.OrderVendReplay

	st2, _ := svc.GetCheckoutStatus(ctx, uuid.Nil, orderID, slotIndex)
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
	if err := svc.EnsureCommerceCallerOrderAccess(ctx, uuid.Nil, orderID, principal); err != nil {
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
	o2, err := svc.CancelOrder(ctx, uuid.Nil, orderID, reason)
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

func idempotencyKeyFingerprint(key string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(key)))
	return hex.EncodeToString(sum[:])[:12]
}
