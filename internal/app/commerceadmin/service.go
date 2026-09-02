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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CommerceRefundDeps creates ledger refund rows (PSP/outbox pipeline).
type CommerceRefundDeps interface {
	CreateRefund(ctx context.Context, in appcommerce.CreateRefundInput) (appcommerce.RefundRowView, error)
}

// CommerceMoneyViewReader loads the admin money read model for an order.
type CommerceMoneyViewReader interface {
	GetOrderMoneyView(ctx context.Context, orderID uuid.UUID) (appcommerce.OrderMoneyView, error)
}

// Service provides operational commerce lists (orders, payments) and reconciliation admin flows.
type Service struct {
	q         *db.Queries
	pool      *pgxpool.Pool
	refunds   CommerceRefundDeps
	moneyView CommerceMoneyViewReader
}

// NewService returns a commerce admin service backed by sqlc queries and optional refund execution.
func NewService(pool *pgxpool.Pool, q *db.Queries, refunds CommerceRefundDeps, moneyView CommerceMoneyViewReader) (*Service, error) {
	if pool == nil {
		return nil, errors.New("commerceadmin: nil pool")
	}
	if q == nil {
		return nil, errors.New("commerceadmin: nil queries")
	}
	return &Service{pool: pool, q: q, refunds: refunds, moneyView: moneyView}, nil
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
func (s *Service) ListOrders(ctx context.Context, scope listscope.CompanyCommerce) (*OrdersListResponse, error) {
	if s == nil || s.q == nil {
		return nil, errors.New("commerceadmin: nil service")
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
		Column1: filterStatus,
		Column2: strings.TrimSpace(scope.Status),
		Column3: filterMachine,
		Column4: mid,
		Column5: st,
		Column6: en,
		Column7: filterSearch,
		Column8: search,
		Limit:   scope.Limit,
		Offset:  scope.Offset,
	}
	countArg := db.CommerceAdminCountOrdersParams{
		Column1: filterStatus,
		Column2: strings.TrimSpace(scope.Status),
		Column3: filterMachine,
		Column4: mid,
		Column5: st,
		Column6: en,
		Column7: filterSearch,
		Column8: search,
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
func (s *Service) ListPayments(ctx context.Context, scope listscope.CompanyCommerce) (*PaymentsListResponse, error) {
	if s == nil || s.q == nil {
		return nil, errors.New("commerceadmin: nil service")
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
		Column1:  filterState,
		Column2:  strings.TrimSpace(scope.Status),
		Column3:  filterProvider,
		Column4:  strings.TrimSpace(scope.PaymentMethod),
		Column5:  filterMachine,
		Column6:  mid,
		Column7:  st,
		Column8:  en,
		Column9:  filterSearch,
		Column10: search,
		Limit:    scope.Limit,
		Offset:   scope.Offset,
	}
	countArg := db.CommerceAdminCountPaymentsParams{
		Column1:  filterState,
		Column2:  strings.TrimSpace(scope.Status),
		Column3:  filterProvider,
		Column4:  strings.TrimSpace(scope.PaymentMethod),
		Column5:  filterMachine,
		Column6:  mid,
		Column7:  st,
		Column8:  en,
		Column9:  filterSearch,
		Column10: search,
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

func (s *Service) ListReconciliationCases(ctx context.Context, scope listscope.CompanyCommerce) (*ReconciliationListResponse, error) {
	if s == nil || s.q == nil {
		return nil, errors.New("commerceadmin: nil service")
	}
	status := strings.TrimSpace(scope.Status)
	caseType := strings.TrimSpace(scope.CaseType)
	rows, err := s.q.CommerceAdminListReconciliationCases(ctx, db.CommerceAdminListReconciliationCasesParams{
		Column1: status != "",
		Column2: status,
		Column3: caseType != "",
		Column4: caseType,
		Limit:   scope.Limit,
		Offset:  scope.Offset,
	})
	if err != nil {
		return nil, err
	}
	total, err := s.q.CommerceAdminCountReconciliationCases(ctx, db.CommerceAdminCountReconciliationCasesParams{
		Column1: status != "",
		Column2: status,
		Column3: caseType != "",
		Column4: caseType,
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

func (s *Service) GetReconciliationCase(ctx context.Context, caseID uuid.UUID) (ReconciliationCaseItem, error) {
	if s == nil || s.q == nil {
		return ReconciliationCaseItem{}, errors.New("commerceadmin: nil service")
	}
	row, err := s.q.CommerceAdminGetReconciliationCase(ctx, caseID)
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
	row, err := qx.CommerceAdminResolveReconciliationCase(ctx, db.CommerceAdminResolveReconciliationCaseParams{Status: st,
		ResolvedBy:     pgtype.UUID{Bytes: in.ResolvedBy, Valid: in.ResolvedBy != uuid.Nil},
		ResolutionNote: pgtype.Text{String: strings.TrimSpace(in.Note), Valid: strings.TrimSpace(in.Note) != ""},

		ID: in.CaseID,
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
			OrderID:    uuid.UUID(row.OrderID.Bytes),
			EventType:  "commerce.reconciliation.case_resolved",
			ActorType:  "admin",
			ActorID:    pgtype.Text{String: in.ResolvedBy.String(), Valid: in.ResolvedBy != uuid.Nil},
			Payload:    compliance.SanitizeJSONBytes(payload),
			OccurredAt: time.Now().UTC(),
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
func (s *Service) ListOrderTimeline(ctx context.Context, orderID uuid.UUID, limit, offset int32) (*OrderTimelineResponse, error) {
	if s == nil || s.q == nil {
		return nil, errors.New("commerceadmin: nil service")
	}
	if orderID == uuid.Nil {
		return nil, errors.New("commerceadmin: order id required")
	}
	if _, err := s.q.CommerceAdminOrderLookup(ctx, orderID); err != nil {
		return nil, err
	}
	rows, err := s.q.CommerceAdminListOrderTimeline(ctx, db.CommerceAdminListOrderTimelineParams{
		OrderID: orderID,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		return nil, err
	}
	total, err := s.q.CommerceAdminCountOrderTimeline(ctx, orderID)
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

// ListRefundRequests lists durable refund review rows for the company.
func (s *Service) ListRefundRequests(ctx context.Context, scope listscope.CompanyCommerce) (*RefundRequestsListResponse, error) {
	if s == nil || s.q == nil {
		return nil, errors.New("commerceadmin: nil service")
	}
	st := strings.TrimSpace(scope.Status)
	rows, err := s.q.CommerceAdminListRefundRequests(ctx, db.CommerceAdminListRefundRequestsParams{
		Column1: st != "",
		Column2: st,
		Limit:   scope.Limit,
		Offset:  scope.Offset,
	})
	if err != nil {
		return nil, err
	}
	total, err := s.q.CommerceAdminCountRefundRequests(ctx, db.CommerceAdminCountRefundRequestsParams{
		Column1: st != "",
		Column2: st,
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

// GetRefundRequest returns one refund_requests row scoped to the company.
func (s *Service) GetRefundRequest(ctx context.Context, refundRequestID uuid.UUID) (RefundRequestItem, error) {
	if s == nil || s.q == nil {
		return RefundRequestItem{}, errors.New("commerceadmin: nil service")
	}
	row, err := s.q.CommerceAdminGetRefundRequest(ctx, refundRequestID)
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
	if in.OrderID == uuid.Nil {
		return CreateOrderRefundResult{}, errors.New("commerceadmin: order_id required")
	}
	idem := strings.TrimSpace(in.IdempotencyKey)
	if idem == "" {
		return CreateOrderRefundResult{}, errors.New("commerceadmin: idempotency_key required")
	}
	if _, err := s.q.CommerceAdminOrderLookup(ctx, in.OrderID); err != nil {
		return CreateOrderRefundResult{}, err
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
		reqRow, err = s.q.CommerceAdminGetRefundRequestByIdempotencyKey(ctx, pgtype.Text{String: idem, Valid: true})
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
	reqRow, err = s.q.CommerceAdminUpdateRefundRequestLinkedRefund(ctx, db.CommerceAdminUpdateRefundRequestLinkedRefundParams{RefundID: pgtype.UUID{Bytes: refund.ID, Valid: true},
		Status: rs,

		ID: reqRow.ID,
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
		OrderID:    in.OrderID,
		EventType:  "commerce.refund.requested",
		ActorType:  "admin",
		ActorID:    pgtype.Text{String: in.RequestedBy.String(), Valid: in.RequestedBy != uuid.Nil},
		Payload:    compliance.SanitizeJSONBytes(payload),
		OccurredAt: time.Now().UTC(),
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
	cs, err := s.GetReconciliationCase(ctx, in.CaseID)
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
		ID:          r.ID.String(),
		OrderID:     r.OrderID.String(),
		AmountMinor: r.AmountMinor,
		Currency:    r.Currency,
		Status:      r.Status,
		CreatedAt:   r.CreatedAt.UTC(),
		UpdatedAt:   r.UpdatedAt.UTC(),
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

// GetPayment returns one payment row by id.
func (s *Service) GetPayment(ctx context.Context, paymentID uuid.UUID) (PaymentDetailItem, error) {
	if s == nil || s.q == nil {
		return PaymentDetailItem{}, errors.New("commerceadmin: nil service")
	}
	pay, err := s.q.GetPaymentByID(ctx, paymentID)
	if err != nil {
		return PaymentDetailItem{}, err
	}
	ord, err := s.q.GetOrderByID(ctx, pay.OrderID)
	if err != nil {
		return PaymentDetailItem{}, err
	}
	isWinner := ord.WinningPaymentID.Valid && uuid.UUID(ord.WinningPaymentID.Bytes) == pay.ID
	item := PaymentDetailItem{
		PaymentListItem: PaymentListItem{
			PaymentID:            pay.ID.String(),
			OrderID:              pay.OrderID.String(),
			MachineID:            ord.MachineID.String(),
			Provider:             pay.Provider,
			PaymentState:         pay.State,
			OrderStatus:          ord.Status,
			AmountMinor:          pay.AmountMinor,
			Currency:             pay.Currency,
			ReconciliationStatus: pay.ReconciliationStatus,
			SettlementStatus:     pay.SettlementStatus,
			CreatedAt:            pay.CreatedAt.UTC(),
			UpdatedAt:            pay.UpdatedAt.UTC(),
		},
		Outcome:          pay.Outcome,
		AttemptSeq:       pay.AttemptSeq,
		IsWinningPayment: isWinner,
	}
	if pay.SupersedesPaymentID.Valid {
		sid := uuid.UUID(pay.SupersedesPaymentID.Bytes).String()
		item.SupersedesPaymentID = &sid
	}
	return item, nil
}

// GetOrderMoneyView returns the admin money read model for an order.
func (s *Service) GetOrderMoneyView(ctx context.Context, orderID uuid.UUID) (OrderMoneyViewResponse, error) {
	if s == nil || s.moneyView == nil {
		return OrderMoneyViewResponse{}, errors.New("commerceadmin: money view not configured")
	}
	view, err := s.moneyView.GetOrderMoneyView(ctx, orderID)
	if err != nil {
		return OrderMoneyViewResponse{}, err
	}
	out := OrderMoneyViewResponse{
		OrderID:              orderID.String(),
		OutstandingLiability: view.OutstandingLiability,
		AcceptanceEvents:     make([]OrderMoneyAcceptanceEvent, 0, len(view.AcceptanceEvents)),
		Payments:             make([]OrderMoneyPaymentItem, 0, len(view.Payments)),
	}
	if view.WinningPaymentID != nil {
		s := view.WinningPaymentID.String()
		out.WinningPaymentID = &s
	}
	for _, p := range view.Payments {
		out.Payments = append(out.Payments, OrderMoneyPaymentItem{
			PaymentID:       p.Payment.ID.String(),
			Provider:        p.Payment.Provider,
			State:           p.Payment.State,
			Outcome:         p.Payment.Outcome,
			AmountMinor:     p.Payment.AmountMinor,
			Currency:        p.Payment.Currency,
			IsWinner:        p.IsWinner,
			IsLosingCapture: p.IsLosingCapture,
		})
	}
	if view.CashAllocation != nil {
		out.CashAllocation = &OrderMoneyCashAllocation{
			AmountMinor:            view.CashAllocation.AmountMinor,
			PreOrderCreditMinor:    view.CashAllocation.PreOrderCreditMinor,
			PostOrderInsertedMinor: view.CashAllocation.PostOrderInsertedMinor,
			ConsentSource:          view.CashAllocation.ConsentSource,
		}
	}
	if view.CashChange != nil {
		out.CashChange = &OrderMoneyCashChange{
			ChangeDueMinor:       view.CashChange.ChangeDueMinor,
			ChangeDispensedMinor: view.CashChange.ChangeDispensedMinor,
			Outcome:              view.CashChange.Outcome,
			LiabilityMinor:       view.CashChange.LiabilityMinor,
		}
	}
	for _, ev := range view.AcceptanceEvents {
		out.AcceptanceEvents = append(out.AcceptanceEvents, OrderMoneyAcceptanceEvent{
			DeviceEventID:     ev.DeviceEventID,
			DenominationMinor: ev.DenominationMinor,
			CreditSource:      ev.CreditSource,
			AcceptedAt:        ev.AcceptedAt.UTC(),
		})
	}
	return out, nil
}
