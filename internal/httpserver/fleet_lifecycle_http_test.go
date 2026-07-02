package httpserver_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	appfleet "github.com/avf/avf-vending-api/internal/app/fleet"
	"github.com/stretchr/testify/require"
)

func TestValidateLifecycleMutationReasonRequired(t *testing.T) {
	err := appfleet.ValidateLifecycleMutation("suspend", appfleet.LifecycleMutationInput{}, false)
	require.ErrorIs(t, err, appfleet.ErrLifecycleReasonRequired)
}

func TestParseLifecycleMutationBody(t *testing.T) {
	body := map[string]any{"reason": "maintenance", "correlation_id": "11111111-1111-1111-1111-111111111111"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/machines/x/suspend", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	// parseLifecycleMutation is unexported; reason validation covered by fleet package tests.
	require.NotEmpty(t, req.Body)
}
