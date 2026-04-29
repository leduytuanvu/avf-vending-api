package payments

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AdminService backs org-scoped payment operations APIs (webhooks audit, settlements, disputes, finance export).
type AdminService struct {
	q     *db.Queries
	pool  *pgxpool.Pool
	audit compliance.EnterpriseRecorder
}

// NewAdminService returns a payments admin service; audit may be nil (mutations skip audit — callers should wire real audit in production).
func NewAdminService(pool *pgxpool.Pool, q *db.Queries, audit compliance.EnterpriseRecorder) (*AdminService, error) {
	if pool == nil {
		return nil, errors.New("payments: nil pool")
	}
	if q == nil {
		return nil, errors.New("payments: nil queries")
	}
	return &AdminService{pool: pool, q: q, audit: audit}, nil
}

// WebhookEventItem is an admin list row for stored PSP webhooks.
type WebhookEventItem struct {
	ID               int64      `json:"id"`
	PaymentID        *string    `json:"paymentId,omitempty"`
	Provider         string     `json:"provider"`
	ProviderRef      *string    `json:"providerRef,omitempty"`
	ProviderEventID  *string    `json:"providerEventId,omitempty"`
	EventType        string     `json:"eventType"`
	ReceivedAt       time.Time  `json:"receivedAt"`
	SignatureValid   bool       `json:"signatureValid"`
	ValidationStatus string     `json:"validationStatus"`
	AppliedAt        *time.Time `json:"appliedAt,omitempty"`
	IngressStatus    string     `json:"ingressStatus"`
	IngressError     *string    `json:"ingressError,omitempty"`
	Payload          []byte     `json:"payload"`
}

// WebhookEventsListResponse paginates webhook events.
type WebhookEventsListResponse struct {
	Items []WebhookEventItem `json:"items"`
	Total int64              `json:"total"`
}

// SettlementListItem represents one imported provider settlement report.
type SettlementListItem struct {
	ID                   string    `json:"id"`
	Provider             string    `json:"provider"`
	ProviderSettlementID string    `json:"providerSettlementId"`
	GrossAmountMinor     int64     `json:"grossAmountMinor"`
	FeeAmountMinor       int64     `json:"feeAmountMinor"`
	NetAmountMinor       int64     `json:"netAmountMinor"`
	Currency             string    `json:"currency"`
	SettlementDate       string    `json:"settlementDate"`
	TransactionRefs      []byte    `json:"transactionRefs"`
	Status               string    `json:"status"`
	CreatedAt            time.Time `json:"createdAt"`
}

// SettlementsListResponse paginates settlements.
type SettlementsListResponse struct {
	Items []SettlementListItem `json:"items"`
	Total int64                `json:"total"`
}

// SettlementImportItem is one settlement line in a bulk import.
type SettlementImportItem struct {
	ProviderSettlementID string   `json:"providerSettlementId"`
	GrossAmountMinor     int64    `json:"grossAmountMinor"`
	FeeAmountMinor       int64    `json:"feeAmountMinor"`
	NetAmountMinor       int64    `json:"netAmountMinor"`
	Currency             string   `json:"currency"`
	SettlementDate       string   `json:"settlementDate"` // YYYY-MM-DD
	TransactionRefs      []string `json:"transactionRefs"`
}

// SettlementImportResult returns per-row outcomes.
type SettlementImportResult struct {
	Settlement SettlementListItem `json:"settlement"`
	Matched    bool               `json:"matched"`
}

// SettlementImportResponse is the API body for POST import.
type SettlementImportResponse struct {
	Results []SettlementImportResult `json:"results"`
}

// DisputeListItem is an admin view for a dispute/chargeback row.
type DisputeListItem struct {
	ID                string     `json:"id"`
	Provider          string     `json:"provider"`
	ProviderDisputeID string     `json:"providerDisputeId"`
	PaymentID         *string    `json:"paymentId,omitempty"`
	OrderID           *string    `json:"orderId,omitempty"`
	AmountMinor       int64      `json:"amountMinor"`
	Currency          string     `json:"currency"`
	Reason            *string    `json:"reason,omitempty"`
	Status            string     `json:"status"`
	OpenedAt          time.Time  `json:"openedAt"`
	ResolvedAt        *time.Time `json:"resolvedAt,omitempty"`
	ResolutionNote    *string    `json:"resolutionNote,omitempty"`
}

type DisputesListResponse struct {
	Items []DisputeListItem `json:"items"`
	Total int64             `json:"total"`
}

type ResolveDisputeInput struct {
	OrganizationID uuid.UUID
	DisputeID      uuid.UUID
	Status         string
	Note           string
	ResolvedBy     uuid.UUID
}

// ReconciliationStalePaidOrderRow is an order stuck in paid/vending with a captured PSP row past SLA.
type ReconciliationStalePaidOrderRow struct {
	OrderID         string    `json:"orderId"`
	MachineID       string    `json:"machineId"`
	OrderStatus     string    `json:"orderStatus"`
	OrderUpdatedAt  time.Time `json:"orderUpdatedAt"`
	PaymentID       string    `json:"paymentId"`
	PaymentState    string    `json:"paymentState"`
	Provider        string    `json:"provider"`
	AmountMinor     int64     `json:"amountMinor"`
	Currency        string    `json:"currency"`
	PaymentCaptured bool      `json:"paymentCaptured"`
}

// ReconciliationProviderAheadRow is a stored PSP webhook body claiming capture while the payment row remains pre-capture.
type ReconciliationProviderAheadRow struct {
	ProviderEventID int64     `json:"providerEventId"`
	PaymentID       *string   `json:"paymentId,omitempty"`
	OrderID         string    `json:"orderId"`
	Provider        string    `json:"provider"`
	PaymentState    string    `json:"paymentState"`
	ReceivedAt      time.Time `json:"receivedAt"`
}

// ReconciliationMissingEvidenceRow is a captured PSP ledger row without applied provider ingress evidence.
type ReconciliationMissingEvidenceRow struct {
	PaymentID   string    `json:"paymentId"`
	OrderID     string    `json:"orderId"`
	Provider    string    `json:"provider"`
	State       string    `json:"state"`
	UpdatedAt   time.Time `json:"updatedAt"`
	AmountMinor int64     `json:"amountMinor"`
	Currency    string    `json:"currency"`
}

// ReconciliationWebhookAmountMismatchRow is an applied webhook evidencing a different amount/currency than payments.
type ReconciliationWebhookAmountMismatchRow struct {
	ProviderEventID    int64     `json:"providerEventId"`
	PaymentID          string    `json:"paymentId"`
	Provider           string    `json:"provider"`
	WebhookAmountMinor *int64    `json:"webhookAmountMinor,omitempty"`
	PaymentAmountMinor int64     `json:"paymentAmountMinor"`
	WebhookCurrency    string    `json:"webhookCurrency,omitempty"`
	PaymentCurrency    string    `json:"paymentCurrency"`
	ReceivedAt         time.Time `json:"receivedAt"`
}

// ReconciliationDriftReport lists reconciliation slices for operator triage.
type ReconciliationDriftReport struct {
	StalePaidIncomplete                   []ReconciliationStalePaidOrderRow        `json:"stalePaidOrdersNotCompleted"`
	ProviderCapturedVsLocalPending        []ReconciliationProviderAheadRow         `json:"providerCapturedVsLocalPending"`
	LocalCapturedMissingProviderAudit     []ReconciliationMissingEvidenceRow       `json:"localCapturedMissingProviderEvidence"`
	AppliedWebhookVsPaymentAmountMismatch []ReconciliationWebhookAmountMismatchRow `json:"appliedWebhookVsPaymentAmountMismatch"`
}

// ListWebhookEvents returns org-scoped webhook ingress rows (includes join via payments when organization_id denorm is null).
func (s *AdminService) ListWebhookEvents(ctx context.Context, organizationID uuid.UUID, limit, offset int32) (*WebhookEventsListResponse, error) {
	if s == nil || s.q == nil {
		return nil, errors.New("payments: nil service")
	}
	if organizationID == uuid.Nil {
		return nil, errors.New("payments: organization required")
	}
	total, err := s.q.CountPaymentProviderEventsForOrgAdmin(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	rows, err := s.q.ListPaymentProviderEventsForOrgAdmin(ctx, db.ListPaymentProviderEventsForOrgAdminParams{
		Limit:          limit,
		Offset:         offset,
		OrganizationID: organizationID,
	})
	if err != nil {
		return nil, err
	}
	out := &WebhookEventsListResponse{Total: total, Items: make([]WebhookEventItem, 0, len(rows))}
	for _, r := range rows {
		item := WebhookEventItem{
			ID:               r.ID,
			Provider:         r.Provider,
			EventType:        r.EventType,
			ReceivedAt:       r.ReceivedAt.UTC(),
			SignatureValid:   r.SignatureValid,
			ValidationStatus: r.ValidationStatus,
			IngressStatus:    r.IngressStatus,
			Payload:          r.Payload,
		}
		if r.PaymentID.Valid {
			payStr := uuid.UUID(r.PaymentID.Bytes).String()
			item.PaymentID = &payStr
		}
		item.ProviderRef = textPtr(r.ProviderRef)
		item.ProviderEventID = textPtr(r.WebhookEventID)
		if r.AppliedAt.Valid {
			t := r.AppliedAt.Time.UTC()
			item.AppliedAt = &t
		}
		item.IngressError = textPtr(r.IngressError)
		out.Items = append(out.Items, item)
	}
	return out, nil
}

func textPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	s := t.String
	return &s
}

// ListSettlements lists imported PSP settlement rows for an organization.
func (s *AdminService) ListSettlements(ctx context.Context, organizationID uuid.UUID, limit, offset int32) (*SettlementsListResponse, error) {
	if s == nil || s.q == nil {
		return nil, errors.New("payments: nil service")
	}
	if organizationID == uuid.Nil {
		return nil, errors.New("payments: organization required")
	}
	total, err := s.q.CountPaymentProviderSettlementsForOrg(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	rows, err := s.q.ListPaymentProviderSettlementsForOrg(ctx, db.ListPaymentProviderSettlementsForOrgParams{
		OrganizationID: organizationID,
		Limit:          limit,
		Offset:         offset,
	})
	if err != nil {
		return nil, err
	}
	resp := &SettlementsListResponse{Total: total, Items: make([]SettlementListItem, 0, len(rows))}
	for _, r := range rows {
		resp.Items = append(resp.Items, SettlementListItem{
			ID:                   r.ID.String(),
			Provider:             r.Provider,
			ProviderSettlementID: r.ProviderSettlementID,
			GrossAmountMinor:     r.GrossAmountMinor,
			FeeAmountMinor:       r.FeeAmountMinor,
			NetAmountMinor:       r.NetAmountMinor,
			Currency:             strings.ToUpper(strings.TrimSpace(r.Currency)),
			SettlementDate:       r.SettlementDate.Time.Format("2006-01-02"),
			TransactionRefs:      r.TransactionRefs,
			Status:               r.Status,
			CreatedAt:            r.CreatedAt.UTC(),
		})
	}
	return resp, nil
}

// ImportSettlements upserts settlement rows and opens reconciliation cases when referenced payment totals disagree with gross_amount_minor.
func (s *AdminService) ImportSettlements(ctx context.Context, organizationID uuid.UUID, provider string, items []SettlementImportItem) (*SettlementImportResponse, error) {
	if s == nil || s.q == nil || s.pool == nil {
		return nil, errors.New("payments: nil service")
	}
	if organizationID == uuid.Nil {
		return nil, errors.New("payments: organization required")
	}
	prov := strings.TrimSpace(provider)
	if prov == "" {
		return nil, errors.New("payments: provider required")
	}
	out := &SettlementImportResponse{Results: make([]SettlementImportResult, 0, len(items))}

	for _, it := range items {
		sid := strings.TrimSpace(it.ProviderSettlementID)
		if sid == "" {
			return nil, errors.New("payments: providerSettlementId required")
		}
		cur := strings.ToUpper(strings.TrimSpace(it.Currency))
		if len(cur) != 3 {
			return nil, fmt.Errorf("payments: invalid currency for settlement %q", sid)
		}
		sd, err := time.Parse("2006-01-02", strings.TrimSpace(it.SettlementDate))
		if err != nil {
			return nil, fmt.Errorf("payments: settlementDate must be YYYY-MM-DD: %w", err)
		}
		refs := make([]string, 0, len(it.TransactionRefs))
		for _, r := range it.TransactionRefs {
			r = strings.TrimSpace(r)
			if r != "" {
				refs = append(refs, r)
			}
		}
		refsJSON, err := json.Marshal(refs)
		if err != nil {
			return nil, err
		}

		tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return nil, err
		}
		defer func() { _ = tx.Rollback(ctx) }()
		qtx := db.New(tx)

		row, err := qtx.UpsertPaymentProviderSettlement(ctx, db.UpsertPaymentProviderSettlementParams{
			OrganizationID:       organizationID,
			Provider:             prov,
			ProviderSettlementID: sid,
			GrossAmountMinor:     it.GrossAmountMinor,
			FeeAmountMinor:       it.FeeAmountMinor,
			NetAmountMinor:       it.NetAmountMinor,
			Currency:             cur,
			SettlementDate:       pgtype.Date{Time: sd, Valid: true},
			TransactionRefs:      refsJSON,
			Status:               "imported",
			Metadata:             []byte(`{}`),
		})
		if err != nil {
			return nil, err
		}

		matched := true
		if len(refs) > 0 {
			tot, err := qtx.SettlementReferencedPaymentsTotalForOrg(ctx, db.SettlementReferencedPaymentsTotalForOrgParams{
				OrganizationID: organizationID,
				Lower:          prov,
				Column3:        refs,
			})
			if err != nil {
				return nil, err
			}
			if tot.ReferencedTotalMinor != it.GrossAmountMinor {
				matched = false
				correlation := "settlement:" + prov + ":" + sid
				meta, _ := json.Marshal(map[string]any{
					"provider_settlement_id":   sid,
					"gross_amount_minor":       it.GrossAmountMinor,
					"referenced_total_minor":   tot.ReferencedTotalMinor,
					"referenced_payment_count": tot.ReferencedPaymentCount,
					"currency":                 cur,
				})
				md := compliance.SanitizeJSONBytes(meta)
				_, err = qtx.UpsertCommerceReconciliationCase(ctx, db.UpsertCommerceReconciliationCaseParams{
					OrganizationID: organizationID,
					CaseType:       "settlement_amount_mismatch",
					Severity:       "critical",
					Reason:         "settlement gross amount does not match referenced internal payments",
					Metadata:       md,
					CorrelationKey: pgtype.Text{String: correlation, Valid: true},
				})
				if err != nil {
					return nil, err
				}
				row, err = qtx.UpdatePaymentProviderSettlementStatusForOrg(ctx, db.UpdatePaymentProviderSettlementStatusForOrgParams{
					ID:             row.ID,
					OrganizationID: organizationID,
					Status:         "mismatch_flagged",
				})
				if err != nil {
					return nil, err
				}
			}
		} else if it.GrossAmountMinor != 0 {
			matched = false
			correlation := "settlement:" + prov + ":" + sid
			meta, _ := json.Marshal(map[string]any{
				"provider_settlement_id": sid,
				"gross_amount_minor":     it.GrossAmountMinor,
				"referenced_total_minor": 0,
				"note":                   "no transaction_refs on settlement import",
			})
			md := compliance.SanitizeJSONBytes(meta)
			_, err = qtx.UpsertCommerceReconciliationCase(ctx, db.UpsertCommerceReconciliationCaseParams{
				OrganizationID: organizationID,
				CaseType:       "settlement_amount_mismatch",
				Severity:       "warning",
				Reason:         "settlement import missing transaction references for non-zero gross amount",
				Metadata:       md,
				CorrelationKey: pgtype.Text{String: correlation, Valid: true},
			})
			if err != nil {
				return nil, err
			}
			row, err = qtx.UpdatePaymentProviderSettlementStatusForOrg(ctx, db.UpdatePaymentProviderSettlementStatusForOrgParams{
				ID:             row.ID,
				OrganizationID: organizationID,
				Status:         "mismatch_flagged",
			})
			if err != nil {
				return nil, err
			}
		}

		if matched && row.Status != "mismatch_flagged" {
			row, err = qtx.UpdatePaymentProviderSettlementStatusForOrg(ctx, db.UpdatePaymentProviderSettlementStatusForOrgParams{
				ID:             row.ID,
				OrganizationID: organizationID,
				Status:         "reconciled",
			})
			if err != nil {
				return nil, err
			}
		}

		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}

		out.Results = append(out.Results, SettlementImportResult{
			Matched: matched,
			Settlement: SettlementListItem{
				ID:                   row.ID.String(),
				Provider:             row.Provider,
				ProviderSettlementID: row.ProviderSettlementID,
				GrossAmountMinor:     row.GrossAmountMinor,
				FeeAmountMinor:       row.FeeAmountMinor,
				NetAmountMinor:       row.NetAmountMinor,
				Currency:             row.Currency,
				SettlementDate:       row.SettlementDate.Time.Format("2006-01-02"),
				TransactionRefs:      row.TransactionRefs,
				Status:               row.Status,
				CreatedAt:            row.CreatedAt.UTC(),
			},
		})
	}
	return out, nil
}

// ListDisputes lists payment disputes for an organization.
func (s *AdminService) ListDisputes(ctx context.Context, organizationID uuid.UUID, limit, offset int32) (*DisputesListResponse, error) {
	if s == nil || s.q == nil {
		return nil, errors.New("payments: nil service")
	}
	if organizationID == uuid.Nil {
		return nil, errors.New("payments: organization required")
	}
	total, err := s.q.CountPaymentDisputesForOrg(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	rows, err := s.q.ListPaymentDisputesForOrg(ctx, db.ListPaymentDisputesForOrgParams{
		OrganizationID: organizationID,
		Limit:          limit,
		Offset:         offset,
	})
	if err != nil {
		return nil, err
	}
	resp := &DisputesListResponse{Total: total, Items: make([]DisputeListItem, 0, len(rows))}
	for _, r := range rows {
		item := DisputeListItem{
			ID:                r.ID.String(),
			Provider:          r.Provider,
			ProviderDisputeID: r.ProviderDisputeID,
			AmountMinor:       r.AmountMinor,
			Currency:          r.Currency,
			Status:            r.Status,
			OpenedAt:          r.OpenedAt.UTC(),
		}
		item.PaymentID = uuidPtr(r.PaymentID)
		item.OrderID = uuidPtr(r.OrderID)
		item.Reason = textPtr(r.Reason)
		if r.ResolvedAt.Valid {
			t := r.ResolvedAt.Time.UTC()
			item.ResolvedAt = &t
		}
		item.ResolutionNote = textPtr(r.ResolutionNote)
		resp.Items = append(resp.Items, item)
	}
	return resp, nil
}

func uuidPtr(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	s := uuid.UUID(u.Bytes).String()
	return &s
}

// ResolveDispute applies a terminal resolution to a dispute (audit via HTTP layer).
func (s *AdminService) ResolveDispute(ctx context.Context, in ResolveDisputeInput) (DisputeListItem, error) {
	if s == nil || s.q == nil {
		return DisputeListItem{}, errors.New("payments: nil service")
	}
	st := strings.TrimSpace(strings.ToLower(in.Status))
	switch st {
	case "won", "lost", "closed":
	default:
		return DisputeListItem{}, errors.New("payments: status must be won, lost, or closed")
	}
	note := strings.TrimSpace(in.Note)
	row, err := s.q.ResolvePaymentDisputeForOrg(ctx, db.ResolvePaymentDisputeForOrgParams{
		ID:             in.DisputeID,
		OrganizationID: in.OrganizationID,
		Status:         st,
		ResolutionNote: pgtype.Text{String: note, Valid: note != ""},
		ResolvedBy:     pgtype.UUID{Bytes: in.ResolvedBy, Valid: in.ResolvedBy != uuid.Nil},
	})
	if err != nil {
		return DisputeListItem{}, err
	}
	item := DisputeListItem{
		ID:                row.ID.String(),
		Provider:          row.Provider,
		ProviderDisputeID: row.ProviderDisputeID,
		AmountMinor:       row.AmountMinor,
		Currency:          row.Currency,
		Status:            row.Status,
		OpenedAt:          row.OpenedAt.UTC(),
	}
	item.PaymentID = uuidPtr(row.PaymentID)
	item.OrderID = uuidPtr(row.OrderID)
	item.Reason = textPtr(row.Reason)
	if row.ResolvedAt.Valid {
		t := row.ResolvedAt.Time.UTC()
		item.ResolvedAt = &t
	}
	item.ResolutionNote = textPtr(row.ResolutionNote)
	return item, nil
}

// WriteFinanceExportCSV streams payment rows for finance (date bounds inclusive on created_at).
func (s *AdminService) WriteFinanceExportCSV(ctx context.Context, w io.Writer, organizationID uuid.UUID, from, to time.Time) error {
	if s == nil || s.q == nil {
		return errors.New("payments: nil service")
	}
	if organizationID == uuid.Nil {
		return errors.New("payments: organization required")
	}
	if !from.Before(to) && !from.Equal(to) {
		return errors.New("payments: from must be <= to")
	}
	rows, err := s.q.ListPaymentsFinanceExportForOrg(ctx, db.ListPaymentsFinanceExportForOrgParams{
		OrganizationID: organizationID,
		CreatedAt:      from,
		CreatedAt_2:    to,
	})
	if err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{
		"payment_id", "order_id", "machine_id", "provider", "state", "amount_minor", "currency",
		"reconciliation_status", "settlement_status", "created_at", "updated_at",
	})
	for _, p := range rows {
		_ = cw.Write([]string{
			p.ID.String(),
			p.OrderID.String(),
			p.MachineID.String(),
			p.Provider,
			p.State,
			strconv.FormatInt(p.AmountMinor, 10),
			p.Currency,
			p.ReconciliationStatus,
			p.SettlementStatus,
			p.CreatedAt.UTC().Format(time.RFC3339Nano),
			p.UpdatedAt.UTC().Format(time.RFC3339Nano),
		})
	}
	cw.Flush()
	return cw.Error()
}

// ListPaymentReconciliationDrift returns read-only drift probes (no mutations).
func (s *AdminService) ListPaymentReconciliationDrift(ctx context.Context, organizationID uuid.UUID, staleAfterSeconds int64, limit int32) (*ReconciliationDriftReport, error) {
	if s == nil || s.q == nil {
		return nil, errors.New("payments: nil service")
	}
	if organizationID == uuid.Nil {
		return nil, errors.New("payments: organization required")
	}
	if staleAfterSeconds <= 0 {
		staleAfterSeconds = int64(2 * time.Hour / time.Second)
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	staleRows, err := s.q.ListReconciliationStalePaidOrdersNotCompleted(ctx, db.ListReconciliationStalePaidOrdersNotCompletedParams{
		OrganizationID: organizationID,
		Column2:        staleAfterSeconds,
		Limit:          limit,
	})
	if err != nil {
		return nil, err
	}
	provRows, err := s.q.ListReconciliationProviderCapturedLocalPending(ctx, db.ListReconciliationProviderCapturedLocalPendingParams{
		OrganizationID: organizationID,
		Limit:          limit,
	})
	if err != nil {
		return nil, err
	}
	missRows, err := s.q.ListReconciliationLocalCapturedWithoutProviderEvidence(ctx, db.ListReconciliationLocalCapturedWithoutProviderEvidenceParams{
		OrganizationID: organizationID,
		Limit:          limit,
	})
	if err != nil {
		return nil, err
	}
	mismatchRows, err := s.q.ListReconciliationAppliedWebhookAmountMismatch(ctx, db.ListReconciliationAppliedWebhookAmountMismatchParams{
		OrganizationID: organizationID,
		Limit:          limit,
	})
	if err != nil {
		return nil, err
	}
	out := &ReconciliationDriftReport{
		StalePaidIncomplete:                   make([]ReconciliationStalePaidOrderRow, 0, len(staleRows)),
		ProviderCapturedVsLocalPending:        make([]ReconciliationProviderAheadRow, 0, len(provRows)),
		LocalCapturedMissingProviderAudit:     make([]ReconciliationMissingEvidenceRow, 0, len(missRows)),
		AppliedWebhookVsPaymentAmountMismatch: make([]ReconciliationWebhookAmountMismatchRow, 0, len(mismatchRows)),
	}
	for _, r := range staleRows {
		out.StalePaidIncomplete = append(out.StalePaidIncomplete, ReconciliationStalePaidOrderRow{
			OrderID:         r.OrderID.String(),
			MachineID:       r.MachineID.String(),
			OrderStatus:     r.OrderStatus,
			OrderUpdatedAt:  r.OrderUpdatedAt.UTC(),
			PaymentID:       r.PaymentID.String(),
			PaymentState:    r.PaymentState,
			Provider:        r.Provider,
			AmountMinor:     r.AmountMinor,
			Currency:        r.Currency,
			PaymentCaptured: r.PaymentState == "captured",
		})
	}
	for _, r := range provRows {
		row := ReconciliationProviderAheadRow{
			ProviderEventID: r.ProviderEventID,
			OrderID:         r.OrderID.String(),
			Provider:        r.Provider,
			PaymentState:    r.PaymentState,
			ReceivedAt:      r.ReceivedAt.UTC(),
		}
		if r.PaymentID.Valid {
			pid := uuid.UUID(r.PaymentID.Bytes)
			if pid != uuid.Nil {
				s := pid.String()
				row.PaymentID = &s
			}
		}
		out.ProviderCapturedVsLocalPending = append(out.ProviderCapturedVsLocalPending, row)
	}
	for _, r := range missRows {
		out.LocalCapturedMissingProviderAudit = append(out.LocalCapturedMissingProviderAudit, ReconciliationMissingEvidenceRow{
			PaymentID:   r.PaymentID.String(),
			OrderID:     r.OrderID.String(),
			Provider:    r.Provider,
			State:       r.State,
			UpdatedAt:   r.UpdatedAt.UTC(),
			AmountMinor: r.AmountMinor,
			Currency:    r.Currency,
		})
	}
	for _, r := range mismatchRows {
		row := ReconciliationWebhookAmountMismatchRow{
			ProviderEventID:    r.ProviderEventID,
			PaymentCurrency:    strings.ToUpper(strings.TrimSpace(r.PaymentCurrency)),
			PaymentAmountMinor: r.PaymentAmountMinor,
			Provider:           r.Provider,
			ReceivedAt:         r.ReceivedAt.UTC(),
		}
		if r.PaymentID.Valid {
			row.PaymentID = uuid.UUID(r.PaymentID.Bytes).String()
		}
		if r.WebhookAmountMinor.Valid {
			v := r.WebhookAmountMinor.Int64
			row.WebhookAmountMinor = &v
		}
		row.WebhookCurrency = strings.ToUpper(strings.TrimSpace(r.WebhookCurrency))
		out.AppliedWebhookVsPaymentAmountMismatch = append(out.AppliedWebhookVsPaymentAmountMismatch, row)
	}
	return out, nil
}
