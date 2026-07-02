package httpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	appfleet "github.com/avf/avf-vending-api/internal/app/fleet"
	"github.com/stretchr/testify/require"
)

func TestEnterpriseFlow_lifecycleReasonRequiredHTTP(t *testing.T) {
	t.Parallel()
	body := map[string]any{"reason": ""}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/machines/x/suspend", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	err := appfleet.ValidateLifecycleMutation("suspend", appfleet.LifecycleMutationInput{Reason: ""}, false)
	require.ErrorIs(t, err, appfleet.ErrLifecycleReasonRequired)
	require.NotEmpty(t, req.Body)
}

func TestEnterpriseFlow_adminEnterpriseRoutesRequireAuth(t *testing.T) {
	t.Parallel()
	h := testMountV1ForAdminREST(t)
	paths := []string{
		"/v1/admin/machines/11111111-1111-1111-1111-111111111111/runtime-sessions/current",
		"/v1/admin/machines/11111111-1111-1111-1111-111111111111/ops-overview",
		"/v1/admin/machines/11111111-1111-1111-1111-111111111111/timeline/unified",
	}
	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		require.Equal(t, http.StatusUnauthorized, rec.Code, path)
	}
}
