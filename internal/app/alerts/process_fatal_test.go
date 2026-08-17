package alerts_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/avf/avf-vending-api/internal/app/alerts"
	platformtelegram "github.com/avf/avf-vending-api/internal/platform/telegram"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestReportProcessTerminalError_InvokesEmergencySend(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer server.Close()

	client := platformtelegram.NewClient(platformtelegram.Config{
		BotToken: "fake-token",
		ChatID:   "1",
		APIBase:  server.URL,
	})
	reporter := alerts.NewServerErrorReporter(zap.NewNop(), nil, client)
	alerts.ReportProcessTerminalError(zap.NewNop(), reporter, "worker", errors.New("runner exploded"))
	require.Equal(t, int32(1), hits.Load())
}

func TestReportProcessTerminalError_NilReporterNoop(t *testing.T) {
	alerts.ReportProcessTerminalError(zap.NewNop(), nil, "api", errors.New("x"))
}
