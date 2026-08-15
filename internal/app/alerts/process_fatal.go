package alerts

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ReportProcessTerminalError reports a process-level unexpected exit to SERVER Telegram.
// It is bounded (few seconds) and never blocks shutdown longer than that.
// Intended for unexpected runner failures — not healthy retry-loop noise.
func ReportProcessTerminalError(log *zap.Logger, reporter *ServerErrorReporter, service string, err error) {
	if err == nil || reporter == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	msg := strings.TrimSpace(err.Error())
	if len(msg) > 500 {
		msg = msg[:500]
	}
	if log != nil {
		log.Error("process_terminal_error",
			zap.String("service", service),
			zap.Error(err),
		)
	}
	reporter.Report(ctx, ServerAlert{
		OccurrenceID: fmt.Sprintf("process_fatal:%s:%s", strings.TrimSpace(service), uuid.NewString()),
		Severity:     "critical",
		Code:         "process_terminal_failure",
		Title:        "Process stopped with unexpected error",
		Service:      strings.TrimSpace(service),
		Operation:    "main",
		Detail: map[string]string{
			"error": msg,
		},
	})
}
