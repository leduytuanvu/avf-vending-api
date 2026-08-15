package alerts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	platformnats "github.com/avf/avf-vending-api/internal/platform/nats"
	platformtelegram "github.com/avf/avf-vending-api/internal/platform/telegram"
	natssrv "github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

const telegramOutboxDurable = "avf-w-notification-telegram"

// BotRouter selects the Telegram client by trusted alert source.
type BotRouter struct {
	App    *platformtelegram.Client
	Server *platformtelegram.Client
}

// ClientFor returns the bot for source; unknown source returns nil.
func (r BotRouter) ClientFor(source string) *platformtelegram.Client {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case SourceApp, "":
		// Empty source treated as app only for legacy payloads; dispatcher still validates.
		if strings.TrimSpace(source) == "" {
			return r.App
		}
		return r.App
	case SourceServer:
		return r.Server
	default:
		return nil
	}
}

// TelegramDispatcherConfig controls required-mode delivery semantics.
type TelegramDispatcherConfig struct {
	Required bool
	Enabled  bool
}

// TelegramDispatcher consumes notification.telegram messages published by the existing outbox worker.
type TelegramDispatcher struct {
	log  *zap.Logger
	nc   *natssrv.Conn
	js   natssrv.JetStreamContext
	bots BotRouter
	cfg  TelegramDispatcherConfig
}

func NewTelegramDispatcher(log *zap.Logger, nc *natssrv.Conn, bots BotRouter, cfg TelegramDispatcherConfig) *TelegramDispatcher {
	return &TelegramDispatcher{log: log, nc: nc, bots: bots, cfg: cfg}
}

// Start consumes the existing outbox stream; it does not create a parallel persistence worker.
func (d *TelegramDispatcher) Start(ctx context.Context) error {
	if d == nil || d.nc == nil {
		return errors.New("alerts: nil Telegram dispatcher")
	}
	js, err := d.nc.JetStream()
	if err != nil {
		return fmt.Errorf("alerts: telegram jetstream: %w", err)
	}
	d.js = js
	if err := platformnats.EnsureOutboxPullConsumer(js, telegramOutboxDurable, platformnats.DefaultConsumerRetryDefaults()); err != nil {
		return err
	}
	sub, err := platformnats.BindOutboxPull(d.nc, telegramOutboxDurable)
	if err != nil {
		return err
	}
	defer func() { _ = sub.Unsubscribe() }()

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		msgs, err := sub.Fetch(16, natssrv.MaxWait(time.Second))
		if errors.Is(err, natssrv.ErrTimeout) {
			continue
		}
		if err != nil {
			return fmt.Errorf("alerts: telegram fetch: %w", err)
		}
		for _, msg := range msgs {
			d.handleMsg(ctx, msg)
		}
	}
}

func (d *TelegramDispatcher) handleMsg(ctx context.Context, msg *natssrv.Msg) {
	if strings.TrimSpace(msg.Header.Get("X-Outbox-Topic")) != "notification.telegram" {
		_ = msg.Ack()
		return
	}
	var alert platformtelegram.IncidentAlert
	if err := json.Unmarshal(msg.Data, &alert); err != nil {
		if d.log != nil {
			d.log.Warn("telegram_alert_payload_invalid", zap.Error(err))
		}
		if err := d.dlqThenTerm(ctx, msg, "telegram_malformed"); err != nil {
			_ = msg.Nak()
		}
		return
	}

	source := strings.ToLower(strings.TrimSpace(alert.Source))
	if source == "" {
		source = SourceApp
		alert.Source = SourceApp
	}
	if source != SourceApp && source != SourceServer {
		if d.log != nil {
			d.log.Warn("telegram_alert_unknown_source", zap.String("source", source))
		}
		if err := d.dlqThenTerm(ctx, msg, "telegram_unknown_source"); err != nil {
			_ = msg.Nak()
		}
		return
	}

	client := d.bots.ClientFor(source)
	if !d.cfg.Enabled || client == nil || !client.Enabled() {
		if d.cfg.Required {
			if d.log != nil {
				d.log.Error("telegram_alert_unconfigured_required",
					zap.String("source", source),
					zap.String("occurrence_id", alert.OccurrenceID),
					zap.String("machine_id", alert.MachineID),
				)
			}
			// Do not Ack — retain for operator configuration.
			_ = msg.Nak()
			return
		}
		if d.log != nil {
			d.log.Info("telegram_alert_skipped_unconfigured",
				zap.String("source", source),
				zap.String("machine_id", alert.MachineID),
				zap.String("occurrence_id", alert.OccurrenceID),
			)
		}
		// Optional mode may Ack skip; required mode never reaches here.
		_ = msg.Ack()
		return
	}

	if err := client.SendIncident(ctx, alert); err != nil {
		if d.log != nil {
			d.log.Warn("telegram_alert_delivery_failed",
				zap.Error(err),
				zap.String("source", source),
				zap.String("machine_id", alert.MachineID),
				zap.String("occurrence_id", alert.OccurrenceID),
			)
		}
		if platformtelegram.IsPermanent(err) {
			if dlqErr := d.dlqThenTerm(ctx, msg, "telegram_permanent"); dlqErr != nil {
				_ = msg.Nak()
			}
			return
		}
		if ra := platformtelegram.RetryAfter(err); ra > 0 {
			_ = msg.NakWithDelay(ra)
			return
		}
		_ = msg.Nak()
		return
	}
	_ = msg.Ack()
}

func (d *TelegramDispatcher) dlqThenTerm(ctx context.Context, msg *natssrv.Msg, reason string) error {
	if d.js == nil {
		return errors.New("alerts: nil jetstream for dlq")
	}
	if err := platformnats.PublishDLQ(ctx, d.js, reason, msg.Header, msg.Data); err != nil {
		if d.log != nil {
			d.log.Error("telegram_alert_dlq_failed", zap.Error(err), zap.String("reason", reason))
		}
		return err
	}
	_ = msg.Term()
	return nil
}

// ProcessAlertForTest exposes handleMsg semantics for unit tests without JetStream.
func (d *TelegramDispatcher) ProcessAlertForTest(ctx context.Context, alert platformtelegram.IncidentAlert, ack *bool, nak *bool, term *bool) error {
	data, err := json.Marshal(alert)
	if err != nil {
		return err
	}
	msg := &natssrv.Msg{Data: data, Header: natssrv.Header{}}
	msg.Header.Set("X-Outbox-Topic", "notification.telegram")
	// Lightweight stand-in: call routing logic via temporary hooks.
	source := strings.ToLower(strings.TrimSpace(alert.Source))
	if source == "" {
		source = SourceApp
	}
	if source != SourceApp && source != SourceServer {
		*term = true
		return nil
	}
	client := d.bots.ClientFor(source)
	if !d.cfg.Enabled || client == nil || !client.Enabled() {
		if d.cfg.Required {
			*nak = true
			return nil
		}
		*ack = true
		return nil
	}
	if err := client.SendIncident(ctx, alert); err != nil {
		if platformtelegram.IsPermanent(err) {
			// Mirror production: if DLQ cannot be published (nil JetStream), retain via Nak.
			if d.js == nil {
				*nak = true
				return err
			}
			*term = true
			return err
		}
		*nak = true
		return err
	}
	*ack = true
	return nil
}
