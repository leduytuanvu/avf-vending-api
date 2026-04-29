package mqtt

import (
	"errors"
	"fmt"
	"net/url"
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
	TopicLayout TopicLayout
	// AppEnv mirrors APP_ENV for TLS policy (optional; empty skips strict checks in Validate).
	AppEnv string

	TLSEnabled         bool
	CAFile             string
	CertFile           string
	KeyFile            string
	ServerName         string
	InsecureSkipVerify bool
}

// LoadBrokerFromEnv reads MQTT_* variables. Defaults TopicPrefix to "avf/devices".
// Client ID resolution matches production Compose, which often sets MQTT_CLIENT_ID from
// MQTT_CLIENT_ID_API / MQTT_CLIENT_ID_INGEST at deploy time; we also accept those names
// directly so local runs and mis-ordered env still wire a non-empty client ID.
func LoadBrokerFromEnv() (BrokerConfig, error) {
	layout, err := parseTopicLayoutEnv(os.Getenv("MQTT_TOPIC_LAYOUT"))
	if err != nil {
		return BrokerConfig{}, err
	}
	return BrokerConfig{
		BrokerURL: strings.TrimSpace(os.Getenv("MQTT_BROKER_URL")),
		ClientID: firstNonEmptyTrimmed(
			os.Getenv("MQTT_CLIENT_ID"),
			os.Getenv("MQTT_CLIENT_ID_API"),
			os.Getenv("MQTT_CLIENT_ID_INGEST"),
		),
		Username:    strings.TrimSpace(os.Getenv("MQTT_USERNAME")),
		Password:    os.Getenv("MQTT_PASSWORD"),
		TopicPrefix: strings.TrimSpace(getenvDefault("MQTT_TOPIC_PREFIX", "avf/devices")),
		TopicLayout: layout,
		AppEnv:      strings.TrimSpace(os.Getenv("APP_ENV")),

		TLSEnabled:         getenvBool("MQTT_TLS_ENABLED", false),
		CAFile:             strings.TrimSpace(os.Getenv("MQTT_CA_FILE")),
		CertFile:           strings.TrimSpace(os.Getenv("MQTT_CERT_FILE")),
		KeyFile:            strings.TrimSpace(os.Getenv("MQTT_KEY_FILE")),
		ServerName:         strings.TrimSpace(os.Getenv("MQTT_SERVER_NAME")),
		InsecureSkipVerify: getenvBool("MQTT_INSECURE_SKIP_VERIFY", false),
	}, nil
}

func parseTopicLayoutEnv(v string) (TopicLayout, error) {
	s := strings.TrimSpace(v)
	if s == "" {
		return TopicLayoutLegacy, nil
	}
	switch strings.ToLower(s) {
	case "legacy":
		return TopicLayoutLegacy, nil
	case "enterprise":
		return TopicLayoutEnterprise, nil
	default:
		return "", fmt.Errorf("mqtt: MQTT_TOPIC_LAYOUT must be legacy or enterprise")
	}
}

func firstNonEmptyTrimmed(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

func getenvDefault(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return def
}

func getenvBool(key string, def bool) bool {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return def
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func brokerAppEnvAllowsInsecureMQTTTLS(appEnv string) bool {
	switch strings.ToLower(strings.TrimSpace(appEnv)) {
	case "", "development", "test":
		return true
	default:
		return false
	}
}

// Validate checks minimal broker settings and TLS policy.
func (c BrokerConfig) Validate() error {
	if strings.TrimSpace(c.BrokerURL) == "" {
		return errors.New("mqtt: MQTT_BROKER_URL is required")
	}
	if _, err := url.Parse(c.BrokerURL); err != nil {
		return fmt.Errorf("mqtt: invalid MQTT_BROKER_URL %q: %w", c.BrokerURL, err)
	}
	if strings.TrimSpace(c.ClientID) == "" {
		return errors.New("mqtt: MQTT_CLIENT_ID is required")
	}
	if strings.TrimSpace(c.TopicPrefix) == "" {
		return errors.New("mqtt: MQTT_TOPIC_PREFIX must be non-empty when set")
	}
	if err := ValidateTopicPrefix(c.TopicPrefix); err != nil {
		return fmt.Errorf("mqtt: MQTT_TOPIC_PREFIX: %w", err)
	}
	if strings.TrimSpace(c.ServerName) != "" && strings.ContainsAny(c.ServerName, " \t\r\n/+#") {
		return errors.New("mqtt: MQTT_SERVER_NAME must be a DNS name without whitespace, slash, or MQTT wildcards")
	}
	if c.InsecureSkipVerify && !brokerAppEnvAllowsInsecureMQTTTLS(c.AppEnv) {
		return errors.New("mqtt: MQTT_INSECURE_SKIP_VERIFY is only allowed when APP_ENV is development or test")
	}
	if (strings.TrimSpace(c.CertFile) != "") != (strings.TrimSpace(c.KeyFile) != "") {
		return errors.New("mqtt: MQTT_CERT_FILE and MQTT_KEY_FILE must both be set for mutual TLS")
	}
	if !brokerAppEnvAllowsInsecureMQTTTLS(c.AppEnv) && !brokerURLImpliesTLS(c.BrokerURL) && !c.TLSEnabled {
		return errors.New("mqtt: staging/production MQTT requires TLS (ssl:// or tls:// MQTT_BROKER_URL, or MQTT_TLS_ENABLED=true)")
	}
	return nil
}
