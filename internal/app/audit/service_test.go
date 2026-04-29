package audit

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSanitizeAuditJSONBytesRedactsEnterpriseSecrets(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"jwt":"eyJhbGciOi...",
		"private_key":"-----BEGIN PRIVATE KEY-----",
		"mqtt_password":"mqttpass",
		"nested":{"refresh_token":"refresh","webhook_secret":"whsec","safe":"ok"}
	}`)

	var got map[string]any
	require.NoError(t, json.Unmarshal(sanitizeAuditJSONBytes(raw), &got))
	require.Equal(t, "[REDACTED]", got["jwt"])
	require.Equal(t, "[REDACTED]", got["private_key"])
	require.Equal(t, "[REDACTED]", got["mqtt_password"])

	nested, ok := got["nested"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "[REDACTED]", nested["refresh_token"])
	require.Equal(t, "[REDACTED]", nested["webhook_secret"])
	require.Equal(t, "ok", nested["safe"])
}

func TestWithTransportMetaDefaultsFillsMissingAuditFields(t *testing.T) {
	t.Parallel()

	ctx := compliance.WithTransportMeta(context.Background(), compliance.TransportMeta{
		RequestID: "req-1",
		TraceID:   "trace-1",
		IP:        "203.0.113.10",
		UserAgent: "audit-test",
	})
	in := compliance.EnterpriseAuditRecord{
		OrganizationID: uuid.New(),
		ActorType:      compliance.ActorUser,
		Action:         compliance.ActionAuthLogout,
		ResourceType:   "auth.session",
	}

	got := withTransportMetaDefaults(ctx, in)
	require.Equal(t, "req-1", *got.RequestID)
	require.Equal(t, "trace-1", *got.TraceID)
	require.Equal(t, "203.0.113.10", *got.IPAddress)
	require.Equal(t, "audit-test", *got.UserAgent)
}
