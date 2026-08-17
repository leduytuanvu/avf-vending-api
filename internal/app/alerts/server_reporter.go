package alerts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/id"
	platformtelegram "github.com/avf/avf-vending-api/internal/platform/telegram"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// ServerAlert is a non-machine server-origin alert payload.
type ServerAlert struct {
	OccurrenceID  string
	Severity      string
	Code          string
	Title         string
	Service       string
	Operation     string
	TraceID       string
	CorrelationID string
	Detail        map[string]string
}

// ServerErrorReporter queues durable SERVER Telegram intents, with a bounded emergency fallback.
type ServerErrorReporter struct {
	log       *zap.Logger
	pool      *pgxpool.Pool
	emergency *platformtelegram.Client
	mu        sync.Mutex
	reporting bool // recursion guard
}

func NewServerErrorReporter(log *zap.Logger, pool *pgxpool.Pool, emergency *platformtelegram.Client) *ServerErrorReporter {
	return &ServerErrorReporter{log: log, pool: pool, emergency: emergency}
}

// Report queues a durable notification when the pool is available; otherwise uses emergency send.
func (r *ServerErrorReporter) Report(ctx context.Context, in ServerAlert) {
	if r == nil {
		return
	}
	r.mu.Lock()
	if r.reporting {
		r.mu.Unlock()
		return
	}
	r.reporting = true
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		r.reporting = false
		r.mu.Unlock()
	}()

	sev := NormalizeSeverity(in.Severity)
	if sev != "high" && sev != "critical" {
		return
	}
	occ := strings.TrimSpace(in.OccurrenceID)
	if occ == "" {
		occ = id.NewUUIDV7String()
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = strings.TrimSpace(in.Code)
	}
	if title == "" {
		title = "Server error"
	}

	payload, err := json.Marshal(map[string]any{
		"schema_version": 1,
		"source":         SourceServer,
		"occurrence_id":  occ,
		"severity":       sev,
		"code":           strings.TrimSpace(in.Code),
		"title":          title,
		"dedupe_key":     strings.TrimSpace(in.Code),
		"group_key":      strings.TrimSpace(in.Code),
		"service":        strings.TrimSpace(in.Service),
		"operation":      strings.TrimSpace(in.Operation),
		"trace_id":       strings.TrimSpace(in.TraceID),
		"correlation_id": strings.TrimSpace(in.CorrelationID),
		"detail":         sanitizeDetailMap(in.Detail),
	})
	if err != nil {
		r.emergencySend(ctx, title)
		return
	}

	if r.pool != nil {
		agg := uuid.Nil
		_, err := db.New(r.pool).InsertOutboxEventIdempotent(ctx, db.InsertOutboxEventIdempotentParams{
			Topic:          "notification.telegram",
			EventType:      "server.error.alert",
			Payload:        payload,
			AggregateType:  "server_alert",
			AggregateID:    agg,
			IdempotencyKey: pgtype.Text{String: TelegramServerIdempotencyKey(occ), Valid: true},
		})
		if err == nil || errors.Is(err, context.Canceled) {
			return
		}
		if r.log != nil {
			r.log.Warn("server_alert_outbox_failed", zap.Error(err))
		}
	}
	r.emergencySend(ctx, FormatServerEmergency(title, in))
}

func (r *ServerErrorReporter) emergencySend(ctx context.Context, text string) {
	if r.emergency == nil || !r.emergency.Enabled() {
		if r.log != nil {
			r.log.Error("server_alert_emergency_unavailable", zap.String("text", text))
		}
		return
	}
	ectx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := r.emergency.SendText(ectx, text); err != nil && r.log != nil {
		r.log.Error("server_alert_emergency_failed", zap.Error(err))
	}
}

// FormatServerEmergency builds a short emergency message.
func FormatServerEmergency(title string, in ServerAlert) string {
	return platformtelegram.BoundMessage(fmt.Sprintf("🚨 [SERVER][%s] %s\nService: %s\nOperation: %s\nOccurrence: %s",
		strings.ToUpper(NormalizeSeverity(in.Severity)),
		title,
		strings.TrimSpace(in.Service),
		strings.TrimSpace(in.Operation),
		strings.TrimSpace(in.OccurrenceID),
	))
}

func sanitizeDetailMap(in map[string]string) map[string]string {
	if in == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		nk := strings.ToLower(strings.ReplaceAll(k, "-", "_"))
		if strings.Contains(nk, "token") || strings.Contains(nk, "password") || strings.Contains(nk, "authorization") || strings.Contains(nk, "cookie") || strings.Contains(nk, "secret") {
			out[k] = "[REDACTED]"
			continue
		}
		out[k] = v
	}
	return out
}
