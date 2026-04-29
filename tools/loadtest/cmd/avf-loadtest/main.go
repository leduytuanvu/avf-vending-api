// Command avf-loadtest is an optional fleet-shaped load harness (AVF vending API).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/tools/loadtest"
	"github.com/google/uuid"
)

func main() {
	rawScenario := flag.String("scenario", "storm", "suite | storm | checkin | grpc | admin | webhook | mqtt | prom | dry-run")
	httpBase := flag.String("http-base", envOr("AVF_LOADTEST_HTTP_BASE", "http://localhost:8080"), "REST API base URL")
	grpcAddr := flag.String("grpc-addr", envOr("AVF_LOADTEST_GRPC_ADDR", "localhost:9090"), "host:port for plaintext gRPC")
	metricsURL := flag.String("metrics-url", envOr("AVF_LOADTEST_METRICS_URL", ""), "optional scrape URL for Prometheus text (often ops listener)")
	metricsToken := flag.String("metrics-token", envOr("METRICS_SCRAPE_TOKEN", ""), "Bearer token when metrics require auth")

	manifestPath := flag.String("manifest", envOr("LOADTEST_MACHINE_MANIFEST", ""), "TSV machine_uuid<TAB>jwt or JSON manifest path")
	machinesFlag := flag.Int("machines", 0, "truncate manifest to first N machines (0=all)")
	concurrency := flag.Int("concurrency", envIntOr("AVF_LOADTEST_CONCURRENCY", 32), "parallel workers per scenario")

	duration := flag.Duration("duration", envDurOr("AVF_LOADTEST_DURATION", 30*time.Second), "iteration window for looping scenarios (admin phase in storm)")

	stormWaves := flag.Int("storm-waves", envIntOr("AVF_LOADTEST_STORM_WAVES", 3), "check-in reconnect waves for suite/storm (≥1)")

	adminJWT := flag.String("admin-jwt", envOr("ADMIN_JWT", ""), "Bearer token for admin REST scenarios")
	orgIDStr := flag.String("organization-id", envOr("LOADTEST_ORGANIZATION_ID", ""), "organization UUID for admin reports")

	productID := flag.String("grpc-product-id", envOr("LOADTEST_PRODUCT_ID", ""), "when set, run full cash vend on gRPC after core runtime calls")
	slotIndex := flag.Int("grpc-slot-index", envIntOr("LOADTEST_SLOT_INDEX", 0), "planogram slot index for grpc cash vend")

	webhookSecret := flag.String("webhook-secret", envOr("COMMERCE_PAYMENT_WEBHOOK_SECRET", ""), "HMAC secret for webhook burst")
	webhookOrder := flag.String("webhook-order-id", envOr("LOADTEST_WEBHOOK_ORDER_ID", ""), "order UUID for webhook POST path")
	webhookPayment := flag.String("webhook-payment-id", envOr("LOADTEST_WEBHOOK_PAYMENT_ID", ""), "payment UUID for webhook POST path")
	webhookBurst := flag.Int("webhook-burst", envIntOr("LOADTEST_WEBHOOK_BURST", 20), "signed webhook POST count")
	webhookDupEvery := flag.Int("webhook-duplicate-every", envIntOr("LOADTEST_WEBHOOK_DUP_EVERY", 5), "replay duplicate webhook_event_id every N (0=off)")

	skipMQTT := flag.Bool("skip-mqtt", envBool("LOADTEST_SKIP_MQTT"), "skip MQTT fleet phase (suite/storm)")
	skipWebhook := flag.Bool("skip-webhook", envBool("LOADTEST_SKIP_WEBHOOK"), "skip signed webhook burst (suite/storm)")

	mqttBroker := flag.String("mqtt-broker", envOr("MQTT_BROKER_URL", "tcp://localhost:1883"), "MQTT broker URL (tcp:// or ssl://)")
	mqttUser := flag.String("mqtt-user", envOr("MQTT_USERNAME", ""), "")
	mqttPass := flag.String("mqtt-pass", envOr("MQTT_PASSWORD", ""), "")
	mqttPrefix := flag.String("mqtt-topic-prefix", envOr("MQTT_TOPIC_PREFIX", "avf/devices"), "")
	mqttLayout := flag.String("mqtt-layout", envOr("MQTT_TOPIC_LAYOUT", "legacy"), "legacy | enterprise")
	mqttMachine := flag.String("mqtt-machine-id", envOr("LOADTEST_MQTT_MACHINE_ID", ""), "machine UUID for MQTT topics")
	mqttAckDeadline := flag.Duration("mqtt-ack-deadline", envDurOr("LOADTEST_MQTT_ACK_DEADLINE", 5*time.Second), "")

	execute := flag.Bool("execute", false, "perform real traffic (blocked when LOAD_TEST_ENV=production)")
	flag.Parse()

	if v := strings.TrimSpace(os.Getenv("EXECUTE_LOAD_TEST")); v == "true" || v == "1" || v == "yes" {
		*execute = true
	}
	loadEnv := strings.TrimSpace(strings.ToLower(os.Getenv("LOAD_TEST_ENV")))
	if loadEnv == "production" && *execute {
		fmt.Fprintln(os.Stderr, "avf-loadtest: refusing -execute when LOAD_TEST_ENV=production — use a dedicated staging tier")
		os.Exit(2)
	}

	scenario := strings.TrimSpace(strings.ToLower(*rawScenario))

	if !*execute {
		printDryRunPlan(scenario, *httpBase, *grpcAddr, *manifestPath, *machinesFlag, *stormWaves)
		fmt.Println("avf-loadtest: dry-run only. Set -execute or EXECUTE_LOAD_TEST=true after reading docs/testing/load-test.md")
		os.Exit(0)
	}

	ctx := context.Background()
	var manifest []loadtest.MachineRow
	if *manifestPath != "" {
		var err error
		if strings.HasSuffix(strings.ToLower(*manifestPath), ".json") {
			manifest, err = loadtest.ParseManifestJSON(*manifestPath)
		} else {
			manifest, err = loadtest.ParseManifestTSV(*manifestPath)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if *machinesFlag > 0 && len(manifest) > *machinesFlag {
			manifest = manifest[:*machinesFlag]
		}
	} else if scenario != "dry-run" && scenario != "prom" && scenario != "mqtt" && scenario != "webhook" && scenario != "admin" {
		fmt.Fprintln(os.Stderr, "avf-loadtest: -manifest required for scenario", scenario)
		os.Exit(2)
	}

	switch scenario {
	case "dry-run":
		printDryRunPlan(scenario, *httpBase, *grpcAddr, *manifestPath, *machinesFlag, *stormWaves)
		os.Exit(0)

	case "prom":
		if strings.TrimSpace(*metricsURL) == "" {
			fmt.Fprintln(os.Stderr, "metrics-url required")
			os.Exit(2)
		}
		snap, err := loadtest.FetchPrometheus(ctx, *metricsURL, *metricsToken)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		printSnap(snap)
		os.Exit(0)

	case "checkin":
		rec := &loadtest.LatencyRecorder{}
		start := time.Now()
		if err := loadtest.ParallelFleet(ctx, manifest, *concurrency, func(ctx context.Context, m loadtest.MachineRow) error {
			return loadtest.RunHTTPCheckIns(ctx, *httpBase, []loadtest.MachineRow{m}, 1, rec)
		}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(loadtestReport(rec, time.Since(start)))

	case "grpc":
		rec := &loadtest.LatencyRecorder{}
		start := time.Now()
		deadline, cancel := context.WithTimeout(ctx, *duration)
		defer cancel()
		if err := loadtest.ParallelFleet(deadline, manifest, *concurrency, func(ctx context.Context, m loadtest.MachineRow) error {
			conn, err := loadtest.GRPCDial(ctx, *grpcAddr)
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()
			return loadtest.RunGRPCRuntime(ctx, conn, m, *productID, int32(*slotIndex), rec)
		}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(loadtestReport(rec, time.Since(start)))

	case "admin":
		org, err := uuid.Parse(strings.TrimSpace(*orgIDStr))
		if err != nil || strings.TrimSpace(*adminJWT) == "" {
			fmt.Fprintln(os.Stderr, "admin scenario needs -admin-jwt and -organization-id")
			os.Exit(2)
		}
		rec := &loadtest.LatencyRecorder{}
		start := time.Now()
		deadline, cancel := context.WithTimeout(ctx, *duration)
		defer cancel()
		for startAt := time.Now(); time.Since(startAt) < *duration; {
			select {
			case <-deadline.Done():
				goto adminDone
			default:
			}
			if err := loadtest.RunHTTPAdminSequence(deadline, *httpBase, *adminJWT, org, 1, rec); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
	adminDone:
		fmt.Println(loadtestReport(rec, time.Since(start)))

	case "webhook":
		orderID, err := uuid.Parse(strings.TrimSpace(*webhookOrder))
		if err != nil {
			fmt.Fprintln(os.Stderr, "webhook-order-id:", err)
			os.Exit(2)
		}
		payID, err := uuid.Parse(strings.TrimSpace(*webhookPayment))
		if err != nil {
			fmt.Fprintln(os.Stderr, "webhook-payment-id:", err)
			os.Exit(2)
		}
		rec := &loadtest.LatencyRecorder{}
		start := time.Now()
		if err := loadtest.WebhookBurst(ctx, *httpBase, *webhookSecret, orderID, payID, *webhookBurst, *webhookDupEvery, rec); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(loadtestReport(rec, time.Since(start)))

	case "mqtt":
		mid := strings.TrimSpace(*mqttMachine)
		if mid == "" {
			fmt.Fprintln(os.Stderr, "mqtt-machine-id required")
			os.Exit(2)
		}
		cmdTopic, ackTopic := loadtest.MQTTTopics(*mqttPrefix, *mqttLayout, mid)
		payload, err := loadtest.MQTTCommandJSON("lt-" + uuid.NewString())
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		rec := &loadtest.LatencyRecorder{}
		start := time.Now()
		if err := loadtest.MQTTCommandScenario(ctx, *mqttBroker, *mqttUser, *mqttPass, 1, cmdTopic, ackTopic, payload, *mqttAckDeadline, rec); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		fmt.Println(loadtestReport(rec, time.Since(start)))

	case "suite", "storm":
		secret := strings.TrimSpace(*webhookSecret)
		skipWh := *skipWebhook || secret == ""
		var orderID, payID uuid.UUID
		if !skipWh {
			var perr error
			orderID, perr = uuid.Parse(strings.TrimSpace(*webhookOrder))
			if perr != nil {
				fmt.Fprintln(os.Stderr, "suite/storm webhook: webhook-order-id:", perr)
				os.Exit(2)
			}
			payID, perr = uuid.Parse(strings.TrimSpace(*webhookPayment))
			if perr != nil {
				fmt.Fprintln(os.Stderr, "suite/storm webhook: webhook-payment-id:", perr)
				os.Exit(2)
			}
		}

		rep, err := loadtest.RunStorm(ctx, loadtest.StormConfig{
			HTTPBase:      *httpBase,
			GRPCAddr:      *grpcAddr,
			Manifest:      manifest,
			Concurrency:   *concurrency,
			Duration:      *duration,
			StormWaves:    *stormWaves,
			AdminJWT:      *adminJWT,
			OrgIDStr:      *orgIDStr,
			ProductID:     *productID,
			SlotIndex:     int32(*slotIndex),
			SkipMQTT:      *skipMQTT,
			SkipWebhook:   skipWh,
			MQTTBroker:    *mqttBroker,
			MQTTUser:      *mqttUser,
			MQTTPass:      *mqttPass,
			MQTTPrefix:    *mqttPrefix,
			MQTTLayout:    *mqttLayout,
			MQTTAckDL:     *mqttAckDeadline,
			WebhookSecret: secret,
			WebhookOrder:  orderID,
			WebhookPay:    payID,
			WebhookBurst:  *webhookBurst,
			WebhookDupN:   *webhookDupEvery,
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		printStormReports(rep)

	default:
		fmt.Fprintln(os.Stderr, "unknown scenario:", scenario)
		os.Exit(2)
	}

	if strings.TrimSpace(*metricsURL) != "" && scenario != "prom" {
		snap, err := loadtest.FetchPrometheus(context.Background(), *metricsURL, *metricsToken)
		if err != nil {
			fmt.Fprintln(os.Stderr, "metrics scrape:", err)
			os.Exit(0)
		}
		fmt.Println("metrics snapshot:")
		printSnap(snap)
	}
}

func printStormReports(r loadtest.StormReports) {
	fmt.Println("--- storm / suite ---")
	fmt.Println(r.CheckIn)
	fmt.Println(r.GRPCSync)
	fmt.Println(r.GRPTelem)
	fmt.Println(r.GRPOffline)
	if r.GRPCCommerce != "" {
		fmt.Println(r.GRPCCommerce)
	}
	fmt.Println(r.MQTT)
	fmt.Println(r.Webhook)
	fmt.Println(r.Admin)
}

func loadtestReport(rec *loadtest.LatencyRecorder, window time.Duration) string {
	return rec.Report(window).String()
}

func printSnap(s loadtest.Snapshot) {
	if s.OutboxPendingTotal != nil {
		fmt.Printf("  outbox_pending_total=%g\n", *s.OutboxPendingTotal)
	}
	if s.OutboxLagSum != nil && s.OutboxLagCount != nil && *s.OutboxLagCount > 0 {
		fmt.Printf("  outbox_lag_seconds_mean=%g (sum=%g count=%g)\n", *s.OutboxLagSum/(*s.OutboxLagCount), *s.OutboxLagSum, *s.OutboxLagCount)
	}
	if s.PaymentWebhookReq != nil {
		fmt.Printf("  avf_commerce_payment_webhook_requests_total(sum)=%g\n", *s.PaymentWebhookReq)
	}
	if s.RedisRateLimitHits != nil {
		fmt.Printf("  avf_redis_rate_limit_hits_total=%g\n", *s.RedisRateLimitHits)
	}
	if s.MQTTAckTimeouts != nil {
		fmt.Printf("  avf_mqtt_command_ack_timeout_total=%g\n", *s.MQTTAckTimeouts)
	}
	if s.DBPoolAcquired != nil {
		fmt.Printf("  avf_db_pool_acquired_conns=%g\n", *s.DBPoolAcquired)
	}
	if s.DBPoolIdle != nil {
		fmt.Printf("  avf_db_pool_idle_conns=%g\n", *s.DBPoolIdle)
	}
	if s.DBPoolTotal != nil {
		fmt.Printf("  avf_db_pool_total_conns=%g\n", *s.DBPoolTotal)
	}
	if s.DBPoolMax != nil {
		fmt.Printf("  avf_db_pool_max_conns=%g\n", *s.DBPoolMax)
	}
}

func printDryRunPlan(scenario, httpBase, grpcAddr, manifestPath string, machines, stormWaves int) {
	fmt.Printf("scenario=%s http=%s grpc=%s manifest=%s machines=%d storm_waves=%d\n", scenario, httpBase, grpcAddr, manifestPath, machines, stormWaves)
}

func envOr(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v != "" {
		return v
	}
	return def
}

func envIntOr(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envBool(key string) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	return v == "1" || v == "true" || v == "yes"
}

func envDurOr(key string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}
