package observability_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}

func TestRequiredMetricNamesAreDefined(t *testing.T) {
	root := repoRoot(t)
	type metricDef struct {
		path          string
		subsystem     string // avf_* promauto Subsystem (empty when canonical only)
		shortName     string // avf_* Name after subsystem
		canonicalName string // flat prometheus Name in productionmetrics (replaces legacy avf_* for HTTP/gRPC access)
	}
	required := map[string]metricDef{
		"http_requests_total (canonical; middleware still in request_metrics.go)": {path: "internal/platform/observability/productionmetrics/metrics.go", canonicalName: "http_requests_total"},
		"http_request_duration_seconds":                                           {path: "internal/platform/observability/productionmetrics/metrics.go", canonicalName: "http_request_duration_seconds"},
		"grpc_requests_total (grpcprom calls productionmetrics)":                  {path: "internal/platform/observability/productionmetrics/metrics.go", canonicalName: "grpc_requests_total"},
		"grpc_request_duration_seconds":                                           {path: "internal/platform/observability/productionmetrics/metrics.go", canonicalName: "grpc_request_duration_seconds"},
		"grpc_errors_total":                                                       {path: "internal/platform/observability/productionmetrics/metrics.go", canonicalName: "grpc_errors_total"},
		"payment_webhook_amount_currency_mismatch_total":                          {path: "internal/platform/observability/productionmetrics/metrics.go", canonicalName: "payment_webhook_amount_currency_mismatch_total"},
		"payment_provider_probe_stale_pending_queue":                              {path: "internal/platform/observability/productionmetrics/metrics.go", canonicalName: "payment_provider_probe_stale_pending_queue"},
		"avf_machine_connectivity_total":                                          {path: "internal/app/telemetryapp/telemetry_worker_prom.go", subsystem: "machine", shortName: "connectivity_total"},
		"avf_machine_last_seen_age_seconds":                                       {path: "internal/app/telemetryapp/telemetry_worker_prom.go", subsystem: "machine", shortName: "last_seen_age_seconds"},
		"avf_mqtt_command_state_total":                                            {path: "internal/observability/mqttprom/commands.go", subsystem: "mqtt_command", shortName: "state_total"},
		"avf_mqtt_publish_failures_total":                                         {path: "internal/platform/mqtt/publisher.go", subsystem: "mqtt", shortName: "publish_failures_total"},
		"avf_mqtt_command_ack_timeout_total":                                      {path: "internal/observability/mqttprom/commands.go", subsystem: "mqtt_command", shortName: "ack_timeout_total"},
		"avf_commerce_payment_webhook_outcomes_total":                             {path: "internal/httpserver/commerce_webhook_metrics.go", subsystem: "commerce", shortName: "payment_webhook_outcomes_total"},
		"avf_commerce_payment_paid_vend_failed_total":                             {path: "internal/app/background/reconciler.go", subsystem: "commerce", shortName: "payment_paid_vend_failed_total"},
		"avf_commerce_refund_pending_too_long_total":                              {path: "internal/app/background/reconciler.go", subsystem: "commerce", shortName: "refund_pending_too_long_total"},
		"avf_redis_rate_limit_hits_total":                                         {path: "internal/httpserver/request_metrics.go", subsystem: "redis", shortName: "rate_limit_hits_total"},
		"avf_worker_outbox_oldest_pending_age_seconds":                            {path: "internal/app/background/outboxmetrics/outbox.go", subsystem: "worker_outbox", shortName: "oldest_pending_age_seconds"},
		"avf_worker_outbox_dead_lettered_total":                                   {path: "internal/app/background/outboxmetrics/outbox.go", subsystem: "worker_outbox", shortName: "dead_lettered_total"},
		"avf_reconciler_cycle_completions_total":                                  {path: "internal/observability/reconcilerprom/telemetry.go", subsystem: "reconciler", shortName: "cycle_completions_total"},
		"avf_reconciler_cycle_duration_seconds":                                   {path: "internal/observability/reconcilerprom/telemetry.go", subsystem: "reconciler", shortName: "cycle_duration_seconds"},
		"avf_object_storage_failures_total":                                       {path: "internal/app/artifacts/service.go", subsystem: "object_storage", shortName: "failures_total"},
	}
	for metric, def := range required {
		t.Run(metric, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(def.path)))
			if err != nil {
				t.Fatal(err)
			}
			text := string(raw)
			if def.canonicalName != "" {
				if !strings.Contains(text, `"`+def.canonicalName+`"`) {
					t.Fatalf("metric Name %q not defined in %s", def.canonicalName, def.path)
				}
				return
			}
			if !containsMetricOption(text, "Subsystem", def.subsystem) || !containsMetricOption(text, "Name", def.shortName) {
				t.Fatalf("metric not defined in %s (subsystem=%q name=%q)", def.path, def.subsystem, def.shortName)
			}
		})
	}
}

func containsMetricOption(text, key, value string) bool {
	return strings.Contains(text, key+":") && strings.Contains(text, `"`+value+`"`)
}

func TestGrafanaDashboardsAreValidJSON(t *testing.T) {
	root := repoRoot(t)
	patterns := []string{
		filepath.Join(root, "deployments", "prod", "observability", "grafana", "provisioning", "dashboards", "json", "*.json"),
		filepath.Join(root, "ops", "grafana", "provisioning", "dashboards", "json", "*.json"),
	}
	for _, pattern := range patterns {
		files, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		if len(files) == 0 {
			t.Fatalf("no dashboards matched %s", pattern)
		}
		for _, path := range files {
			t.Run(filepath.Base(path), func(t *testing.T) {
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				var dashboard struct {
					UID    string `json:"uid"`
					Title  string `json:"title"`
					Panels []any  `json:"panels"`
				}
				if err := json.Unmarshal(raw, &dashboard); err != nil {
					t.Fatalf("invalid dashboard JSON: %v", err)
				}
				if dashboard.UID == "" || dashboard.Title == "" || len(dashboard.Panels) == 0 {
					t.Fatalf("dashboard must have uid, title, and panels: %+v", dashboard)
				}
			})
		}
	}
}

func TestPrometheusAlertsAreValidYAML(t *testing.T) {
	path := filepath.Join(repoRoot(t), "deployments", "prod", "observability", "prometheus", "alerts.yml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Groups []struct {
			Name  string `yaml:"name"`
			Rules []struct {
				Alert string `yaml:"alert"`
				Expr  string `yaml:"expr"`
				For   string `yaml:"for"`
			} `yaml:"rules"`
		} `yaml:"groups"`
	}
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("invalid alerts YAML: %v", err)
	}
	if len(parsed.Groups) == 0 {
		t.Fatal("expected alert groups")
	}

	required := map[string]bool{
		"AVFAPIHigh5xxRate":                       false,
		"AVFGRPCHighErrorRate":                    false,
		"AVFPaymentPaidVendNotCompleted":          false,
		"AVFCommandACKTimeoutSpike":               false,
		"AVFMachineOfflineSpike":                  false,
		"AVFNATSOutboxLagHigh":                    false,
		"AVFRedisUnavailableProduction":           false,
		"AVFPostgreSQLUnavailable":                false,
		"AVFMQTTIngestDown":                       false,
		"AVFReconcilerFailing":                    false,
		"AVFRefundPendingTooLong":                 false,
		"AVFPaymentWebhookAmountCurrencyMismatch": false,
		"AVFDataPlaneDependencyDegraded":          false,
	}
	for _, group := range parsed.Groups {
		if group.Name == "" || len(group.Rules) == 0 {
			t.Fatalf("alert group must have name and rules: %+v", group)
		}
		for _, rule := range group.Rules {
			if rule.Alert == "" || rule.Expr == "" {
				t.Fatalf("alert rule must have alert and expr: %+v", rule)
			}
			if _, ok := required[rule.Alert]; ok {
				required[rule.Alert] = true
			}
		}
	}
	for alert, found := range required {
		if !found {
			t.Fatalf("missing required alert %s", alert)
		}
	}
}
