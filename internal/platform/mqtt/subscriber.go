package mqtt

import (
	"context"
	"fmt"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"go.uber.org/zap"
)

// IngestHooks carries optional post-dispatch callbacks (e.g. Prometheus in cmd/mqtt-ingest).
type IngestHooks struct {
	// OnDispatchOutcome is invoked after each MQTT message Dispatch to ing; success false when Dispatch returned an error.
	OnDispatchOutcome func(success bool, topic string, payloadBytes int)
	// OnIngressRejected is invoked when Dispatch rejects a message before DeviceIngest (validation, size, complexity).
	OnIngressRejected func(topic string, reason string, payloadBytes int)
}

// Subscriber connects to a broker and routes device publications to DeviceIngest.
type Subscriber struct {
	cfg    BrokerConfig
	log    *zap.Logger
	hooks  *IngestHooks
	limits *TelemetryIngressLimits
}

// NewSubscriber validates cfg and returns a subscriber handle. hooks and limits may be nil.
func NewSubscriber(cfg BrokerConfig, log *zap.Logger, hooks *IngestHooks, limits *TelemetryIngressLimits) (*Subscriber, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Subscriber{cfg: cfg, log: log, hooks: hooks, limits: limits}, nil
}

// Run connects, subscribes to device topics, and blocks until ctx is cancelled.
//
// Reconnect: uses Paho AutoReconnect + ConnectRetry (5s) by default. On each successful connect,
// subscriptions are (re)applied at QoS 1 via OnConnectHandler. Session state uses the Paho default
// (clean start / non-persistent session unless changed in a future config knob). After a broker
// restart, expect a disconnect and full resubscribe; in-flight QoS 1 server-to-client messages may
// be redelivered; application dedupe stays responsibility of device receipts and telemetry keys.
//
// Ops: warn logs "mqtt ingest failed" / "mqtt subscribe failed"; optional hooks for Prometheus; see ops/METRICS.md.
func (s *Subscriber) Run(ctx context.Context, ing DeviceIngest) error {
	topics := InboundTopicPatterns(s.cfg.TopicLayout, s.cfg.TopicPrefix)

	opts := pahomqtt.NewClientOptions()
	opts.AddBroker(s.cfg.BrokerURL)
	opts.SetClientID(s.cfg.ClientID)
	if s.cfg.Username != "" {
		opts.SetUsername(s.cfg.Username)
		opts.SetPassword(s.cfg.Password)
	}
	opts.SetKeepAlive(30 * time.Second)
	opts.SetPingTimeout(10 * time.Second)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(5 * time.Second)
	opts.SetOrderMatters(false)

	if err := s.cfg.applySecurity(opts); err != nil {
		return fmt.Errorf("mqtt tls: %w", err)
	}

	opts.SetDefaultPublishHandler(func(_ pahomqtt.Client, msg pahomqtt.Message) {
		if msg == nil {
			return
		}
		n := len(msg.Payload())
		err := Dispatch(ctx, s.cfg.TopicLayout, s.cfg.TopicPrefix, msg.Topic(), msg.Payload(), ing, s.limits, s.hooks)
		if s.hooks != nil && s.hooks.OnDispatchOutcome != nil {
			s.hooks.OnDispatchOutcome(err == nil, msg.Topic(), n)
		}
		if err != nil && s.log != nil {
			s.log.Warn("mqtt ingest failed",
				zap.Error(err),
				zap.String("topic", msg.Topic()),
				zap.Int("payload_bytes", n),
			)
		}
	})

	opts.SetOnConnectHandler(func(c pahomqtt.Client) {
		for _, t := range topics {
			if token := c.Subscribe(t, 1, nil); token.Wait() && token.Error() != nil && s.log != nil {
				s.log.Error("mqtt subscribe failed", zap.String("topic", t), zap.Error(token.Error()))
			} else if s.log != nil {
				s.log.Info("mqtt subscribed", zap.String("topic", t))
			}
		}
	})

	client := pahomqtt.NewClient(opts)
	token := client.Connect()
	token.Wait()
	if token.Error() != nil {
		return fmt.Errorf("mqtt connect: %w", token.Error())
	}
	defer client.Disconnect(250)

	if s.log != nil {
		s.log.Info("mqtt ingest connected", zap.String("broker", s.cfg.BrokerURL), zap.Strings("patterns", topics))
	}

	<-ctx.Done()
	return ctx.Err()
}
