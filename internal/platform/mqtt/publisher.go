package mqtt

import (
	"context"
	"fmt"
	"strings"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Publisher is a thin MQTT client for outbound publishes (API process). It is separate from ingest Subscriber.
type Publisher struct {
	cfg    BrokerConfig
	log    *zap.Logger
	client pahomqtt.Client
}

// NewPublisher connects to the broker using cfg (must pass Validate). clientIDSuffix is appended to cfg.ClientID.
func NewPublisher(cfg BrokerConfig, log *zap.Logger, clientIDSuffix string) (*Publisher, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	cid := strings.TrimSpace(cfg.ClientID)
	if cid == "" {
		return nil, fmt.Errorf("mqtt: publisher requires MQTT_CLIENT_ID")
	}
	if strings.TrimSpace(clientIDSuffix) != "" {
		cid = cid + clientIDSuffix
	}

	opts := pahomqtt.NewClientOptions()
	opts.AddBroker(cfg.BrokerURL)
	opts.SetClientID(cid)
	if cfg.Username != "" {
		opts.SetUsername(cfg.Username)
		opts.SetPassword(cfg.Password)
	}
	opts.SetKeepAlive(30 * time.Second)
	opts.SetPingTimeout(10 * time.Second)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(5 * time.Second)
	opts.SetOrderMatters(false)

	client := pahomqtt.NewClient(opts)
	token := client.Connect()
	token.Wait()
	if token.Error() != nil {
		return nil, fmt.Errorf("mqtt publisher connect: %w", token.Error())
	}

	return &Publisher{cfg: cfg, log: log, client: client}, nil
}

// Close disconnects the MQTT session.
func (p *Publisher) Close() {
	if p == nil || p.client == nil {
		return
	}
	p.client.Disconnect(250)
}

// Health returns nil when the client reports an active broker session.
func (p *Publisher) Health(ctx context.Context) error {
	_ = ctx
	if p == nil || p.client == nil {
		return fmt.Errorf("mqtt: nil publisher")
	}
	if !p.client.IsConnectionOpen() {
		return fmt.Errorf("mqtt: publisher not connected")
	}
	return nil
}

// PublishDeviceDispatch publishes JSON payload to the machine commands/dispatch channel (QoS 1, not retained).
func (p *Publisher) PublishDeviceDispatch(ctx context.Context, machineID uuid.UUID, payload []byte) error {
	_ = ctx
	if p == nil || p.client == nil {
		return fmt.Errorf("mqtt: nil publisher")
	}
	topic := OutboundCommandDispatchTopic(p.cfg.TopicPrefix, machineID)
	tok := p.client.Publish(topic, 1, false, payload)
	tok.Wait()
	if tok.Error() != nil {
		if p.log != nil {
			p.log.Warn("mqtt publish failed", zap.String("topic", topic), zap.Error(tok.Error()))
		}
		return fmt.Errorf("mqtt publish: %w", tok.Error())
	}
	return nil
}
