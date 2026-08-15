package processalert

import (
	"github.com/avf/avf-vending-api/internal/app/alerts"
	"github.com/avf/avf-vending-api/internal/config"
	platformtelegram "github.com/avf/avf-vending-api/internal/platform/telegram"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// NewReporter builds a ServerErrorReporter for process-terminal failures.
// pool may be nil (emergency-only path).
func NewReporter(log *zap.Logger, cfg *config.Config, pool *pgxpool.Pool) *alerts.ServerErrorReporter {
	if cfg == nil {
		return nil
	}
	return alerts.NewServerErrorReporter(log, pool, platformtelegram.NewClient(platformtelegram.Config{
		BotToken: cfg.Telegram.ServerToken(),
		ChatID:   cfg.Telegram.ServerChatID(),
	}))
}

// ReportTerminal reports an unexpected process exit once.
func ReportTerminal(log *zap.Logger, cfg *config.Config, pool *pgxpool.Pool, service string, err error) {
	alerts.ReportProcessTerminalError(log, NewReporter(log, cfg, pool), service, err)
}
