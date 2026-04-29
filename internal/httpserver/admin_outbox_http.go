package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	appoutbox "github.com/avf/avf-vending-api/internal/app/outbox"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func MountAdminOutboxOpsRoutes(r chi.Router, app *api.HTTPApplication, cfg *config.Config, writeRL func(http.Handler) http.Handler) {
	if app == nil || app.TelemetryStore == nil {
		return
	}
	if writeRL == nil {
		writeRL = func(h http.Handler) http.Handler { return h }
	}
	pool := app.TelemetryStore.Pool()
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyRole(auth.RolePlatformAdmin))
		r.Get("/ops/outbox", getAdminOutboxOps(pool))
		r.With(writeRL).Post("/ops/outbox/{outboxId}/retry", postAdminOutboxReplay(pool, app, cfg, "outboxId"))
		r.Get("/ops/retention", getAdminRetentionOps(pool))
	})
}

func MountAdminSystemOutboxRoutes(r chi.Router, app *api.HTTPApplication, cfg *config.Config, writeRL func(http.Handler) http.Handler) {
	if app == nil || app.TelemetryStore == nil {
		return
	}
	if writeRL == nil {
		writeRL = func(h http.Handler) http.Handler { return h }
	}
	pool := app.TelemetryStore.Pool()
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyRole(auth.RolePlatformAdmin))
		r.Get("/system/outbox/stats", getAdminSystemOutboxStats(pool))
		r.Get("/system/outbox", getAdminSystemOutboxList(pool))
		r.Get("/system/outbox/{eventId}", getAdminSystemOutboxGet(pool))
		r.With(writeRL).Post("/system/outbox/{eventId}/replay", postAdminOutboxReplay(pool, app, cfg, "eventId"))
		r.With(writeRL).Post("/system/outbox/{eventId}/mark-dlq", postAdminSystemOutboxMarkDLQ(pool, app, cfg))
	})
}

func buildAdminOutboxEnvelope(ctx context.Context, pool *pgxpool.Pool, limit, offset int32) (V1AdminOutboxOpsEnvelope, error) {
	repo := postgres.NewOutboxRepository(pool)
	stats, err := repo.GetOutboxPipelineStats(ctx)
	if err != nil {
		return V1AdminOutboxOpsEnvelope{}, err
	}
	rows, err := db.New(pool).ListOutboxOpsRows(ctx, db.ListOutboxOpsRowsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return V1AdminOutboxOpsEnvelope{}, err
	}
	var oldest *string
	if stats.OldestPendingCreatedAt != nil {
		oldest = formatAPITimeRFC3339NanoPtr(stats.OldestPendingCreatedAt)
	}
	out := V1AdminOutboxOpsEnvelope{
		Stats: V1AdminOutboxPipelineStats{
			PendingTotal:           stats.PendingTotal,
			PendingDueNow:          stats.PendingDueNow,
			DeadLetteredTotal:      stats.DeadLetteredTotal,
			PublishingLeasedTotal:  stats.PublishingLeasedTotal,
			MaxPendingAttempts:     stats.MaxPendingAttempts,
			OldestPendingCreatedAt: oldest,
		},
		Rows: make([]V1AdminOutboxRow, 0, len(rows)),
		Meta: V1CollectionListMeta{
			Limit:    limit,
			Offset:   offset,
			Returned: len(rows),
		},
	}
	for _, row := range rows {
		out.Rows = append(out.Rows, mapDBOutboxOpsRow(row))
	}
	return out, nil
}

func getAdminOutboxOps(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, offset, err := parseAdminLimitOffset(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_pagination", err.Error())
			return
		}
		out, err := buildAdminOutboxEnvelope(r.Context(), pool, limit, offset)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "outbox_ops_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func getAdminSystemOutboxList(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, offset, err := parseAdminLimitOffset(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_pagination", err.Error())
			return
		}
		out, err := buildAdminOutboxEnvelope(r.Context(), pool, limit, offset)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "outbox_ops_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func getAdminSystemOutboxStats(pool *pgxpool.Pool) http.HandlerFunc {
	repo := postgres.NewOutboxRepository(pool)
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := repo.GetOutboxPipelineStats(r.Context())
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "outbox_stats_failed", err.Error())
			return
		}
		var oldest *string
		if stats.OldestPendingCreatedAt != nil {
			oldest = formatAPITimeRFC3339NanoPtr(stats.OldestPendingCreatedAt)
		}
		writeJSON(w, http.StatusOK, V1AdminOutboxStatsEnvelope{
			Stats: V1AdminOutboxPipelineStats{
				PendingTotal:           stats.PendingTotal,
				PendingDueNow:          stats.PendingDueNow,
				DeadLetteredTotal:      stats.DeadLetteredTotal,
				PublishingLeasedTotal:  stats.PublishingLeasedTotal,
				MaxPendingAttempts:     stats.MaxPendingAttempts,
				OldestPendingCreatedAt: oldest,
			},
		})
	}
}

func getAdminSystemOutboxGet(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseAdminOutboxPathID(w, r, "eventId")
		if !ok {
			return
		}
		row, err := db.New(pool).AdminGetOutboxEventByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "outbox event not found")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "outbox_get_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, mapDBOutboxOpsRow(row))
	}
}

type adminOutboxMarkDLQBody struct {
	Note string `json:"note"`
}

func outboxEligibleForReplay(row db.OutboxEvent) bool {
	if row.PublishedAt.Valid {
		return false
	}
	return row.Status == "dead_letter" || row.DeadLetteredAt.Valid
}

func outboxEligibleForManualDLQ(row db.OutboxEvent) bool {
	if row.PublishedAt.Valid {
		return false
	}
	if row.DeadLetteredAt.Valid {
		return false
	}
	switch row.Status {
	case "published", "dead_letter":
		return false
	default:
		return true
	}
}

func resolvePlatformOutboxAuditOrganization(ctx context.Context, cfg *config.Config, rowOrg pgtype.UUID) (uuid.UUID, error) {
	if rowOrg.Valid {
		return uuid.UUID(rowOrg.Bytes), nil
	}
	if p, ok := auth.PrincipalFromContext(ctx); ok && p.OrganizationID != uuid.Nil {
		return p.OrganizationID, nil
	}
	if cfg != nil && cfg.PlatformAuditOrganizationID != uuid.Nil {
		return cfg.PlatformAuditOrganizationID, nil
	}
	return uuid.Nil, errors.New("platform audit organization unresolved: set PLATFORM_AUDIT_ORGANIZATION_ID, use org-scoped outbox rows, or authenticate with an organization-scoped principal")
}

func postAdminOutboxReplay(pool *pgxpool.Pool, app *api.HTTPApplication, cfg *config.Config, param string) http.HandlerFunc {
	svc := appoutbox.NewAdminService(pool)
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseAdminOutboxPathID(w, r, param)
		if !ok {
			return
		}
		if svc == nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "outbox admin service not configured")
			return
		}
		before, err := svc.GetByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "outbox event not found")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "outbox_get_failed", err.Error())
			return
		}
		var rec compliance.EnterpriseAuditRecord
		var auditSvc compliance.EnterpriseRecorder
		if outboxEligibleForReplay(before) {
			if app == nil || app.EnterpriseAudit == nil {
				writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "enterprise audit not configured")
				return
			}
			auditOrg, err := resolvePlatformOutboxAuditOrganization(r.Context(), cfg, before.OrganizationID)
			if err != nil {
				writeAPIError(w, r.Context(), http.StatusServiceUnavailable, "platform_audit_org_unresolved", err.Error())
				return
			}
			rec = buildOutboxAdminAuditRecord(r.Context(), auditOrg, id, compliance.ActionAdminPlatformOutboxReplay, map[string]any{
				"topic": before.Topic,
			})
			auditSvc = app.EnterpriseAudit
		}
		n, err := svc.ReplayDeadLetterTx(r.Context(), id, auditSvc, rec)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "outbox_retry_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, V1AdminOutboxRetryEnvelope{Retried: n > 0})
	}
}

func postAdminSystemOutboxMarkDLQ(pool *pgxpool.Pool, app *api.HTTPApplication, cfg *config.Config) http.HandlerFunc {
	svc := appoutbox.NewAdminService(pool)
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseAdminOutboxPathID(w, r, "eventId")
		if !ok {
			return
		}
		if svc == nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "outbox admin service not configured")
			return
		}
		before, err := svc.GetByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "outbox event not found")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "outbox_get_failed", err.Error())
			return
		}
		if !outboxEligibleForManualDLQ(before) {
			writeAPIError(w, r.Context(), http.StatusConflict, "nothing_to_mark", "row is not eligible for manual DLQ (already terminal or published)")
			return
		}
		if app == nil || app.EnterpriseAudit == nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "enterprise audit not configured")
			return
		}
		auditOrg, err := resolvePlatformOutboxAuditOrganization(r.Context(), cfg, before.OrganizationID)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusServiceUnavailable, "platform_audit_org_unresolved", err.Error())
			return
		}
		note := ""
		if r.Body != nil {
			var body adminOutboxMarkDLQBody
			_ = json.NewDecoder(r.Body).Decode(&body)
			note = body.Note
		}
		rec := buildOutboxAdminAuditRecord(r.Context(), auditOrg, id, compliance.ActionAdminPlatformOutboxMarkDLQ, map[string]any{
			"topic": before.Topic,
			"note":  note,
		})
		n, err := svc.MarkManualDLQTx(r.Context(), id, note, app.EnterpriseAudit, rec)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "outbox_mark_dlq_failed", err.Error())
			return
		}
		if n == 0 {
			writeAPIError(w, r.Context(), http.StatusConflict, "nothing_to_mark", "row is not eligible for manual DLQ (already terminal or published)")
			return
		}
		writeJSON(w, http.StatusOK, V1AdminOutboxMarkDLQEnvelope{Marked: true})
	}
}

func buildOutboxAdminAuditRecord(ctx context.Context, auditOrg uuid.UUID, id int64, action string, meta map[string]any) compliance.EnterpriseAuditRecord {
	md, _ := json.Marshal(meta)
	rid := strconv.FormatInt(id, 10)
	at, aid := compliance.ActorUser, ""
	if p, ok := auth.PrincipalFromContext(ctx); ok {
		at, aid = p.Actor()
	}
	return compliance.EnterpriseAuditRecord{
		OrganizationID: auditOrg,
		ActorType:      at,
		ActorID:        stringPtrOrNil(aid),
		Action:         action,
		ResourceType:   "outbox_events",
		ResourceID:     &rid,
		Metadata:       md,
	}
}

func parseAdminOutboxPathID(w http.ResponseWriter, r *http.Request, key string) (int64, bool) {
	raw := chi.URLParam(r, key)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_outbox_id", key+" must be a positive integer")
		return 0, false
	}
	return id, true
}

func normalizeOutboxAPIStatus(dbStatus string) string {
	if dbStatus == "dead_letter" {
		return "dlq"
	}
	return dbStatus
}

func mapDBOutboxOpsRow(row db.OutboxEvent) V1AdminOutboxRow {
	var payload json.RawMessage
	if len(row.Payload) > 0 {
		payload = json.RawMessage(row.Payload)
	}
	next := pgTzToAPIPtr(row.NextPublishAfter)
	return V1AdminOutboxRow{
		ID:                   row.ID,
		OrganizationID:       uuidPtrFromPgUUID(row.OrganizationID),
		Topic:                row.Topic,
		EventType:            row.EventType,
		Payload:              payload,
		AggregateType:        row.AggregateType,
		AggregateID:          row.AggregateID.String(),
		IdempotencyKey:       textFromPgText(row.IdempotencyKey),
		CreatedAt:            formatAPITimeRFC3339Nano(row.CreatedAt),
		PublishedAt:          pgTzToAPIPtr(row.PublishedAt),
		PublishAttemptCount:  row.PublishAttemptCount,
		Attempts:             row.PublishAttemptCount,
		MaxAttempts:          row.MaxPublishAttempts,
		LastPublishError:     textFromPgText(row.LastPublishError),
		LastPublishAttemptAt: pgTzToAPIPtr(row.LastPublishAttemptAt),
		NextPublishAfter:     next,
		NextAttemptAt:        next,
		DeadLetteredAt:       pgTzToAPIPtr(row.DeadLetteredAt),
		Status:               normalizeOutboxAPIStatus(row.Status),
		LockedBy:             textFromPgText(row.LockedBy),
		LockedUntil:          pgTzToAPIPtr(row.LockedUntil),
		UpdatedAt:            formatAPITimeRFC3339Nano(row.UpdatedAt),
	}
}

func getAdminRetentionOps(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := postgres.GetEnterpriseRetentionStatus(r.Context(), pool)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "retention_status_failed", err.Error())
			return
		}
		now := time.Now().UTC()
		out := V1AdminRetentionOpsEnvelope{
			Tables: make([]V1AdminRetentionTableStatus, 0, len(rows)),
		}
		for _, row := range rows {
			item := V1AdminRetentionTableStatus{
				TableName: row.TableName,
				TotalRows: row.TotalRows,
			}
			if row.OldestRecordAt != nil {
				item.OldestRecordAt = formatAPITimeRFC3339NanoPtr(row.OldestRecordAt)
				ageDays := int64(now.Sub(row.OldestRecordAt.UTC()).Hours() / 24)
				item.OldestRecordAgeDays = &ageDays
			}
			out.Tables = append(out.Tables, item)
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func pgTzToAPIPtr(t pgtype.Timestamptz) *string {
	if !t.Valid {
		return nil
	}
	s := formatAPITimeRFC3339Nano(t.Time.UTC())
	return &s
}
