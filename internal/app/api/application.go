package api

import (
	"strings"
	"time"

	appactivation "github.com/avf/avf-vending-api/internal/app/activation"
	appadminops "github.com/avf/avf-vending-api/internal/app/adminops"
	appanomalies "github.com/avf/avf-vending-api/internal/app/anomalies"
	appartifacts "github.com/avf/avf-vending-api/internal/app/artifacts"
	appaudit "github.com/avf/avf-vending-api/internal/app/audit"
	appauth "github.com/avf/avf-vending-api/internal/app/auth"
	appcatalogadmin "github.com/avf/avf-vending-api/internal/app/catalogadmin"
	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	appcommerceadmin "github.com/avf/avf-vending-api/internal/app/commerceadmin"
	appdevice "github.com/avf/avf-vending-api/internal/app/device"
	appfeatureflags "github.com/avf/avf-vending-api/internal/app/featureflags"
	appfinance "github.com/avf/avf-vending-api/internal/app/finance"
	appfleet "github.com/avf/avf-vending-api/internal/app/fleet"
	appfleetadmin "github.com/avf/avf-vending-api/internal/app/fleetadmin"
	appinventoryadmin "github.com/avf/avf-vending-api/internal/app/inventoryadmin"
	appmediaadmin "github.com/avf/avf-vending-api/internal/app/mediaadmin"
	appoperator "github.com/avf/avf-vending-api/internal/app/operator"
	appotaadmin "github.com/avf/avf-vending-api/internal/app/otaadmin"
	apppayments "github.com/avf/avf-vending-api/internal/app/payments"
	appplanogram "github.com/avf/avf-vending-api/internal/app/planogram"
	approvisioning "github.com/avf/avf-vending-api/internal/app/provisioning"
	appreporting "github.com/avf/avf-vending-api/internal/app/reporting"
	approllout "github.com/avf/avf-vending-api/internal/app/rollout"
	appsalecatalog "github.com/avf/avf-vending-api/internal/app/salecatalog"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/platform/auth/revocation"
	platformmqtt "github.com/avf/avf-vending-api/internal/platform/mqtt"
	"github.com/google/uuid"
)

// HTTPApplication groups versioned HTTP API application services (ports only at the HTTP boundary).
type HTTPApplication struct {
	Auth             *appauth.Service
	CatalogAdmin     *appcatalogadmin.Service
	InventoryAdmin   *appinventoryadmin.Service
	AdminMachines    MachinesAdminService
	AdminTechnicians TechniciansAdminService
	AdminAssignments AssignmentsAdminService
	// Fleet mutates sites, machines, technicians, and assignments (admin CRUD); list views use Admin* services above.
	Fleet          *appfleet.Service
	AdminCommands  CommandsAdminService
	AdminOTA       OTAAdminService
	Payments       PaymentsService
	Orders         OrdersService
	Reconciliation ReconciliationAdminService
	// PaymentOps lists webhooks/settlements/disputes and finance export (P1.2); nil hides routes.
	PaymentOps      *apppayments.AdminService
	MachineShadow   MachineShadowService
	MachineOperator *appoperator.Service
	Commerce        *appcommerce.Service
	// EnterpriseAudit persists audit_events (internal writes + GET /v1/admin/audit/events).
	EnterpriseAudit *appaudit.Service
	Activation      *appactivation.Service
	RemoteCommands  *appdevice.MQTTCommandDispatcher
	Artifacts       *appartifacts.Service
	// TelemetryStore serves read-only telemetry projection endpoints (rollups / incidents / snapshot).
	TelemetryStore *postgres.Store
	// SaleCatalog optional shared snapshot builder (e.g. Redis cache); nil uses an uncached builder in HTTP handlers.
	SaleCatalog appsalecatalog.SnapshotBuilder
	Reporting   ReportingService
	// ReportingSyncMaxSpan caps synchronous GET reporting windows (from Capacity); zero defaults in HTTP handlers.
	ReportingSyncMaxSpan time.Duration
	// ReportingExportMaxSpan caps CSV / export report downloads (typically wider than sync JSON).
	ReportingExportMaxSpan time.Duration
	Finance                FinanceService
	// FeatureFlags manages tenant feature flags and staged machine config rollouts (optional).
	FeatureFlags *appfeatureflags.Service
	// MediaAdmin manages enterprise media_assets + presigned uploads when object storage is enabled (nil when Artifacts is nil).
	MediaAdmin *appmediaadmin.Service
	// Planogram manages enterprise draft/publish/version planogram APIs when wired from api.NewHTTPApplication.
	Planogram *appplanogram.Service
	// AdminOps exposes tenant operational troubleshooting APIs (machine health, commands, inventory anomalies).
	AdminOps *appadminops.Service
	// Anomalies runs P2.4 operational detectors and unified tenant anomaly + restock suggestion APIs.
	Anomalies *appanomalies.Service
	// Provisioning creates machines in bulk (optional; nil hides routes).
	Provisioning *approvisioning.Service
	// Rollout runs fleet rollouts against the MQTT command ledger (optional; nil hides routes).
	Rollout *approllout.Service
	// CashSettlementVarianceReviewThresholdMinor flags close review when abs(variance) exceeds this (minor units); 0 means default 500 at runtime.
	CashSettlementVarianceReviewThresholdMinor int64
	// ListPaymentProviders returns non-secret PSP registry rows for GET /v1/admin/payment/providers (nil hides the route).
	ListPaymentProviders func() []PaymentProviderRegistryInfo
}

// PaymentProviderRegistryInfo is a read-only admin view of a registered payment provider adapter (secrets are never included).
type PaymentProviderRegistryInfo struct {
	Key              string `json:"key"`
	QuerySupported   bool   `json:"query_supported"`
	WebhookProfile   string `json:"webhook_profile"`
	ConfigSource     string `json:"config_source"`
	DefaultForEnv    bool   `json:"default_for_env,omitempty"`
	ActiveSessionKey bool   `json:"active_session_key,omitempty"`
}

// HTTPApplicationDeps wires real domain services for the HTTP API process.
type HTTPApplicationDeps struct {
	Store              *postgres.Store
	Fleet              *appfleet.Service
	Commerce           *appcommerce.Service
	MQTTCommandPublish appdevice.MQTTDispatchPublisher
	// MQTTTopicPrefix and MQTTTopicLayout match the API MQTT publisher (ledger.route_key / diagnostics). Empty prefix leaves topic empty in status metadata.
	MQTTTopicPrefix string
	MQTTTopicLayout string
	Artifacts       *appartifacts.Service
	HTTPAuth        config.HTTPAuthConfig
	MachineJWT      config.MachineJWTConfig
	// CatalogMediaCacheBumper optional; Redis-backed media epoch bump when configured.
	CatalogMediaCacheBumper appmediaadmin.CatalogMediaCacheBumper
	// AccessRevocation optional; Redis-backed access JTI / subject revocation (logout, admin deactivate).
	AccessRevocation revocation.Store
	SessionCache     plauth.RefreshSessionCache
	LoginFailures    plauth.LoginFailureCounter
	// SaleCatalog optional shared snapshot builder for HTTP + gRPC (e.g. Redis cache).
	SaleCatalog appsalecatalog.SnapshotBuilder
	// EnterpriseAudit optional; when nil it is constructed from Store.Pool().
	EnterpriseAudit *appaudit.Service
	// CashSettlementVarianceReviewThresholdMinor from CASH_SETTLEMENT_VARIANCE_REVIEW_THRESHOLD_MINOR (0 = use handler default).
	CashSettlementVarianceReviewThresholdMinor int64
	// AuditCriticalFailOpen mirrors AUDIT_CRITICAL_FAIL_OPEN (development/test only); passed to appaudit.NewService when EnterpriseAudit is nil.
	AuditCriticalFailOpen bool
	// AdminAuthSecurity configures MFA policy, login lockout, and password validation for interactive admins.
	AdminAuthSecurity config.AdminAuthSecurityConfig
	AppEnv            config.AppEnvironment
	// ReportingSyncMaxSpan from cfg.Capacity (optional; zero uses reporting.DefaultReportingWindow in handlers).
	ReportingSyncMaxSpan time.Duration
	// ReportingExportMaxSpan from cfg.Capacity EffectiveReportingExportMaxSpan.
	ReportingExportMaxSpan time.Duration
	// ProductMediaThumbSize / ProductMediaDisplaySize bound catalog WebP variants (zero uses mediaadmin defaults).
	ProductMediaThumbSize   int
	ProductMediaDisplaySize int
}

// NewHTTPApplication constructs HTTP ports backed by real adapters where they exist.
// Callers must supply a non-nil Store and Fleet service when DATABASE_URL is enabled for this process.
func NewHTTPApplication(deps HTTPApplicationDeps) *HTTPApplication {
	if deps.Store == nil {
		panic("api.NewHTTPApplication: nil Store")
	}
	if deps.Fleet == nil {
		panic("api.NewHTTPApplication: nil Fleet service")
	}
	if deps.Commerce == nil {
		panic("api.NewHTTPApplication: nil Commerce service")
	}
	pool := deps.Store.Pool()
	queries := db.New(pool)
	auditSvc := deps.EnterpriseAudit
	if auditSvc == nil {
		auditSvc = appaudit.NewService(pool, appaudit.ServiceOpts{CriticalFailOpen: deps.AuditCriticalFailOpen})
	}
	issuer, err := plauth.NewSessionIssuerFromHTTPAuth(deps.HTTPAuth)
	if err != nil {
		panic("api.NewHTTPApplication: session issuer: " + err.Error())
	}
	issuer.ConfigureMachineTokens(deps.MachineJWT)
	authSvc, err := appauth.NewService(appauth.Deps{
		Queries:          queries,
		Issuer:           issuer,
		Pool:             pool,
		AccessRevocation: deps.AccessRevocation,
		SessionCache:     deps.SessionCache,
		LoginFailures:    deps.LoginFailures,
		EnterpriseAudit:  auditSvc,
		OnAdminMutation:  WireAuthAdminMutationAudit(auditSvc),
		AdminSecurity:    deps.AdminAuthSecurity,
		AppEnv:           deps.AppEnv,
	})
	if err != nil {
		panic("api.NewHTTPApplication: auth service: " + err.Error())
	}
	actPepper := plauth.TrimSecret(deps.HTTPAuth.JWTSecret)
	if len(actPepper) == 0 {
		actPepper = plauth.TrimSecret(deps.HTTPAuth.LoginJWTSecret)
	}
	activationSvc := appactivation.NewService(deps.Store.Pool(), issuer, actPepper, auditSvc)
	var pm appcatalogadmin.ProductMediaDeps
	if deps.Artifacts != nil {
		pm.Store = deps.Artifacts.Store()
		pm.MaxUploadBytes = deps.Artifacts.MaxUploadBytes()
		pm.PresignTTL = deps.Artifacts.DownloadPresignTTL()
	}
	catSvc, err := appcatalogadmin.NewService(queries, pool, auditSvc, pm)
	if err != nil {
		panic("api.NewHTTPApplication: catalog admin: " + err.Error())
	}
	catSvc.SetCatalogCacheInvalidator(deps.CatalogMediaCacheBumper)
	catSvc.SetPromotionAuditHook(appcatalogadmin.EnterprisePromotionAuditHook(auditSvc))
	invSvc, err := appinventoryadmin.NewService(queries)
	if err != nil {
		panic("api.NewHTTPApplication: inventory admin: " + err.Error())
	}
	machineOp := appoperator.NewServiceFromDeps(appoperator.Deps{
		Sessions:    postgres.NewOperatorRepository(pool),
		Machines:    postgres.NewMachineRepository(pool),
		Technicians: postgres.NewTechnicianRepository(pool),
		Assignments: postgres.NewTechnicianAssignmentRepository(pool),
	})
	fleetAdm, err := appfleetadmin.NewService(queries)
	if err != nil {
		panic("api.NewHTTPApplication: fleet admin: " + err.Error())
	}
	otaAdm, err := appotaadmin.NewService(queries, pool, deps.Store, auditSvc)
	if err != nil {
		panic("api.NewHTTPApplication: ota admin: " + err.Error())
	}
	commerceAdm, err := appcommerceadmin.NewService(pool, queries, deps.Commerce)
	if err != nil {
		panic("api.NewHTTPApplication: commerce admin: " + err.Error())
	}
	paymentOps, err := apppayments.NewAdminService(pool, queries, auditSvc)
	if err != nil {
		panic("api.NewHTTPApplication: payment ops admin: " + err.Error())
	}
	var outboundMQTTTopic func(uuid.UUID) (string, error)
	if p := strings.TrimSpace(deps.MQTTTopicPrefix); p != "" {
		layout := platformmqtt.NormalizeTopicLayout(deps.MQTTTopicLayout)
		outboundMQTTTopic = func(mid uuid.UUID) (string, error) {
			return platformmqtt.OutboundCommandPublishTopicStrict(layout, p, mid)
		}
	}
	remoteCmd := appdevice.NewMQTTCommandDispatcher(appdevice.MQTTCommandDispatcherDeps{
		Workflow:             deps.Store,
		Store:                deps.Store,
		Publisher:            deps.MQTTCommandPublish,
		Machines:             postgres.NewFleetRepository(pool),
		OutboundCommandTopic: outboundMQTTTopic,
	})
	reportingSvc := appreporting.NewService(queries)
	financeSvc := appfinance.NewService(queries, auditSvc)

	ffSvc, err := appfeatureflags.NewService(queries, pool, auditSvc)
	if err != nil {
		panic("api.NewHTTPApplication: feature flags: " + err.Error())
	}

	setupRepo := postgres.NewSetupRepository(pool)
	planogramSvc := appplanogram.NewService(appplanogram.Deps{
		Pool:           pool,
		Setup:          setupRepo,
		RemoteCommands: remoteCmd,
		Audit:          auditSvc,
	})

	var mediaSvc *appmediaadmin.Service
	if deps.Artifacts != nil {
		ms, merr := appmediaadmin.NewService(appmediaadmin.Deps{
			Pool:             pool,
			Store:            deps.Artifacts.Store(),
			Audit:            auditSvc,
			PresignPutTTL:    deps.Artifacts.DownloadPresignTTL(),
			MaxUploadBytes:   deps.Artifacts.MaxUploadBytes(),
			Cache:            deps.CatalogMediaCacheBumper,
			ThumbMaxPixels:   deps.ProductMediaThumbSize,
			DisplayMaxPixels: deps.ProductMediaDisplaySize,
		})
		if merr != nil {
			panic("api.NewHTTPApplication: media admin: " + merr.Error())
		}
		mediaSvc = ms
	}

	adminOpsSvc, adminOpsErr := appadminops.NewService(appadminops.Deps{
		Pool:           pool,
		RemoteCommands: remoteCmd,
	})
	if adminOpsErr != nil {
		panic("api.NewHTTPApplication: admin ops: " + adminOpsErr.Error())
	}

	anomSvc, anomErr := appanomalies.NewService(pool, invSvc)
	if anomErr != nil {
		panic("api.NewHTTPApplication: anomalies: " + anomErr.Error())
	}

	provSvc := approvisioning.NewService(approvisioning.Deps{
		Pool:       pool,
		Fleet:      deps.Fleet,
		Activation: activationSvc,
		Audit:      auditSvc,
	})
	var rollSvc *approllout.Service
	if remoteCmd != nil {
		rollSvc = approllout.NewService(approllout.Deps{
			Pool:       pool,
			Dispatcher: remoteCmd,
			Audit:      auditSvc,
		})
	}

	return &HTTPApplication{
		Auth:                   authSvc,
		CatalogAdmin:           catSvc,
		InventoryAdmin:         invSvc,
		AdminMachines:          fleetAdm,
		AdminTechnicians:       fleetAdm,
		AdminAssignments:       fleetAdm,
		AdminCommands:          fleetAdm,
		Fleet:                  deps.Fleet,
		AdminOTA:               otaAdm,
		Payments:               commerceAdm,
		Orders:                 commerceAdm,
		Reconciliation:         commerceAdm,
		PaymentOps:             paymentOps,
		MachineShadow:          NewSQLMachineShadow(pool),
		MachineOperator:        machineOp,
		Commerce:               deps.Commerce,
		EnterpriseAudit:        auditSvc,
		Activation:             activationSvc,
		RemoteCommands:         remoteCmd,
		Artifacts:              deps.Artifacts,
		TelemetryStore:         deps.Store,
		SaleCatalog:            deps.SaleCatalog,
		Reporting:              reportingSvc,
		ReportingSyncMaxSpan:   deps.ReportingSyncMaxSpan,
		ReportingExportMaxSpan: deps.ReportingExportMaxSpan,
		Finance:                financeSvc,
		FeatureFlags:           ffSvc,
		MediaAdmin:             mediaSvc,
		Planogram:              planogramSvc,
		AdminOps:               adminOpsSvc,
		Anomalies:              anomSvc,
		Provisioning:           provSvc,
		Rollout:                rollSvc,
		CashSettlementVarianceReviewThresholdMinor: deps.CashSettlementVarianceReviewThresholdMinor,
	}
}
