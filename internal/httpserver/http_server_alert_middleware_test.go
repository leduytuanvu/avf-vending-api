package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/avf/avf-vending-api/internal/app/alerts"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestRequestLoggingMiddleware_Pages5xxNot4xx(t *testing.T) {
	t.Cleanup(func() {
		SetHTTPServerAlertReporter(nil)
		httpServerAlertTestHook = nil
	})

	var reports []alerts.ServerAlert
	httpServerAlertTestHook = func(a alerts.ServerAlert) { reports = append(reports, a) }

	mw := requestLoggingMiddleware(zap.NewNop())

	h4 := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	rr := httptest.NewRecorder()
	h4.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/x", nil))
	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Empty(t, reports)

	h5 := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	rr = httptest.NewRecorder()
	h5.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/y", nil))
	require.Equal(t, http.StatusInternalServerError, rr.Code)
	require.Len(t, reports, 1)
	require.Equal(t, "http_5xx", reports[0].Code)

	h401 := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	rr = httptest.NewRecorder()
	h401.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/z", nil))
	require.Len(t, reports, 1)
}

func TestHTTPPanicReportsOnce_NotDoubleWith5xx(t *testing.T) {
	t.Cleanup(func() {
		SetHTTPServerAlertReporter(nil)
		httpServerAlertTestHook = nil
	})

	var reports []alerts.ServerAlert
	httpServerAlertTestHook = func(a alerts.ServerAlert) { reports = append(reports, a) }

	// Mirror production order: recover outermost, logging inside.
	h := recoverServerAlertMiddleware(zap.NewNop())(
		requestLoggingMiddleware(zap.NewNop())(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				panic("boom")
			}),
		),
	)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/panic", nil))
	require.Equal(t, http.StatusInternalServerError, rr.Code)
	require.Len(t, reports, 1)
	require.Equal(t, "http_panic", reports[0].Code)
}

func TestHTTPOrdinary500ReportsOnce(t *testing.T) {
	t.Cleanup(func() {
		SetHTTPServerAlertReporter(nil)
		httpServerAlertTestHook = nil
	})

	var reports []alerts.ServerAlert
	httpServerAlertTestHook = func(a alerts.ServerAlert) { reports = append(reports, a) }

	h := recoverServerAlertMiddleware(zap.NewNop())(
		requestLoggingMiddleware(zap.NewNop())(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}),
		),
	)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/err", nil))
	require.Equal(t, http.StatusInternalServerError, rr.Code)
	require.Len(t, reports, 1)
	require.Equal(t, "http_5xx", reports[0].Code)
}
