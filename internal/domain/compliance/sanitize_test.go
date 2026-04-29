package compliance

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitizeJSONBytes_redactsSensitiveKeys(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"email":"a@b.com","refresh_token":"secret","nested":{"api_key":"x","card_number":"4111111111111111"}}`)
	out := SanitizeJSONBytes(raw)
	var m map[string]any
	require.NoError(t, json.Unmarshal(out, &m))
	require.Equal(t, "[REDACTED]", m["refresh_token"])
	nested, ok := m["nested"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "[REDACTED]", nested["api_key"])
	require.Equal(t, "[REDACTED]", nested["card_number"])
}

func TestSanitizeJSONBytes_redactsPANLikeStringValues(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"note":"provider returned card 4111 1111 1111 1111 for dispute","safe_id":"1234567890123"}`)
	out := SanitizeJSONBytes(raw)
	var m map[string]any
	require.NoError(t, json.Unmarshal(out, &m))
	require.Equal(t, "provider returned card [REDACTED] for dispute", m["note"])
	require.Equal(t, "1234567890123", m["safe_id"])
}
