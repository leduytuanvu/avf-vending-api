package postgres

import (
	"testing"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/stretchr/testify/require"
)

func TestValidatePricingSourceForPersist_acceptsSupportedValues(t *testing.T) {
	for _, source := range []string{
		appcommerce.PricingSourceServerPriced,
		appcommerce.PricingSourceMachineLocalVerified,
		appcommerce.PricingSourceMachineLocalUnverified,
	} {
		got, err := validatePricingSourceForPersist(source)
		require.NoError(t, err)
		require.Equal(t, source, got)
	}
}

func TestValidatePricingSourceForPersist_rejectsEmptyAndUnsupported(t *testing.T) {
	_, err := validatePricingSourceForPersist("")
	require.Error(t, err)

	for _, source := range []string{"LOCAL_SLOT", "LOCAL_BASE", "provider_native_verified"} {
		_, err := validatePricingSourceForPersist(source)
		require.Error(t, err, source)
	}
}

func TestResolvePricingSourceForOrderPersist_defaultsServerPricedWithoutMachineEvidence(t *testing.T) {
	got, err := resolvePricingSourceForOrderPersist("", false)
	require.NoError(t, err)
	require.Equal(t, appcommerce.PricingSourceServerPriced, got)
}

func TestResolvePricingSourceForOrderPersist_requiresExplicitSourceWithMachineEvidence(t *testing.T) {
	_, err := resolvePricingSourceForOrderPersist("", true)
	require.Error(t, err)

	got, err := resolvePricingSourceForOrderPersist(appcommerce.PricingSourceMachineLocalVerified, true)
	require.NoError(t, err)
	require.Equal(t, appcommerce.PricingSourceMachineLocalVerified, got)
}
