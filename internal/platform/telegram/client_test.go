package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClientSendIncidentOKTrue(t *testing.T) {
	var got struct {
		ChatID string `json:"chat_id"`
		Text   string `json:"text"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/botsecret/sendMessage", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer server.Close()

	client := NewClient(Config{BotToken: "secret", ChatID: "-100123", APIBase: server.URL})
	err := client.SendIncident(context.Background(), IncidentAlert{
		Source:            "app",
		MachineID:         "11111111-1111-1111-1111-111111111111",
		MachineCode:       "AVF-HCM-01",
		MachineName:       "Quay A",
		SerialNumber:      "SN-9",
		SiteID:            "22222222-2222-2222-2222-222222222222",
		ReportedMachineID: "AVF-HCM-01",
		OccurrenceID:      "incident_anr:123",
		Severity:          "critical",
		Code:              "incident_anr",
		Title:             "Cursor crashed",
		DedupeKey:         "fp-1",
	})

	require.NoError(t, err)
	require.Equal(t, "-100123", got.ChatID)
	require.Contains(t, got.Text, "Cursor crashed")
	require.Contains(t, got.Text, "CRITICAL")
	require.Contains(t, got.Text, "AVF-HCM-01")
	require.Contains(t, got.Text, "incident_anr:123")
	require.NotContains(t, got.Text, "secret")
}

func TestClientOKFalseIsFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "description": "Bad Request: chat not found", "error_code": 400})
	}))
	defer server.Close()
	err := NewClient(Config{BotToken: "secret-token-xyz", ChatID: "chat", APIBase: server.URL}).
		SendIncident(context.Background(), IncidentAlert{Severity: "high", Code: "x", Title: "t"})
	require.Error(t, err)
	require.True(t, IsPermanent(err))
	require.NotContains(t, err.Error(), "secret-token-xyz")
}

func TestClient429RetryAfter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": false, "error_code": 429,
			"parameters": map[string]any{"retry_after": 7},
		})
	}))
	defer server.Close()
	err := NewClient(Config{BotToken: "secret", ChatID: "chat", APIBase: server.URL}).
		SendText(context.Background(), "hello")
	require.Error(t, err)
	require.True(t, IsRetryable(err))
	require.Equal(t, 7*time.Second, RetryAfter(err))
}

func TestClientDisabledNotSilentSuccess(t *testing.T) {
	err := NewClient(Config{}).SendIncident(context.Background(), IncidentAlert{Severity: "high", Code: "x"})
	require.Error(t, err)
	require.True(t, IsPermanent(err))
}

func TestFormatIncidentIncludesOccurrence(t *testing.T) {
	text := FormatIncident(IncidentAlert{
		Source:       "app",
		MachineID:    "mid-uuid",
		MachineCode:  "AVF-01",
		OccurrenceID: "incident_timeout:99",
		Fingerprint:  "fp",
		Severity:     "error",
		Code:         "incident_timeout",
		Title:        "Dispense timeout",
		DedupeKey:    "d1",
	})
	require.Contains(t, text, "[APP][ERROR]")
	require.Contains(t, text, "Occurrence ID: incident_timeout:99")
	require.Contains(t, text, "AVF-01")
}

func TestBoundMessageKeepsPrefix(t *testing.T) {
	long := "🚨 [APP][ERROR] title\n" + strings.Repeat("x", 5000)
	out := BoundMessage(long)
	require.LessOrEqual(t, len([]rune(out)), maxMessageChars)
	require.Contains(t, out, "[APP][ERROR]")
}

func TestClientSendsOptionalDocument(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer server.Close()

	err := NewClient(Config{BotToken: "secret", ChatID: "chat", APIBase: server.URL}).SendIncident(context.Background(), IncidentAlert{
		MachineID: "machine-1", Severity: "high", Code: "diagnostic", DocumentURL: "https://example.test/bundle.json",
	})

	require.NoError(t, err)
	require.Equal(t, []string{"/botsecret/sendMessage", "/botsecret/sendDocument"}, paths)
}

func TestTokenAbsentFromTransportError(t *testing.T) {
	client := NewClient(Config{
		BotToken: "super-secret-bot-token",
		ChatID:   "chat",
		APIBase:  "http://127.0.0.1:1",
		HTTP:     &http.Client{Timeout: 50 * time.Millisecond},
	})
	err := client.SendText(context.Background(), "hi")
	require.Error(t, err)
	require.True(t, IsRetryable(err))
	require.False(t, IsPermanent(err))
	require.NotContains(t, err.Error(), "super-secret-bot-token")
}

func TestClient5xxIsRetryable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "description": "upstream"})
	}))
	defer server.Close()

	err := NewClient(Config{BotToken: "secret-5xx-token", ChatID: "chat", APIBase: server.URL}).
		SendText(context.Background(), "hello")
	require.Error(t, err)
	require.True(t, IsRetryable(err))
	require.False(t, IsPermanent(err))
	var se *SendError
	require.True(t, errors.As(err, &se))
	require.Equal(t, http.StatusBadGateway, se.Status)
	require.NotContains(t, err.Error(), "secret-5xx-token")
}

func TestClientHTTPTimeoutIsRetryable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer server.Close()

	err := NewClient(Config{
		BotToken: "secret-timeout-token",
		ChatID:   "chat",
		APIBase:  server.URL,
		HTTP:     &http.Client{Timeout: 50 * time.Millisecond},
	}).SendText(context.Background(), "hello")
	require.Error(t, err)
	require.True(t, IsRetryable(err))
	require.False(t, IsPermanent(err))
	require.Contains(t, err.Error(), "telegram: transport:")
	require.NotContains(t, err.Error(), "secret-timeout-token")
}
