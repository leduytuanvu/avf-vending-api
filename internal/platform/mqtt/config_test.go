package mqtt

import (
	"testing"
)

func TestLoadBrokerFromEnv_clientIDFallback(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("MQTT_BROKER_URL", "tcp://emqx:1883")
	t.Setenv("MQTT_CLIENT_ID", "")
	t.Setenv("MQTT_CLIENT_ID_API", "from-api-var")
	t.Setenv("MQTT_CLIENT_ID_INGEST", "from-ingest-var")
	t.Setenv("MQTT_USERNAME", "u")
	t.Setenv("MQTT_PASSWORD", "p")
	t.Setenv("MQTT_SERVER_NAME", "mqtt.example.com")

	cfg, err := LoadBrokerFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ClientID != "from-api-var" {
		t.Fatalf("expected ClientID from MQTT_CLIENT_ID_API, got %q", cfg.ClientID)
	}
	if cfg.ServerName != "mqtt.example.com" {
		t.Fatalf("expected server name from env, got %q", cfg.ServerName)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestBrokerConfig_validateRejectsWildcardPrefix(t *testing.T) {
	cfg := BrokerConfig{
		BrokerURL:   "tcp://localhost:1883",
		ClientID:    "api-dev",
		TopicPrefix: "avf/+/devices",
		AppEnv:      "development",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected wildcard topic prefix to be rejected")
	}
}

func TestBrokerConfig_buildTLSConfigServerName(t *testing.T) {
	cfg := BrokerConfig{
		BrokerURL:   "ssl://emqx:8883",
		ClientID:    "api-prod",
		TopicPrefix: "avf/devices",
		AppEnv:      "production",
		ServerName:  "mqtt.example.com",
	}
	tc, err := cfg.buildTLSConfig()
	if err != nil {
		t.Fatal(err)
	}
	if tc == nil || tc.ServerName != "mqtt.example.com" {
		t.Fatalf("expected tls config server name, got %#v", tc)
	}
}

func TestLoadBrokerFromEnv_clientIDPrimaryWins(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("MQTT_BROKER_URL", "tcp://emqx:1883")
	t.Setenv("MQTT_CLIENT_ID", "primary-id")
	t.Setenv("MQTT_CLIENT_ID_API", "ignored")
	t.Setenv("MQTT_USERNAME", "u")
	t.Setenv("MQTT_PASSWORD", "p")

	cfg, err := LoadBrokerFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ClientID != "primary-id" {
		t.Fatalf("expected primary MQTT_CLIENT_ID, got %q", cfg.ClientID)
	}
}

func TestLoadBrokerFromEnv_ingestFallback(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("MQTT_BROKER_URL", "tcp://emqx:1883")
	t.Setenv("MQTT_CLIENT_ID", "")
	t.Setenv("MQTT_CLIENT_ID_API", "")
	t.Setenv("MQTT_CLIENT_ID_INGEST", "ingest-only")
	t.Setenv("MQTT_USERNAME", "u")
	t.Setenv("MQTT_PASSWORD", "p")

	cfg, err := LoadBrokerFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ClientID != "ingest-only" {
		t.Fatalf("expected ClientID from MQTT_CLIENT_ID_INGEST, got %q", cfg.ClientID)
	}
}

func TestLoadBrokerFromEnv_invalidTopicLayout(t *testing.T) {
	t.Setenv("MQTT_TOPIC_LAYOUT", "nope")
	_, err := LoadBrokerFromEnv()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBrokerConfig_validateInsecureSkipVerifyProductionRejected(t *testing.T) {
	cfg := BrokerConfig{
		BrokerURL:          "ssl://emqx:8883",
		ClientID:           "c1",
		TopicPrefix:        "avf/devices",
		AppEnv:             "production",
		InsecureSkipVerify: true,
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error")
	}
}

func TestBrokerConfig_validateProductionRequiresTLS(t *testing.T) {
	cfg := BrokerConfig{
		BrokerURL:   "tcp://emqx:1883",
		ClientID:    "api-prod",
		TopicPrefix: "avf/devices",
		AppEnv:      "production",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected production plain MQTT to be rejected")
	}

	cfg.TLSEnabled = true
	if err := cfg.Validate(); err != nil {
		t.Fatalf("MQTT_TLS_ENABLED should allow TCP URL with TLS config: %v", err)
	}
}

func TestBrokerConfig_validateDevelopmentAllowsPlainMQTT(t *testing.T) {
	cfg := BrokerConfig{
		BrokerURL:   "tcp://localhost:1883",
		ClientID:    "api-dev",
		TopicPrefix: "avf/devices",
		AppEnv:      "development",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("development plain MQTT should remain usable: %v", err)
	}
}

func TestCommandAttemptStatusLifecycleMapping(t *testing.T) {
	cases := map[string]CommandLedgerLifecycleState{
		"pending":     CommandLifecycleQueued,
		"sent":        CommandLifecyclePublished,
		"completed":   CommandLifecycleAcked,
		"ack_timeout": CommandLifecycleFailed,
		"expired":     CommandLifecycleExpired,
		"cancelled":   CommandLifecycleCanceled,
	}
	for in, want := range cases {
		got, ok := MapCommandAttemptStatusToLifecycle(in)
		if !ok || got != want {
			t.Fatalf("%s mapped to (%q,%v), want (%q,true)", in, got, ok, want)
		}
	}
}
