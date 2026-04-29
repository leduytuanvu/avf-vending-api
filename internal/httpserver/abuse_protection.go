package httpserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	appmw "github.com/avf/avf-vending-api/internal/middleware"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/platform/ratelimit"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AbuseProtection wires fixed-window abuse limits (login, refresh, admin writes, machine routes, webhooks, public setup).
type AbuseProtection struct {
	cfg     config.AbuseRateLimitConfig
	backend ratelimit.Backend
	log     *zap.Logger
}

// NewAbuseProtection constructs handlers from config. backend may be nil when cfg.Enabled is false.
func NewAbuseProtection(cfg config.AbuseRateLimitConfig, backend ratelimit.Backend, log *zap.Logger) *AbuseProtection {
	if log == nil {
		log = zap.NewNop()
	}
	return &AbuseProtection{cfg: cfg, backend: backend, log: log}
}

func (a *AbuseProtection) enabled() bool {
	return a.cfg.Enabled && a.backend != nil
}

func (a *AbuseProtection) window() time.Duration {
	w := a.cfg.LockoutWindow
	if w < time.Minute {
		return time.Minute
	}
	return w
}

func (a *AbuseProtection) noop(next http.Handler) http.Handler { return next }

// LoginPOST limits POST /v1/auth/login by client IP + stable hash of normalized email (bucket auth_login:{ip}:{email_hash}).
func (a *AbuseProtection) LoginPOST() func(http.Handler) http.Handler {
	if !a.enabled() {
		return a.noop
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
			r.Body = io.NopCloser(bytes.NewReader(body))
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			var lr struct {
				Email string `json:"email"`
			}
			_ = json.Unmarshal(body, &lr)
			email := strings.TrimSpace(strings.ToLower(lr.Email))
			emailDig := sha256.Sum256([]byte(email))
			emailH := hex.EncodeToString(emailDig[:])
			key := ratelimit.StableKey("auth_login", clientIP(r), emailH)
			a.apply(w, r, key, int64(a.cfg.LoginPerMinute), next)
		})
	}
}

// RefreshPOST limits POST /v1/auth/refresh by refresh token fingerprint (opaque tokens act as jti surrogate) + IP (bucket auth_refresh:{token}:{ip}).
func (a *AbuseProtection) RefreshPOST() func(http.Handler) http.Handler {
	if !a.enabled() {
		return a.noop
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
			r.Body = io.NopCloser(bytes.NewReader(body))
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			var rr struct {
				RefreshToken string `json:"refreshToken"`
			}
			_ = json.Unmarshal(body, &rr)
			fp := sha256.Sum256([]byte(strings.TrimSpace(rr.RefreshToken)))
			tokenFP := hex.EncodeToString(fp[:16])
			key := ratelimit.StableKey("auth_refresh", tokenFP, clientIP(r))
			a.apply(w, r, key, int64(a.cfg.RefreshPerMinute), next)
		})
	}
}

// AdminMutation limits mutating methods under /v1/admin per account + organization scope.
func (a *AbuseProtection) AdminMutation() func(http.Handler) http.Handler {
	if !a.enabled() {
		return a.noop
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isMutatingHTTPMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}
			if !strings.HasPrefix(r.URL.Path, "/v1/admin/") {
				next.ServeHTTP(w, r)
				return
			}
			p, ok := auth.PrincipalFromContext(r.Context())
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			org := p.OrganizationID
			if p.HasRole(auth.RolePlatformAdmin) {
				if q := strings.TrimSpace(r.URL.Query().Get("organization_id")); q != "" {
					if id, err := uuid.Parse(q); err == nil {
						org = id
					}
				}
			}
			key := ratelimit.StableKey("admin_mut", p.Subject, org.String())
			a.apply(w, r, key, int64(a.cfg.AdminMutationPerMinute), next)
		})
	}
}

// MachineScoped limits /v1/machines/{machineId}/... telemetry + runtime routes per machine id.
// CommandDispatchPOST limits POST /v1/machines/{machineId}/commands/dispatch per machine id + client IP.
func (a *AbuseProtection) CommandDispatchPOST() func(http.Handler) http.Handler {
	if !a.enabled() {
		return a.noop
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/commands/dispatch") {
				next.ServeHTTP(w, r)
				return
			}
			mid, ok := machineIDFromV1Path(r.URL.Path)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			key := ratelimit.StableKey("cmd_disp", mid.String(), clientIP(r))
			a.apply(w, r, key, int64(a.cfg.CommandDispatchPerMinute), next)
		})
	}
}

func (a *AbuseProtection) MachineScoped() func(http.Handler) http.Handler {
	if !a.enabled() {
		return a.noop
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mid, ok := machineIDFromV1Path(r.URL.Path)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			key := ratelimit.StableKey("machine", mid.String())
			a.apply(w, r, key, int64(a.cfg.MachinePerMinute), next)
		})
	}
}

// WebhookPOST limits PSP payment webhooks by provider + IP + webhook_event_id from JSON body (bucket webhook:{provider}:{source}).
func (a *AbuseProtection) WebhookPOST() func(http.Handler) http.Handler {
	if !a.enabled() {
		return a.noop
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			r.Body = io.NopCloser(bytes.NewReader(body))
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			var wh struct {
				Provider       string `json:"provider"`
				WebhookEventID string `json:"webhook_event_id"`
			}
			_ = json.Unmarshal(body, &wh)
			ev := strings.TrimSpace(wh.WebhookEventID)
			pr := strings.TrimSpace(wh.Provider)
			key := ratelimit.StableKey("webhook", pr, clientIP(r), ev)
			a.apply(w, r, key, int64(a.cfg.WebhookPerMinute), next)
		})
	}
}

// ReportsReadGET limits GET report endpoints per interactive subject + organization scope.
func (a *AbuseProtection) ReportsReadGET() func(http.Handler) http.Handler {
	if !a.enabled() {
		return a.noop
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || !strings.Contains(r.URL.Path, "/reports") {
				next.ServeHTTP(w, r)
				return
			}
			p, ok := auth.PrincipalFromContext(r.Context())
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			org := p.OrganizationID
			if p.HasRole(auth.RolePlatformAdmin) {
				if q := strings.TrimSpace(r.URL.Query().Get("organization_id")); q != "" {
					if id, err := uuid.Parse(q); err == nil {
						org = id
					}
				}
			}
			key := ratelimit.StableKey("reports_read", p.Subject, org.String())
			a.apply(w, r, key, int64(a.cfg.ReportsReadPerMinute), next)
		})
	}
}

// PasswordResetRequestPOST limits POST /v1/auth/password/reset/request by IP + org id + email hash.
func (a *AbuseProtection) PasswordResetRequestPOST() func(http.Handler) http.Handler {
	if !a.enabled() {
		return a.noop
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
			r.Body = io.NopCloser(bytes.NewReader(body))
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			var pr struct {
				OrganizationID string `json:"organizationId"`
				Email          string `json:"email"`
			}
			_ = json.Unmarshal(body, &pr)
			org := strings.TrimSpace(pr.OrganizationID)
			email := strings.TrimSpace(strings.ToLower(pr.Email))
			emailDig := sha256.Sum256([]byte(email))
			emailH := hex.EncodeToString(emailDig[:])
			key := ratelimit.StableKey("password_reset_req", clientIP(r), org, emailH)
			a.apply(w, r, key, int64(a.cfg.PublicPerMinute), next)
		})
	}
}

// PasswordResetConfirmPOST limits POST /v1/auth/password/reset/confirm by IP + hash of reset token (opaque secret never logged).
func (a *AbuseProtection) PasswordResetConfirmPOST() func(http.Handler) http.Handler {
	if !a.enabled() {
		return a.noop
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
			r.Body = io.NopCloser(bytes.NewReader(body))
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			var pc struct {
				Token string `json:"token"`
			}
			_ = json.Unmarshal(body, &pc)
			fp := sha256.Sum256([]byte(strings.TrimSpace(pc.Token)))
			tokenFP := hex.EncodeToString(fp[:24])
			key := ratelimit.StableKey("password_reset_confirm", clientIP(r), tokenFP)
			a.apply(w, r, key, int64(a.cfg.PublicPerMinute), next)
		})
	}
}

// ActivationClaimPOST limits POST /v1/setup/activation-codes/claim by client IP + hash of activation code (bucket activation_claim:{ip}:{activation_code_hash}).
func (a *AbuseProtection) ActivationClaimPOST() func(http.Handler) http.Handler {
	if !a.enabled() {
		return a.noop
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			r.Body = io.NopCloser(bytes.NewReader(body))
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			var ac struct {
				ActivationCode string `json:"activationCode"`
			}
			_ = json.Unmarshal(body, &ac)
			code := strings.TrimSpace(ac.ActivationCode)
			codeDig := sha256.Sum256([]byte(code))
			codeH := hex.EncodeToString(codeDig[:])
			key := ratelimit.StableKey("activation_claim", clientIP(r), codeH)
			a.apply(w, r, key, int64(a.cfg.PublicPerMinute), next)
		})
	}
}

func (a *AbuseProtection) apply(w http.ResponseWriter, r *http.Request, key string, limit int64, next http.Handler) {
	ok, retryAfter := a.backend.Allow(r.Context(), key, limit, a.window())
	if ok {
		next.ServeHTTP(w, r)
		return
	}
	reqID := appmw.RequestIDFromContext(r.Context())
	corr := appmw.CorrelationIDFromContext(r.Context())
	a.log.Warn("abuse rate limit exceeded",
		zap.String("path", r.URL.Path),
		zap.String("method", r.Method),
		zap.String("request_id", reqID),
		zap.String("correlation_id", corr),
		zap.String("client_ip", clientIP(r)),
	)
	writeAbuseRateLimited(w, r.Context(), retryAfter)
}

func writeAbuseRateLimited(w http.ResponseWriter, ctx context.Context, retryAfter time.Duration) {
	sec := int(math.Ceil(retryAfter.Seconds()))
	if sec < 1 {
		sec = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(sec))
	writeAPIErrorDetails(w, ctx, http.StatusTooManyRequests, "rate_limited", "too many requests", map[string]any{
		"retry_after_seconds": sec,
	})
}

func isMutatingHTTPMethod(m string) bool {
	switch m {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func machineIDFromV1Path(path string) (uuid.UUID, bool) {
	const pfx = "/v1/machines/"
	if !strings.HasPrefix(path, pfx) {
		return uuid.Nil, false
	}
	rest := strings.TrimPrefix(path, pfx)
	if rest == "" {
		return uuid.Nil, false
	}
	i := strings.IndexByte(rest, '/')
	if i <= 0 {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(rest[:i])
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}
