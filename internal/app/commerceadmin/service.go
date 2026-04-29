package commerceadmin

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/app/listscope"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CommerceRefundDeps creates ledger refund rows (PSP/outbox pipeline).
type CommerceRefundDeps interface {
	CreateRefund(ctx context.Context, in appcommerce.CreateRefundInput) (appcommerce.RefundRowView, error)
}

// Service provides operational commerce lists (orders, payments) and reconciliation admin flows.
type Service struct {
	q       *db.Queries
	pool    *pgxpool.Pool
	refunds CommerceRefundDeps
}

// NewService returns a commerce admin service backed by sqlc queries and optional refund execution.
func NewService(pool *pgxpool.Pool, q *db.Queries, refunds CommerceRefundDeps) (*Service, error) {
	if pool == nil {
		return nil, errors.New("commerceadmin: nil pool")
	}
	if q == nil {
		return nil, errors.New("commerceadmin: nil queries")
	}
	return &Service{pool: pool, q: q, refunds: refunds}, nil
}

func isPGUniqueViolation(err error) bool {
	var pe *pgconn.PgError
	return errors.As(err, &pe) && pe.Code == "23505"
}

func refundRequestStatusFromLedger(state string) string {
	switch strings.TrimSpace(strings.ToLower(state)) {
	case "completed":
		return "succeeded"
	case "failed":
		return "failed"
	default:
		return "processing"
	}
}

func timeRangeOrAll(from, to *time.Time) (time.Time, time.Time) {
	start := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC)
	if from != nil {
		start = from.UTC()
	}
	if to != nil {
		end = to.UTC()
	}
	return start, end
}

func pgTextToStringPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	s := t.String
	return &s
}

func pgUUIDStringPtr(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	s := uuid.UUID(u.Bytes).String()
	return &s
}

func pgInt8Ptr(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	x := v.Int64
	return &x
}

func pgTimePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	tt := t.Time.UTC()
	return &tt
}

// ListOrders implements api.OrdersService.
func (s *Service) ListOrders(ctx context.Context, scope listscope.TenantCommerce) (*OrdersListResponse, error) {
	if s == nil || s.q == nil {
		return nil, errors.New("commerceadmin: nil service")
	}
	if scope.OrganizationID == uuid.Nil {
		return nil, listscope.ErrCommerceOrganizationQueryRequired
	}
	st, en := timeRangeOrAll(scope.From, scope.To)
	filterStatus := strings.TrimSpace(scope.Status) != ""
	filterMachine := scope.MachineID != nil && *scope.MachineID != uuid.Nil
	mid := uuid.Nil
	if filterMachine {
		mid = *scope.MachineID
	}
	search := strings.TrimSpace(scope.Search)
	filterSearch := search != ""

	listArg := db.CommerceAdminListOrdersParams{
		OrganizationID: scope.OrganizationID,
		Column2:        filterStatus,
		Column3:        strings.TrimSpace(scope.Status),
		Column4:        filterMachine,
		Column5:        mid,
		Column6:        st,
		Column7:        en,
		Column8:        filterSearch,
		Column9:        search,
		Limit:          scope.Limit,
		Offset:         scope.Offset,
	}
	countArg := db.CommerceAdminCountOrdersParams{
		OrganizationID: scope.OrganizationID,
		Column2:        filterStatus,
		Column3:        strings.TrimSpace(scope.Status),
		Column4:        filterMachine,
		Column5:        mid,
		Column6:        st,
		Column7:        en,
		Column8:        filterSearch,
		Column9:        search,
	}
	rows, err := s.q.CommerceAdminListOrders(ctx, listArg)
	if err != nil {
		return nil, err
	}
	total, err := s.q.CommerceAdminCountOrders(ctx, countArg)
	if err != nil {
		return nil, err
	}
	items := make([]OrderListItem, 0, len(rows))
	for _, o := range rows {
		items = append(items, OrderListItem{
			OrderID:        o.ID.String(),
			OrganizationID: o.OrganizationID.String(),
			MachineID:      o.MachineID.String(),
			Status:         o.Status,
			Currency:       o.Currency,
			SubtotalMinor:  o.SubtotalMinor,
			TaxMinor:       o.TaxMinor,
			TotalMinor:     o.TotalMinor,
			IdempotencyKey: pgTextToStringPtr(o.IdempotencyKey),
			CreatedAt:      o.CreatedAt.UTC(),
			UpdatedAt:      o.UpdatedAt.UTC(),
		})
	}
	return &OrdersListResponse{
		Items: items,
		Meta: listscope.CollectionMeta{
			Limit:    scope.Limit,
			Offset:   scope.Offset,
			Returned: len(items),
			Total:    total,
		},
	}, nil
}

// ListPayments implements api.PaymentsService.
func (s *Service) ListPayments(ctx context.Context, scope listscope.TenantCommerce) (*PaymentsListResponse, error) {
	if s == nil || s.q == nil {
		return nil, errors.New("commerceadmin: nil service")
	}
	if scope.OrganizationID == uuid.Nil {
		return nil, listscope.ErrCommerceOrganizationQueryRequired
	}
	st, en := timeRangeOrAll(scope.From, scope.To)
	filterState := strings.TrimSpace(scope.Status) != ""
	filterProvider := strings.TrimSpace(scope.PaymentMethod) != ""
	filterMachine := scope.MachineID != nil && *scope.MachineID != uuid.Nil
	mid := uuid.Nil
	if filterMachine {
		mid = *scope.MachineID
	}
	search := strings.TrimSpace(scope.Search)
	filterSearch := search != ""

	listArg := db.CommerceAdminListPaymentsParams{
		OrganizationID: scope.OrganizationID,
		Column2:        filterState,
		Column3:        strings.TrimSpace(scope.Status),
		Column4:        filterProvider,
		Column5:        strings.TrimSpace(scope.PaymentMethod),
		Column6:        filterMachine,
		Column7:        mid,
		Column8:        st,
		Column9:        en,
		Column10:       filterSearch,
		Column11:       search,
		Limit:          scope.Limit,
		Offset:         scope.Offset,
	}
	countArg := db.CommerceAdminCountPaymentsParams{
		OrganizationID: scope.OrganizationID,
		Column2:        filterState,
		Column3:        strings.TrimSpace(scope.Status),
		Column4:        filterProvider,
		Column5:        strings.TrimSpace(scope.PaymentMethod),
		Column6:        filterMachine,
		Column7:        mid,
		Column8:        st,
		Column9:        en,
		Column10:       filterSearch,
		Column11:       search,
	}
	rows, err := s.q.CommerceAdminListPayments(ctx, listArg)
	if err != nil {
		return nil, err
	}
	total, err := s.q.CommerceAdminCountPayments(ctx, countArg)
	if err != nil {
		return nil, err
	}
	items := make([]PaymentListItem, 0, len(rows))
	for _, p := range rows {
		items = append(items, PaymentListItem{
			PaymentID:            p.PaymentID.String(),
			OrderID:              p.OrderID.String(),
			OrganizationID:       p.OrganizationID.String(),
			MachineID:            p.MachineID.String(),
			Provider:             p.Provider,
			PaymentState:         p.PaymentState,
			OrderStatus:          p.OrderStatus,
			AmountMinor:          p.AmountMinor,
			Currency:             p.Currency,
			ReconciliationStatus: p.ReconciliationStatus,
			SettlementStatus:     p.SettlementStatus,
			CreatedAt:            p.CreatedAt.UTC(),
			UpdatedAt:            p.UpdatedAt.UTC(),
		})
	}
	return &PaymentsListResponse{
		Items: items,
		Meta: listscope.CollectionMeta{
			Limit:    scope.Limit,
			Offset:   scope.Offset,
			Returned: len(items),
			Total:    total,
		},
	}, nil
}

func (s *Service) ListReconciliationCases(ctx context.Context, scope listscope.TenantCommerce) (*ReconciliationListResponse, error) {
	if s == nil || s.q == nil {
		return nil, errors.New("commerceadmin: nil service")
	}
	if scope.OrganizationID == uuid.Nil {
		return nil, listscope.ErrCommerceOrganizationQueryRequired
	}
	status := strings.TrimSpace(scope.Status)
	caseType := strings.TrimSpace(scope.CaseType)
	rows, err := s.q.CommerceAdminListReconciliationCases(ctx, db.CommerceAdminListReconciliationCasesParams{
		OrganizationID: scope.OrganizationID,
		Column2:        status != "",
		Column3:        status,
		Column4:        caseType != "",
		Column5:        caseType,
		Limit:          scope.Limit,
		Offset:         scope.Offset,
	})
	if err != nil {
		return nil, err
	}
	total, err := s.q.CommerceAdminCountReconciliationCases(ctx, db.CommerceAdminCountReconciliationCasesParams{
		OrganizationID: scope.OrganizationID,
		Column2:        status != "",
		Column3:        status,
		Column4:        caseType != "",
		Column5:        caseType,
	})
	if err != nil {
		return nil, err
	}
	items := make([]ReconciliationCaseItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapReconciliationCase(row))
	}
	return &ReconciliationListResponse{
		Items: items,
		Meta: listscope.CollectionMeta{
			Limit:    scope.Limit,
			Offset:   scope.Offset,
			Returned: len(items),
			Total:    total,
		},
	}, nil
}

func (s *Service) GetReconciliationCase(ctx context.Context, organizationID, caseID uuid.UUID) (ReconciliationCaseItem, error) {
	if s == nil || s.q == nil {
		return ReconciliationCaseItem{}, errors.New("commerceadmin: nil service")
	}
	row, err := s.q.CommerceAdminGetReconciliationCase(ctx, db.CommerceAdminGetReconciliationCaseParams{
		OrganizationID: organizationID,
		ID:             caseID,
	})
	if err != nil {
		return ReconciliationCaseItem{}, err
	}
	return mapReconciliationCase(row), nil
}

func (s *Service) ResolveReconciliationCase(ctx context.Context, in ResolveReconciliationInput) (ReconciliationCaseItem, error) {
	if s == nil || s.q == nil || s.pool == nil {
		return ReconciliationCaseItem{}, errors.New("commerceadmin: nil service")
	}
	st := strings.TrimSpace(strings.ToLower(in.Status))
	switch st {
	case "resolved", "dismissed", "ignored", "escalated":
	default:
		return ReconciliationCaseItem{}, errors.New("commerceadmin: resolution status must be resolved, dismissed, ignored, or escalated")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ReconciliationCaseItem{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qx := s.q.WithTx(tx)
	row, err := qx.CommerceAdminResolveReconciliationCase(ctx, db.CommerceAdminResolveReconciliationCaseParams{
		OrganizationID: in.OrganizationID,
		ID:             in.CaseID,
		Status:         st,
		ResolvedBy:     pgtype.UUID{Bytes: in.ResolvedBy, Valid: in.ResolvedBy != uuid.Nil},
		ResolutionNote: pgtype.Text{String: strings.TrimSpace(in.Note), Valid: strings.TrimSpace(in.Note) != ""},
	})
	if err != nil {
		return ReconciliationCaseItem{}, err
	}
	if row.OrderID.Valid {
		payload, _ := json.Marshal(map[string]any{
			"caseId":         in.CaseID.String(),
			"terminalStatus": st,
			"note":           strings.TrimSpace(in.Note),
		})
		tErr := qx.InsertOrderTimelineEvent(ctx, db.InsertOrderTimelineEventParams{
			OrganizationID: in.OrganizationID,
			OrderID:        uuid.UUID(row.OrderID.Bytes),
			EventType:      "commerce.reconciliation.case_resolved",
			ActorType:      "admin",
			ActorID:        pgtype.Text{String: in.ResolvedBy.String(), Valid: in.ResolvedBy != uuid.Nil},
			Payload:        compliance.SanitizeJSONBytes(payload),
			OccurredAt:     time.Now().UTC(),
		})
		if tErr != nil {
			return ReconciliationCaseItem{}, tErr
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return ReconciliationCaseItem{}, err
	}
	return mapReconciliationCase(row), nil
}

// ListOrderTimeline returns paginated lifecycle events for one order.
func (s *Service) ListOrderTimeline(ctx context.Context, organizationID, orderID uuid.UUID, limit, offset int32) (*OrderTimelineResponse, error) {
	if s == nil || s.q == nil {
		return nil, errors.New("commerceadmin: nil service")
	}
	if organizationID == uuid.Nil || orderID == uuid.Nil {
		return nil, listscope.ErrCommerceOrganizationQueryRequired
	}
	orgRow, err := s.q.CommerceAdminOrderOrganizationID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if orgRow != organizationID {
		return nil, pgx.ErrNoRows
	}
	rows, err := s.q.CommerceAdminListOrderTimeline(ctx, db.CommerceAdminListOrderTimelineParams{
		OrganizationID: organizationID,
		OrderID:        orderID,
		Limit:          limit,
		Offset:         offset,
	})
	if err != nil {
		return nil, err
	}
	total, err := s.q.CommerceAdminCountOrderTimeline(ctx, db.CommerceAdminCountOrderTimelineParams{
		OrganizationID: organizationID,
		OrderID:        orderID,
	})
	if err != nil {
		return nil, err
	}
	items := make([]OrderTimelineEventItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, OrderTimelineEventItem{
			ID:         r.ID.String(),
			EventType:  r.EventType,
			ActorType:  r.ActorType,
			ActorID:    pgTextToStringPtr(r.ActorID),
			Payload:    json.RawMessage(append([]byte(nil), r.Payload...)),
			OccurredAt: r.OccurredAt.UTC(),
			CreatedAt:  r.CreatedAt.UTC(),
		})
	}
	return &OrderTimelineResponse{
		Items: items,
		Meta: listscope.CollectionMeta{
			Limit:    limit,
			Offset:   offset,
			Returned: len(items),
			Total:    total,
		},
	}, nil
}

// ListRefundRequests lists durable refund review rows for the organization.
func (s *Service) ListRefundRequests(ctx context.Context, scope listscope.TenantCommerce) (*RefundRequestsListResponse, error) {
	if s == nil || s.q == nil {
		return nil, errors.New("commerceadmin: nil service")
	}
	if scope.OrganizationID == uuid.Nil {
		return nil, listscope.ErrCommerceOrganizationQueryRequired
	}
	st := strings.TrimSpace(scope.Status)
	rows, err := s.q.CommerceAdminListRefundRequests(ctx, db.CommerceAdminListRefundRequestsParams{
		OrganizationID: scope.OrganizationID,
		Column2:        st != "",
		Column3:        st,
		Limit:          scope.Limit,
		Offset:         scope.Offset,
	})
	if err != nil {
		return nil, err
	}
	total, err := s.q.CommerceAdminCountRefundRequests(ctx, db.CommerceAdminCountRefundRequestsParams{
		OrganizationID: scope.OrganizationID,
		Column2:        st != "",
		Column3:        st,
	})
	if err != nil {
		return nil, err
	}
	items := make([]RefundRequestItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, mapRefundRequestRow(r))
	}
	return &RefundRequestsListResponse{
		Items: items,
		Meta: listscope.CollectionMeta{
			Limit:    scope.Limit,
			Offset:   scope.Offset,
			Returned: len(items),
			Total:    total,
		},
	}, nil
}

// GetRefundRequest returns one refund_requests row scoped to the organization.
func (s *Service) GetRefundRequest(ctx context.Context, organizationID, refundRequestID uuid.UUID) (RefundRequestItem, error) {
	if s == nil || s.q == nil {
		return RefundRequestItem{}, errors.New("commerceadmin: nil service")
	}
	row, err := s.q.CommerceAdminGetRefundRequest(ctx, db.CommerceAdminGetRefundRequestParams{
		OrganizationID: organizationID,
		ID:             refundRequestID,
	})
	if err != nil {
		return RefundRequestItem{}, err
	}
	return mapRefundRequestRow(row), nil
}

// CreateOrderRefund inserts a refund_requests row and executes the ledger refund (idempotent on idempotency key).
func (s *Service) CreateOrderRefund(ctx context.Context, in CreateOrderRefundInput) (CreateOrderRefundResult, error) {
	if s == nil || s.q == nil || s.refunds == nil {
		return CreateOrderRefundResult{}, errors.New("commerceadmin: refund execution not configured")
	}
	if in.OrganizationID == uuid.Nil || in.OrderID == uuid.Nil {
		return CreateOrderRefundResult{}, errors.New("commerceadmin: organization_id and order_id required")
	}
	idem := strings.TrimSpace(in.IdempotencyKey)
	if idem == "" {
		return CreateOrderRefundResult{}, errors.New("commerceadmin: idempotency_key required")
	}
	orgRow, err := s.q.CommerceAdminOrderOrganizationID(ctx, in.OrderID)
	if err != nil {
		return CreateOrderRefundResult{}, err
	}
	if orgRow != in.OrganizationID {
		return CreateOrderRefundResult{}, pgx.ErrNoRows
	}
	pay, err := s.q.GetLatestPaymentForOrder(ctx, in.OrderID)
	if err != nil {
		return CreateOrderRefundResult{}, err
	}
	sumRef, err := s.q.SumNonFailedRefundAmountForPayment(ctx, pay.ID)
	if err != nil {
		return CreateOrderRefundResult{}, err
	}
	remain := pay.AmountMinor - sumRef
	if remain <= 0 {
		return CreateOrderRefundResult{}, errors.New("commerceadmin: nothing_to_refund")
	}
	amt := remain
	if in.AmountMinor != nil && *in.AmountMinor > 0 {
		amt = *in.AmountMinor
	}
	if amt <= 0 || amt > remain {
		return CreateOrderRefundResult{}, errors.New("commerceadmin: invalid_amount")
	}
	cur := strings.TrimSpace(in.Currency)
	if cur == "" {
		cur = pay.Currency
	}
	cur = strings.ToUpper(cur)
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		reason = "operator_requested_order_refund"
	}
	metaBytes, _ := json.Marshal(map[string]any{"source": "admin_order_refund", "refund_request_idempotency": idem})
	reqRow, insErr := s.q.CommerceAdminInsertRefundRequest(ctx, db.CommerceAdminInsertRefundRequestParams{
		OrganizationID: in.OrganizationID,
		OrderID:        in.OrderID,
		PaymentID:      pgtype.UUID{Bytes: pay.ID, Valid: true},
		AmountMinor:    amt,
		Currency:       cur,
		Reason:         pgtype.Text{String: reason, Valid: true},
		Status:         "requested",
		RequestedBy:    pgtype.UUID{Bytes: in.RequestedBy, Valid: in.RequestedBy != uuid.Nil},
		IdempotencyKey: pgtype.Text{String: idem, Valid: true},
	})
	if insErr != nil {
		if !isPGUniqueViolation(insErr) {
			return CreateOrderRefundResult{}, insErr
		}
		reqRow, err = s.q.CommerceAdminGetRefundRequestByOrgIdempotency(ctx, db.CommerceAdminGetRefundRequestByOrgIdempotencyParams{
			OrganizationID: in.OrganizationID,
			IdempotencyKey: pgtype.Text{String: idem, Valid: true},
		})
		if err != nil {
			return CreateOrderRefundResult{}, err
		}
		if reqRow.RefundID.Valid {
			rid := uuid.UUID(reqRow.RefundID.Bytes)
			refRow, err := s.q.GetRefundByIDForOrder(ctx, db.GetRefundByIDForOrderParams{
				ID:      rid,
				OrderID: in.OrderID,
			})
			if err != nil {
				return CreateOrderRefundResult{}, err
			}
			return CreateOrderRefundResult{
				RefundRequest:     mapRefundRequestRow(reqRow),
				LedgerRefundID:    rid.String(),
				LedgerState:       refRow.State,
				LedgerAmountMinor: refRow.AmountMinor,
				LedgerCurrency:    refRow.Currency,
			}, nil
		}
	}
	refund, err := s.refunds.CreateRefund(ctx, appcommerce.CreateRefundInput{
		OrganizationID: in.OrganizationID,
		OrderID:        in.OrderID,
		AmountMinor:    amt,
		Currency:       cur,
		Reason:         reason,
		IdempotencyKey: idem,
		Metadata:       metaBytes,
	})
	if err != nil {
		return CreateOrderRefundResult{}, err
	}
	rs := refundRequestStatusFromLedger(refund.State)
	reqRow, err = s.q.CommerceAdminUpdateRefundRequestLinkedRefund(ctx, db.CommerceAdminUpdateRefundRequestLinkedRefundParams{
		OrganizationID: in.OrganizationID,
		ID:             reqRow.ID,
		RefundID:       pgtype.UUID{Bytes: refund.ID, Valid: true},
		Status:         rs,
	})
	if err != nil {
		return CreateOrderRefundResult{}, err
	}
	payload, _ := json.Marshal(map[string]any{
		"refundRequestId": reqRow.ID.String(),
		"refundId":        refund.ID.String(),
		"amountMinor":     amt,
		"currency":        cur,
	})
	_ = s.q.InsertOrderTimelineEvent(ctx, db.InsertOrderTimelineEventParams{
		OrganizationID: in.OrganizationID,
		OrderID:        in.OrderID,
		EventType:      "commerce.refund.requested",
		ActorType:      "admin",
		ActorID:        pgtype.Text{String: in.RequestedBy.String(), Valid: in.RequestedBy != uuid.Nil},
		Payload:        compliance.SanitizeJSONBytes(payload),
		OccurredAt:     time.Now().UTC(),
	})
	return CreateOrderRefundResult{
		RefundRequest:     mapRefundRequestRow(reqRow),
		LedgerRefundID:    refund.ID.String(),
		LedgerState:       refund.State,
		LedgerAmountMinor: refund.AmountMinor,
		LedgerCurrency:    refund.Currency,
	}, nil
}

// RefundFromReconciliationCase validates the case and executes CreateOrderRefund with a case-scoped idempotency key.
func (s *Service) RefundFromReconciliationCase(ctx context.Context, in RefundFromReconciliationCaseInput) (CreateOrderRefundResult, error) {
	cs, err := s.GetReconciliationCase(ctx, in.OrganizationID, in.CaseID)
	if err != nil {
		return CreateOrderRefundResult{}, err
	}
	st := strings.TrimSpace(strings.ToLower(cs.Status))
	if st != "open" && st != "reviewing" && st != "escalated" {
		return CreateOrderRefundResult{}, errors.New("commerceadmin: case_not_actionable")
	}
	switch cs.CaseType {
	case "payment_paid_vend_failed", "payment_paid_vend_not_started", "webhook_amount_currency_mismatch", "webhook_after_terminal_order":
	default:
		return CreateOrderRefundResult{}, errors.New("commerceadmin: refund_not_supported_for_case_type")
	}
	if cs.OrderID == nil || strings.TrimSpace(*cs.OrderID) == "" {
		return CreateOrderRefundResult{}, errors.New("commerceadmin: order_required")
	}
	orderID, err := uuid.Parse(strings.TrimSpace(*cs.OrderID))
	if err != nil || orderID == uuid.Nil {
		return CreateOrderRefundResult{}, errors.New("commerceadmin: invalid_order_id")
	}
	idem := "reconciliation_case_refund:" + in.CaseID.String()
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		reason = "operator_requested_from_reconciliation_case"
	}
	metaReason := reason + " reconciliation_case_id=" + in.CaseID.String()
	return s.CreateOrderRefund(ctx, CreateOrderRefundInput{
		OrganizationID: in.OrganizationID,
		OrderID:        orderID,
		AmountMinor:    in.AmountMinor,
		Currency:       "",
		Reason:         metaReason,
		RequestedBy:    in.RequestedBy,
		IdempotencyKey: idem,
	})
}

func mapRefundRequestRow(r db.RefundRequest) RefundRequestItem {
	out := RefundRequestItem{
		ID:             r.ID.String(),
		OrganizationID: r.OrganizationID.String(),
		OrderID:        r.OrderID.String(),
		AmountMinor:    r.AmountMinor,
		Currency:       r.Currency,
		Status:         r.Status,
		CreatedAt:      r.CreatedAt.UTC(),
		UpdatedAt:      r.UpdatedAt.UTC(),
	}
	if r.PaymentID.Valid {
		s := uuid.UUID(r.PaymentID.Bytes).String()
		out.PaymentID = &s
	}
	if r.RefundID.Valid {
		s := uuid.UUID(r.RefundID.Bytes).String()
		out.RefundID = &s
	}
	out.Reason = pgTextToStringPtr(r.Reason)
	out.ProviderRefundID = pgTextToStringPtr(r.ProviderRefundID)
	out.IdempotencyKey = pgTextToStringPtr(r.IdempotencyKey)
	out.RequestedBy = pgUUIDStringPtr(r.RequestedBy)
	out.ApprovedBy = pgUUIDStringPtr(r.ApprovedBy)
	out.CompletedAt = pgTimePtr(r.CompletedAt)
	return out
}

func mapReconciliationCase(row db.CommerceReconciliationCase) ReconciliationCaseItem {
	return ReconciliationCaseItem{
		ID:              row.ID.String(),
		OrganizationID:  row.OrganizationID.String(),
		CaseType:        row.CaseType,
		Status:          row.Status,
		Severity:        row.Severity,
		OrderID:         pgUUIDStringPtr(row.OrderID),
		PaymentID:       pgUUIDStringPtr(row.PaymentID),
		VendSessionID:   pgUUIDStringPtr(row.VendSessionID),
		MachineID:       pgUUIDStringPtr(row.MachineID),
		RefundID:        pgUUIDStringPtr(row.RefundID),
		Provider:        pgTextToStringPtr(row.Provider),
		ProviderEventID: pgInt8Ptr(row.ProviderEventID),
		Reason:          row.Reason,
		CorrelationKey:  strings.TrimSpace(row.CorrelationKey),
		Metadata:        row.Metadata,
		FirstDetectedAt: row.FirstDetectedAt.UTC(),
		LastDetectedAt:  row.LastDetectedAt.UTC(),
		ResolvedAt:      pgTimePtr(row.ResolvedAt),
		ResolvedBy:      pgUUIDStringPtr(row.ResolvedBy),
		ResolutionNote:  pgTextToStringPtr(row.ResolutionNote),
	}
}
