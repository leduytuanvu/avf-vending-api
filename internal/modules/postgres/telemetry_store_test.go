package postgres

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseIncidentPayloadCursorEnvelope(t *testing.T) {
	payload := []byte(`{
		"eventType":"cursor.incident",
		"severity":"error",
		"payload":{
			"fingerprint":"cursor-fp-123",
			"cursor":{
				"title":"Device sync failed",
				"detail":{"retryable":true}
			}
		}
	}`)

	severity, code, title, dedupe, err := ParseIncidentPayload(payload)
	require.NoError(t, err)
	require.Equal(t, "error", severity)
	require.Equal(t, "cursor.incident", code)
	require.Equal(t, "Device sync failed", title)
	require.Equal(t, "cursor-fp-123", dedupe)
}

func TestParseIncidentPayloadRequiresCode(t *testing.T) {
	_, _, _, _, err := ParseIncidentPayload([]byte(`{"severity":"high","title":"missing code"}`))
	require.Error(t, err)
}
