package config_test

import (
	"testing"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestMoMoCallbacksWired_requiresIPNAndRedirectPerTenant(t *testing.T) {
	wired := config.MoMoConfig{
		AVF: config.MoMoTenantCredentials{
			PartnerCode: "p",
			AccessKey:   "a",
			SecretKey:   "s",
			Endpoint:    "https://momo.test",
			IPNURL:      "https://api.test/v1/commerce/webhooks/momo",
			RedirectURL: "https://api.test/return",
		},
	}
	require.True(t, wired.MoMoWired())
	require.True(t, wired.MoMoCallbacksWired())

	missingRedirect := wired
	missingRedirect.AVF.RedirectURL = ""
	require.False(t, missingRedirect.MoMoCallbacksWired())
}

func TestZaloPayCallbacksWired_requiresCallbackURL(t *testing.T) {
	wired := config.ZaloPayConfig{
		AppID:       "1",
		Key1:        "k1",
		Key2:        "k2",
		Endpoint:    "https://zalopay.test",
		CallbackURL: "https://api.test/v1/commerce/webhooks/zalopay",
	}
	require.True(t, wired.ZaloPayWired())
	require.True(t, wired.ZaloPayCallbacksWired())

	missing := wired
	missing.CallbackURL = ""
	require.False(t, missing.ZaloPayCallbacksWired())
}
