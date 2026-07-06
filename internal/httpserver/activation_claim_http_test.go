package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/avf/avf-vending-api/internal/app/activation"
	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
)

func activationClaimHTTPTestRouter(t *testing.T, svc *activation.Service) chi.Router {
	t.Helper()
	cfg := &config.Config{
		MQTT: config.MQTTConfig{BrokerURL: "mqtt://example.invalid", TopicPrefix: "avf/devices"},
	}
	app := &api.HTTPApplication{Activation: svc}
	r := chi.NewRouter()
	writeRL := func(h http.Handler) http.Handler { return h }
	mountPublicActivationClaim(r, app, cfg, nil, writeRL)
	return r
}

func TestPostActivationClaim_acceptsValidSixDigitCode(t *testing.T) {
	t.Parallel()
	pool := activationHTTPTestPool(t)
	svc := activationHTTPTestService(t, pool)
	machineID, _ := insertActivationHTTPTestMachine(t, pool)
	ctx := context.Background()
	create, err := svc.CreateCode(ctx, activation.CreateInput{MachineID: machineID, ExpiresInMinutes: 60, MaxUses: 1})
	require.NoError(t, err)
	require.Regexp(t, `^[0-9]{6}$`, create.PlaintextCode)

	r := activationClaimHTTPTestRouter(t, svc)
	body := bytes.NewBufferString(`{"activationCode":"` + create.PlaintextCode + `","deviceFingerprint":{"serialNumber":"claim-http-1","androidId":"aid-1"}}`)
	req := httptest.NewRequest(http.MethodPost, "/setup/activation-codes/claim", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, machineID.String(), resp["machineId"])
	require.NotEmpty(t, resp["machineToken"])
}

func TestPostActivationClaim_rejectsInvalidActivationCodeFormat(t *testing.T) {
	t.Parallel()
	pool := activationHTTPTestPool(t)
	svc := activationHTTPTestService(t, pool)
	r := activationClaimHTTPTestRouter(t, svc)

	for _, bad := range []string{"AVF-12ABCD-34EF56", "12345", "1234567"} {
		body := bytes.NewBufferString(`{"activationCode":"` + bad + `","deviceFingerprint":{"serialNumber":"x"}}`)
		req := httptest.NewRequest(http.MethodPost, "/setup/activation-codes/claim", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code, "code %q", bad)

		var resp map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.Equal(t, "activation_invalid", resp["code"])
	}
}
