package alerts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	platformtelegram "github.com/avf/avf-vending-api/internal/platform/telegram"
	"github.com/stretchr/testify/require"
)

func TestDispatcherRequiredUnconfiguredNaks(t *testing.T) {
	d := NewTelegramDispatcher(nil, nil, BotRouter{}, TelegramDispatcherConfig{Required: true, Enabled: true})
	var ack, nak, term bool
	err := d.ProcessAlertForTest(context.Background(), platformtelegram.IncidentAlert{
		Source: SourceApp, Severity: "high", Code: "x", Title: "t", OccurrenceID: "o1",
	}, &ack, &nak, &term)
	require.NoError(t, err)
	require.False(t, ack)
	require.True(t, nak)
	require.False(t, term)
}

func TestDispatcherRoutesAppAndServerBots(t *testing.T) {
	var appHits, serverHits int
	appSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appHits++
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer appSrv.Close()
	serverSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverHits++
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer serverSrv.Close()

	d := NewTelegramDispatcher(nil, nil, BotRouter{
		App:    platformtelegram.NewClient(platformtelegram.Config{BotToken: "a", ChatID: "c", APIBase: appSrv.URL}),
		Server: platformtelegram.NewClient(platformtelegram.Config{BotToken: "s", ChatID: "c", APIBase: serverSrv.URL}),
	}, TelegramDispatcherConfig{Enabled: true, Required: true})

	var ack, nak, term bool
	require.NoError(t, d.ProcessAlertForTest(context.Background(), platformtelegram.IncidentAlert{
		Source: SourceApp, Severity: "high", Code: "incident_anr", Title: "anr", OccurrenceID: "1",
	}, &ack, &nak, &term))
	require.True(t, ack)
	require.Equal(t, 1, appHits)
	require.Equal(t, 0, serverHits)

	ack, nak, term = false, false, false
	require.NoError(t, d.ProcessAlertForTest(context.Background(), platformtelegram.IncidentAlert{
		Source: SourceServer, Severity: "critical", Code: "http_500", Title: "boom", OccurrenceID: "2",
	}, &ack, &nak, &term))
	require.True(t, ack)
	require.Equal(t, 1, serverHits)
}

func TestDispatcherUnknownSourceTerms(t *testing.T) {
	d := NewTelegramDispatcher(nil, nil, BotRouter{
		App: platformtelegram.NewClient(platformtelegram.Config{BotToken: "a", ChatID: "c"}),
	}, TelegramDispatcherConfig{Enabled: true, Required: true})
	var ack, nak, term bool
	_ = d.ProcessAlertForTest(context.Background(), platformtelegram.IncidentAlert{
		Source: "evil", Severity: "high", Code: "x", Title: "t",
	}, &ack, &nak, &term)
	require.True(t, term)
	require.False(t, ack)
}

func TestDispatcherPermanentWithoutDLQRetainsViaNak(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error_code": 400, "description": "chat not found"})
	}))
	defer srv.Close()
	d := NewTelegramDispatcher(nil, nil, BotRouter{
		App: platformtelegram.NewClient(platformtelegram.Config{BotToken: "a", ChatID: "c", APIBase: srv.URL}),
	}, TelegramDispatcherConfig{Enabled: true, Required: true})
	var ack, nak, term bool
	_ = d.ProcessAlertForTest(context.Background(), platformtelegram.IncidentAlert{
		Source: SourceApp, Severity: "high", Code: "x", Title: "t", OccurrenceID: "o1",
	}, &ack, &nak, &term)
	require.True(t, nak)
	require.False(t, ack)
	require.False(t, term)
}

func TestDispatcherRetryable5xxNaks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	d := NewTelegramDispatcher(nil, nil, BotRouter{
		App: platformtelegram.NewClient(platformtelegram.Config{BotToken: "a", ChatID: "c", APIBase: srv.URL}),
	}, TelegramDispatcherConfig{Enabled: true, Required: true})
	var ack, nak, term bool
	err := d.ProcessAlertForTest(context.Background(), platformtelegram.IncidentAlert{
		Source: SourceApp, Severity: "high", Code: "x", Title: "t", OccurrenceID: "o1",
	}, &ack, &nak, &term)
	require.Error(t, err)
	require.True(t, platformtelegram.IsRetryable(err))
	require.True(t, nak)
	require.False(t, ack)
	require.False(t, term)
}

func TestDispatcherSourceCannotBeDeviceChosenServer(t *testing.T) {
	// Trusted source field on alert routes bots; unknown/evil never falls back to APP when required.
	d := NewTelegramDispatcher(nil, nil, BotRouter{
		App:    platformtelegram.NewClient(platformtelegram.Config{BotToken: "a", ChatID: "c"}),
		Server: platformtelegram.NewClient(platformtelegram.Config{BotToken: "s", ChatID: "c"}),
	}, TelegramDispatcherConfig{Enabled: true, Required: true})
	var ack, nak, term bool
	_ = d.ProcessAlertForTest(context.Background(), platformtelegram.IncidentAlert{
		Source: "server_spoof", Severity: "high", Code: "x", Title: "t",
	}, &ack, &nak, &term)
	require.True(t, term)
	require.False(t, ack)
}
