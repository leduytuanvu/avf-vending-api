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
	const serverChat = "-1000000000002"
	var hits atomic.Int32
	var gotChat string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		var body struct {
			ChatID string `json:"chat_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotChat = body.ChatID
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer server.Close()

	client := platformtelegram.NewClient(platformtelegram.Config{
		BotToken: "fake-token",
		ChatID:   serverChat,
		APIBase:  server.URL,
	})
	reporter := alerts.NewServerErrorReporter(zap.NewNop(), nil, client)
	alerts.ReportProcessTerminalError(zap.NewNop(), reporter, "worker", errors.New("runner exploded"))
	require.Equal(t, int32(1), hits.Load())
	require.Equal(t, serverChat, gotChat)
}

func TestReportProcessTerminalError_NilReporterNoop(t *testing.T) {
	alerts.ReportProcessTerminalError(zap.NewNop(), nil, "api", errors.New("x"))
}
