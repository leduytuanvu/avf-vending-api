package grpcserver

import (
	"testing"

	"github.com/avf/avf-vending-api/internal/config"
	platformmqtt "github.com/avf/avf-vending-api/internal/platform/mqtt"
	platformpayments "github.com/avf/avf-vending-api/internal/platform/payments"
	"github.com/stretchr/testify/require"
)

func TestMapPaymentMethodsProto_CashOnly(t *testing.T) {
	t.Parallel()
	out := mapPaymentMethodsProto(platformpayments.ResolveMachinePaymentMethods(
		&config.Config{AppEnv: config.AppEnvProduction, PaymentEnv: config.PaymentEnvCashOnly},
		platformpayments.NewRegistry(&config.Config{PaymentEnv: config.PaymentEnvCashOnly}),
		nil,
	))
	require.NotNil(t, out)
	require.True(t, out.GetCashEnabled())
	require.False(t, out.GetQrCardEnabled())
	require.Equal(t, platformpayments.PaymentModeCashOnly, out.GetPaymentMode())
	require.Equal(t, platformpayments.QRCardUnavailableReasonProviderUnavailable, out.GetQrCardUnavailableReason())
	require.Equal(t, platformpayments.ProviderStatusUnavailable, out.GetCardQrProviderStatus())
}

func TestMapMqttConfigMetadataProto_EnterpriseProduction(t *testing.T) {
	t.Parallel()
	deps := MachineGRPCServicesDeps{
		MQTTBrokerURL:   "tls://mqtt.example:8883",
		MQTTTopicPrefix: "avf/production",
		Config: &config.Config{
			AppEnv: config.AppEnvProduction,
			MQTT: config.MQTTConfig{
				TopicLayout: "enterprise",
				TLSEnabled:  true,
			},
		},
	}
	out := mapMqttConfigMetadataProto(deps)
	require.NotNil(t, out)
	require.Equal(t, "tls://mqtt.example:8883", out.GetBrokerUrl())
	require.Equal(t, "avf/production", out.GetTopicPrefix())
	require.Equal(t, "enterprise", out.GetTopicLayout())
	require.True(t, out.GetTlsRequired())
	require.Equal(t, platformmqtt.MachineClientIDPolicyTemplate, out.GetClientIdPolicy())
}
