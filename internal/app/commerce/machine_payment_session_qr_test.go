package commerce

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQRFromAttemptPayload(t *testing.T) {
	payload, err := json.Marshal(map[string]any{
		"qr_code_url": "https://momo.test/qr",
		"qr_url":      "https://fallback.test/qr",
	})
	require.NoError(t, err)
	require.Equal(t, "https://momo.test/qr", qrFromAttemptPayload(payload))

	empty, err := json.Marshal(map[string]any{"provider": "momo"})
	require.NoError(t, err)
	require.Equal(t, "", qrFromAttemptPayload(empty))
	require.Equal(t, "", qrFromAttemptPayload([]byte("not-json")))
}
