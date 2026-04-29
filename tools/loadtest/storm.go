package loadtest

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// StormConfig is the full P1.5 reconnect/load orchestration (sequential phases).
type StormConfig struct {
	HTTPBase      string
	GRPCAddr      string
	Manifest      []MachineRow
	Concurrency   int
	Duration      time.Duration
	StormWaves    int
	AdminJWT      string
	OrgIDStr      string
	ProductID     string
	SlotIndex     int32
	SkipMQTT      bool
	SkipWebhook   bool
	MQTTBroker    string
	MQTTUser      string
	MQTTPass      string
	MQTTPrefix    string
	MQTTLayout    string
	MQTTAckDL     time.Duration
	WebhookSecret string
	WebhookOrder  uuid.UUID
	WebhookPay    uuid.UUID
	WebhookBurst  int
	WebhookDupN   int
}

// StormReports is stdout-friendly phase output (for docs / log capture).
type StormReports struct {
	CheckIn      string
	GRPCSync     string
	GRPTelem     string
	GRPOffline   string
	GRPCCommerce string
	MQTT         string
	Webhook      string
	Admin        string
}

// RunStorm executes all fleet-scale phases in order and returns printable lines.
func RunStorm(ctx context.Context, c StormConfig) (StormReports, error) {
	var rep StormReports
	if len(c.Manifest) == 0 {
		return rep, fmt.Errorf("storm: empty manifest")
	}
	waves := c.StormWaves
	if waves < 1 {
		waves = 1
	}

	// 1) Check-in reconnect storm (multiple waves × fleet).
	checkAgg := &LatencyRecorder{}
	tCheck := time.Now()
	for w := 0; w < waves; w++ {
		wave := &LatencyRecorder{}
		if err := ParallelFleet(ctx, c.Manifest, c.Concurrency, func(actx context.Context, m MachineRow) error {
			return RunHTTPCheckIns(actx, c.HTTPBase, []MachineRow{m}, 1, wave)
		}); err != nil {
			return rep, fmt.Errorf("check-in wave %d: %w", w+1, err)
		}
		checkAgg.Merge(wave)
	}
	rep.CheckIn = phaseReport("checkin_reconnect", checkAgg, time.Since(tCheck))

	// 2–4) gRPC: sync, telemetry, offline (distinct recorders); optional commerce.
	syncR := &LatencyRecorder{}
	telR := &LatencyRecorder{}
	offR := &LatencyRecorder{}
	var commR *LatencyRecorder
	if strings.TrimSpace(c.ProductID) != "" {
		commR = &LatencyRecorder{}
	}
	tGRPC := time.Now()
	if err := ParallelFleet(ctx, c.Manifest, c.Concurrency, func(actx context.Context, m MachineRow) error {
		conn, derr := GRPCDial(actx, c.GRPCAddr)
		if derr != nil {
			return derr
		}
		defer func() { _ = conn.Close() }()
		ph := &GRPCPhasedRecorders{Sync: syncR, Telemetry: telR, Offline: offR, Commerce: commR}
		return RunGRPCRuntimePhased(actx, conn, m, c.ProductID, c.SlotIndex, ph)
	}); err != nil {
		return rep, fmt.Errorf("grpc storm: %w", err)
	}
	win := time.Since(tGRPC)
	rep.GRPCSync = phaseReport("grpc_bootstrap_catalog_media", syncR, win)
	rep.GRPTelem = phaseReport("grpc_telemetry_batch", telR, win)
	rep.GRPOffline = phaseReport("grpc_offline_replay", offR, win)
	if commR != nil {
		rep.GRPCCommerce = phaseReport("grpc_commerce_chain", commR, win)
	}

	// 5) MQTT command dispatch + ACK latency (fleet uses each machine's UUID for topics).
	if !c.SkipMQTT {
		mqttR := &LatencyRecorder{}
		tMQTT := time.Now()
		mqttErr := ParallelFleet(ctx, c.Manifest, c.Concurrency, func(actx context.Context, m MachineRow) error {
			cmdTopic, ackTopic := MQTTTopics(c.MQTTPrefix, c.MQTTLayout, m.MachineID.String())
			payload, perr := MQTTCommandJSON("lt-" + uuid.NewString())
			if perr != nil {
				return perr
			}
			return MQTTCommandScenario(actx, c.MQTTBroker, c.MQTTUser, c.MQTTPass, 1, cmdTopic, ackTopic, payload, c.MQTTAckDL, mqttR)
		})
		rep.MQTT = phaseReport("mqtt_command_ack", mqttR, time.Since(tMQTT))
		if mqttErr != nil {
			rep.MQTT += fmt.Sprintf(" (first_error=%v)", mqttErr)
		}
	} else {
		rep.MQTT = "mqtt_command_ack: skipped (LOADTEST_SKIP_MQTT or -skip-mqtt)"
	}

	// 6) Payment webhook burst + duplicate replay.
	if !c.SkipWebhook && strings.TrimSpace(c.WebhookSecret) != "" {
		wh := &LatencyRecorder{}
		tWh := time.Now()
		if err := WebhookBurst(ctx, c.HTTPBase, c.WebhookSecret, c.WebhookOrder, c.WebhookPay, c.WebhookBurst, c.WebhookDupN, wh); err != nil {
			rep.Webhook = fmt.Sprintf("webhook_burst: error=%v", err)
		} else {
			rep.Webhook = phaseReport("webhook_signed_burst", wh, time.Since(tWh))
		}
	} else {
		rep.Webhook = "webhook_signed_burst: skipped (set COMMERCE_PAYMENT_WEBHOOK_SECRET + order/payment UUIDs)"
	}

	// 7) Admin dashboard / list / report reads (duration-bound).
	org, err := uuid.Parse(strings.TrimSpace(c.OrgIDStr))
	if err == nil && strings.TrimSpace(c.AdminJWT) != "" {
		adm := &LatencyRecorder{}
		tAdm := time.Now()
		admDeadline, admCancel := context.WithTimeout(ctx, c.Duration)
		defer admCancel()
		var admErr error
		for admDeadline.Err() == nil {
			admErr = RunHTTPAdminSequence(admDeadline, c.HTTPBase, c.AdminJWT, org, 1, adm)
			if admErr != nil {
				break
			}
		}
		rep.Admin = phaseReport("admin_read_pressure", adm, time.Since(tAdm))
		if admErr != nil {
			rep.Admin += fmt.Sprintf(" (last_error=%v)", admErr)
		}
	} else {
		rep.Admin = "admin_read_pressure: skipped (set ADMIN_JWT + LOADTEST_ORGANIZATION_ID)"
	}

	return rep, nil
}

func phaseReport(name string, rec *LatencyRecorder, window time.Duration) string {
	if rec == nil {
		return fmt.Sprintf("%s: (no samples)", name)
	}
	return fmt.Sprintf("%s: %s", name, rec.Report(window).String())
}
