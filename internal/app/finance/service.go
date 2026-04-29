package finance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// Service manages immutable finance daily closes.
type Service struct {
	q     *db.Queries
	audit compliance.EnterpriseRecorder
}

// NewService constructs the finance application service.
func NewService(q *db.Queries, audit compliance.EnterpriseRecorder) *Service {
	if q == nil {
		panic("finance.NewService: nil queries")
	}
	return &Service{q: q, audit: audit}
}

// CreateDailyClose computes totals for the calendar closeDate in timezone and persists an immutable snapshot.
func (s *Service) CreateDailyClose(ctx context.Context, in CreateDailyCloseInput) (*DailyCloseView, error) {
	if strings.TrimSpace(in.IdempotencyKey) == "" {
		return nil, ErrInvalidDailyCloseInput
	}
	rawDate := strings.TrimSpace(in.CloseDate)
	tzName := strings.TrimSpace(in.Timezone)
	if rawDate == "" || tzName == "" {
		return nil, ErrInvalidDailyCloseInput
	}
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		return nil, ErrInvalidDailyCloseInput
	}
	d, err := time.ParseInLocation("2006-01-02", rawDate, loc)
	if err != nil {
		return nil, ErrInvalidDailyCloseInput
	}
	y, m, day := d.Date()
	startLocal := time.Date(y, m, day, 0, 0, 0, 0, loc)
	endLocal := startLocal.AddDate(0, 0, 1)
	fromUTC := startLocal.UTC()
	toUTC := endLocal.UTC()

	if err := validateScope(ctx, s.q, in); err != nil {
		return nil, err
	}

	existingID, err := s.q.GetFinanceDailyCloseByIdempotencyKey(ctx, db.GetFinanceDailyCloseByIdempotencyKeyParams{
		OrganizationID: in.OrganizationID,
		IdempotencyKey: strings.TrimSpace(in.IdempotencyKey),
	})
	if err == nil {
		v := mapFinanceDailyClose(existingID)
		return &v, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("finance daily close idempotency lookup: %w", err)
	}

	closePgDate := pgtype.Date{Time: time.Date(y, m, day, 0, 0, 0, 0, time.UTC), Valid: true}
	dup, err := s.q.FinanceDailyCloseExistsForScope(ctx, db.FinanceDailyCloseExistsForScopeParams{
		OrganizationID: in.OrganizationID,
		CloseDate:      closePgDate,
		Timezone:       tzName,
		Column4:        in.SiteID,
		Column5:        in.MachineID,
	})
	if err != nil {
		return nil, fmt.Errorf("finance daily close scope lookup: %w", err)
	}
	if dup {
		return nil, ErrDuplicateDailyClose
	}

	agg, err := s.q.FinanceDailyCloseAggregate(ctx, db.FinanceDailyCloseAggregateParams{
		OrganizationID: in.OrganizationID,
		Column2:        fromUTC,
		Column3:        toUTC,
		Column4:        in.SiteID,
		Column5:        in.MachineID,
	})
	if err != nil {
		return nil, fmt.Errorf("finance daily close aggregate: %w", err)
	}
	net := agg.GrossSalesMinor - agg.DiscountMinor - agg.RefundMinor

	insert := db.InsertFinanceDailyCloseParams{
		OrganizationID:  in.OrganizationID,
		CloseDate:       closePgDate,
		Timezone:        tzName,
		SiteID:          uuidToNullablePg(in.SiteID),
		MachineID:       uuidToNullablePg(in.MachineID),
		IdempotencyKey:  strings.TrimSpace(in.IdempotencyKey),
		GrossSalesMinor: agg.GrossSalesMinor,
		DiscountMinor:   agg.DiscountMinor,
		RefundMinor:     agg.RefundMinor,
		NetMinor:        net,
		CashMinor:       agg.CashMinor,
		QrWalletMinor:   agg.QrWalletMinor,
		FailedMinor:     agg.FailedMinor,
		PendingMinor:    agg.PendingMinor,
	}

	row, err := s.q.InsertFinanceDailyClose(ctx, insert)
	if err != nil {
		if isPGUniqueViolation(err) {
			var pe *pgconn.PgError
			if errors.As(err, &pe) && strings.Contains(pe.ConstraintName, "finance_daily_closes_org_idem") {
				got, err2 := s.q.GetFinanceDailyCloseByIdempotencyKey(ctx, db.GetFinanceDailyCloseByIdempotencyKeyParams{
					OrganizationID: in.OrganizationID,
					IdempotencyKey: strings.TrimSpace(in.IdempotencyKey),
				})
				if err2 == nil {
					v := mapFinanceDailyClose(got)
					return &v, nil
				}
			}
			if errors.As(err, &pe) && strings.Contains(pe.ConstraintName, "finance_daily_closes_scope") {
				return nil, ErrDuplicateDailyClose
			}
		}
		return nil, fmt.Errorf("finance daily close insert: %w", err)
	}

	v := mapFinanceDailyClose(row)
	s.emitDailyCloseAudit(ctx, in, row.ID)
	return &v, nil
}

func (s *Service) emitDailyCloseAudit(ctx context.Context, in CreateDailyCloseInput, closeID uuid.UUID) {
	if s.audit == nil {
		return
	}
	rid := closeID.String()
	md, err := json.Marshal(map[string]any{
		"close_date":      strings.TrimSpace(in.CloseDate),
		"timezone":        strings.TrimSpace(in.Timezone),
		"idempotency_key": strings.TrimSpace(in.IdempotencyKey),
	})
	if err != nil {
		md = []byte("{}")
	}
	md = compliance.SanitizeJSONBytes(md)
	actorType := strings.TrimSpace(in.ActorType)
	if actorType == "" {
		actorType = compliance.ActorUser
	}
	_ = s.audit.Record(ctx, compliance.EnterpriseAuditRecord{
		OrganizationID: in.OrganizationID,
		ActorType:      actorType,
		ActorID:        in.ActorID,
		Action:         compliance.ActionFinanceDailyCloseCreated,
		ResourceType:   "finance.daily_close",
		ResourceID:     &rid,
		Metadata:       md,
	})
}

// GetDailyClose returns one immutable close row for the organization.
func (s *Service) GetDailyClose(ctx context.Context, organizationID, closeID uuid.UUID) (*DailyCloseView, error) {
	row, err := s.q.GetFinanceDailyCloseByIDForOrg(ctx, db.GetFinanceDailyCloseByIDForOrgParams{
		ID:             closeID,
		OrganizationID: organizationID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrDailyCloseNotFound
		}
		return nil, fmt.Errorf("finance daily close get: %w", err)
	}
	v := mapFinanceDailyClose(row)
	return &v, nil
}

// ListDailyCloseParams carries pagination for tenant-scoped listing.
type ListDailyCloseParams struct {
	OrganizationID uuid.UUID
	Limit          int32
	Offset         int32
}

// ListDailyClose lists closes newest-first for an organization.
func (s *Service) ListDailyClose(ctx context.Context, p ListDailyCloseParams) (*DailyCloseListResponse, error) {
	if p.Limit <= 0 {
		p.Limit = 50
	}
	rows, err := s.q.ListFinanceDailyClosesForOrg(ctx, db.ListFinanceDailyClosesForOrgParams{
		OrganizationID: p.OrganizationID,
		Limit:          p.Limit,
		Offset:         p.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("finance daily close list: %w", err)
	}
	total, err := s.q.CountFinanceDailyClosesForOrg(ctx, p.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("finance daily close count: %w", err)
	}
	items := make([]DailyCloseView, 0, len(rows))
	for _, r := range rows {
		items = append(items, mapFinanceDailyClose(r))
	}
	return &DailyCloseListResponse{
		Items: items,
		Meta: DailyCloseMeta{
			Limit:    p.Limit,
			Offset:   p.Offset,
			Returned: len(items),
			Total:    total,
		},
	}, nil
}

func validateScope(ctx context.Context, q *db.Queries, in CreateDailyCloseInput) error {
	if in.MachineID != uuid.Nil {
		m, err := q.GetMachineByID(ctx, in.MachineID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrInvalidDailyCloseInput
			}
			return fmt.Errorf("finance daily close machine lookup: %w", err)
		}
		if m.OrganizationID != in.OrganizationID {
			return ErrInvalidDailyCloseInput
		}
		if in.SiteID != uuid.Nil && m.SiteID != in.SiteID {
			return ErrInvalidDailyCloseInput
		}
		return nil
	}
	if in.SiteID != uuid.Nil {
		site, err := q.GetSiteByID(ctx, in.SiteID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrInvalidDailyCloseInput
			}
			return fmt.Errorf("finance daily close site lookup: %w", err)
		}
		if site.OrganizationID != in.OrganizationID {
			return ErrInvalidDailyCloseInput
		}
	}
	return nil
}

func mapFinanceDailyClose(row db.FinanceDailyClose) DailyCloseView {
	v := DailyCloseView{
		ID:               row.ID.String(),
		OrganizationID:   row.OrganizationID.String(),
		CloseDate:        pgDateToString(row.CloseDate),
		Timezone:         row.Timezone,
		IdempotencyKey:   row.IdempotencyKey,
		GrossSalesMinor:  row.GrossSalesMinor,
		DiscountMinor:    row.DiscountMinor,
		RefundMinor:      row.RefundMinor,
		NetMinor:         row.NetMinor,
		CashMinor:        row.CashMinor,
		QRWalletMinor:    row.QrWalletMinor,
		FailedMinor:      row.FailedMinor,
		PendingMinor:     row.PendingMinor,
		CreatedAtRFC3339: row.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
	if row.SiteID.Valid {
		u := uuid.UUID(row.SiteID.Bytes)
		v.SiteID = u.String()
	}
	if row.MachineID.Valid {
		u := uuid.UUID(row.MachineID.Bytes)
		v.MachineID = u.String()
	}
	return v
}

func pgDateToString(d pgtype.Date) string {
	if !d.Valid {
		return ""
	}
	y, m, day := d.Time.Date()
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
}

func uuidToNullablePg(id uuid.UUID) pgtype.UUID {
	if id == uuid.Nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{Bytes: id, Valid: true}
}

func isPGUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
