package loadtest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// MQTTCommandScenario publishes one command and waits for first ACK payload on ackTopic.
func MQTTCommandScenario(ctx context.Context, brokerURL, username, password string, qos byte, commandTopic, ackTopic string, payload []byte, ackDeadline time.Duration, recorder *LatencyRecorder) error {
	opts := mqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetConnectTimeout(10 * time.Second).
		SetOrderMatters(false).
		SetAutoReconnect(false)
	if username != "" {
		opts.SetUsername(username).SetPassword(password)
	}
	cli := mqtt.NewClient(opts)
	token := cli.Connect()
	if !token.WaitTimeout(15 * time.Second) {
		return fmt.Errorf("mqtt connect timeout")
	}
	if token.Error() != nil {
		return token.Error()
	}
	defer cli.Disconnect(200)

	got := make(chan string, 2)
	var once sync.Once
	cb := func(_ mqtt.Client, msg mqtt.Message) {
		once.Do(func() {
			got <- string(msg.Payload())
		})
	}
	subTok := cli.Subscribe(ackTopic, qos, cb)
	subTok.WaitTimeout(15 * time.Second)
	if subTok.Error() != nil {
		return fmt.Errorf("mqtt subscribe: %w", subTok.Error())
	}

	start := time.Now()
	pubTok := cli.Publish(commandTopic, qos, false, payload)
	pubTok.WaitTimeout(15 * time.Second)
	if pubTok.Error() != nil {
		recorder.Add(time.Since(start), true)
		return pubTok.Error()
	}

	select {
	case <-got:
		recorder.Add(time.Since(start), false)
		return nil
	case <-time.After(ackDeadline):
		recorder.Add(time.Since(start), true)
		return fmt.Errorf("ack deadline exceeded")
	case <-ctx.Done():
		recorder.Add(time.Since(start), true)
		return ctx.Err()
	}
}

// MQTTTopics returns enterprise-style command + ack topics for a machine UUID string.
func MQTTTopics(topicPrefix, layout, machineID string) (commandTopic, ackTopic string) {
	prefix := strings.TrimRight(strings.TrimSpace(topicPrefix), "/")
	switch strings.TrimSpace(strings.ToLower(layout)) {
	case "enterprise":
		return fmt.Sprintf("%s/machines/%s/commands", prefix, machineID),
			fmt.Sprintf("%s/machines/%s/commands/ack", prefix, machineID)
	default:
		return fmt.Sprintf("%s/%s/commands", prefix, machineID),
			fmt.Sprintf("%s/%s/commands/ack", prefix, machineID)
	}
}

// MQTTCommandJSON builds the diagnostic ping envelope used by mqtt smoke scripts.
func MQTTCommandJSON(commandID string) ([]byte, error) {
	m := map[string]any{
		"command_id": commandID,
		"type":       "diagnostic.ping",
		"payload":    map[string]any{},
		"sent_at":    time.Now().UTC().Format(time.RFC3339),
	}
	return json.Marshal(m)
}
