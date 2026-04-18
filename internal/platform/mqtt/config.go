package mqtt

import (
	"errors"
	"os"
	"strings"
)

// BrokerConfig is loaded from the environment by ingest workers (no shared config package coupling).
type BrokerConfig struct {
	BrokerURL   string
	ClientID    string
	Username    string
	Password    string
	TopicPrefix string
}

// LoadBrokerFromEnv reads MQTT_* variables. Defaults TopicPrefix to "avf/devices".
func LoadBrokerFromEnv() BrokerConfig {
	return BrokerConfig{
		BrokerURL:   strings.TrimSpace(os.Getenv("MQTT_BROKER_URL")),
		ClientID:    strings.TrimSpace(os.Getenv("MQTT_CLIENT_ID")),
		Username:    strings.TrimSpace(os.Getenv("MQTT_USERNAME")),
		Password:    os.Getenv("MQTT_PASSWORD"),
		TopicPrefix: strings.TrimSpace(getenvDefault("MQTT_TOPIC_PREFIX", "avf/devices")),
	}
}

func getenvDefault(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return def
}

// Validate checks minimal broker settings.
func (c BrokerConfig) Validate() error {
	if strings.TrimSpace(c.BrokerURL) == "" {
		return errors.New("mqtt: MQTT_BROKER_URL is required")
	}
	if strings.TrimSpace(c.ClientID) == "" {
		return errors.New("mqtt: MQTT_CLIENT_ID is required")
	}
	if strings.TrimSpace(c.TopicPrefix) == "" {
		return errors.New("mqtt: MQTT_TOPIC_PREFIX must be non-empty when set")
	}
	return nil
}
