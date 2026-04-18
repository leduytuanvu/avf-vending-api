package mqtt

import (
	"testing"
)

func TestLoadBrokerFromEnv_clientIDFallback(t *testing.T) {
	t.Setenv("MQTT_BROKER_URL", "tcp://emqx:1883")
	t.Setenv("MQTT_CLIENT_ID", "")
	t.Setenv("MQTT_CLIENT_ID_API", "from-api-var")
	t.Setenv("MQTT_CLIENT_ID_INGEST", "from-ingest-var")
	t.Setenv("MQTT_USERNAME", "u")
	t.Setenv("MQTT_PASSWORD", "p")

	cfg := LoadBrokerFromEnv()
	if cfg.ClientID != "from-api-var" {
		t.Fatalf("expected ClientID from MQTT_CLIENT_ID_API, got %q", cfg.ClientID)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestLoadBrokerFromEnv_clientIDPrimaryWins(t *testing.T) {
	t.Setenv("MQTT_BROKER_URL", "tcp://emqx:1883")
	t.Setenv("MQTT_CLIENT_ID", "primary-id")
	t.Setenv("MQTT_CLIENT_ID_API", "ignored")
	t.Setenv("MQTT_USERNAME", "u")
	t.Setenv("MQTT_PASSWORD", "p")

	cfg := LoadBrokerFromEnv()
	if cfg.ClientID != "primary-id" {
		t.Fatalf("expected primary MQTT_CLIENT_ID, got %q", cfg.ClientID)
	}
}

func TestLoadBrokerFromEnv_ingestFallback(t *testing.T) {
	t.Setenv("MQTT_BROKER_URL", "tcp://emqx:1883")
	t.Setenv("MQTT_CLIENT_ID", "")
	t.Setenv("MQTT_CLIENT_ID_API", "")
	t.Setenv("MQTT_CLIENT_ID_INGEST", "ingest-only")
	t.Setenv("MQTT_USERNAME", "u")
	t.Setenv("MQTT_PASSWORD", "p")

	cfg := LoadBrokerFromEnv()
	if cfg.ClientID != "ingest-only" {
		t.Fatalf("expected ClientID from MQTT_CLIENT_ID_INGEST, got %q", cfg.ClientID)
	}
}
