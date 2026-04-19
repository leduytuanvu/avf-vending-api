package commerceadmin

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/listscope"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Service provides read-only operational commerce lists (orders, payments).
type Service struct {
	q *db.Queries
}

// NewService returns a commerce admin list service backed by sqlc queries.
func NewService(q *db.Queries) (*Service, error) {
	if q == nil {
		return nil, errors.New("commerceadmin: nil queries")
	}
	return &Service{q: q}, nil
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
