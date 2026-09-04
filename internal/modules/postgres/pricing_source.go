package postgres

import (
	"errors"
	"fmt"
	"strings"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
)

var supportedPricingSources = map[string]struct{}{
	appcommerce.PricingSourceServerPriced:           {},
	appcommerce.PricingSourceMachineLocalVerified:   {},
	appcommerce.PricingSourceMachineLocalUnverified: {},
}

// validatePricingSourceForPersist rejects empty or unsupported values before they reach orders_pricing_source_check.
func validatePricingSourceForPersist(raw string) (string, error) {
	source := strings.TrimSpace(raw)
	if source == "" {
		return "", errors.New("postgres: pricing_source required")
	}
	if _, ok := supportedPricingSources[source]; !ok {
		return "", fmt.Errorf("postgres: unsupported pricing_source %q", source)
	}
	return source, nil
}

func hasMachinePricingEvidence(revision *int64, snapshot []byte) bool {
	if len(snapshot) > 0 {
		return true
	}
	if revision != nil && *revision > 0 {
		return true
	}
	return false
}

// resolvePricingSourceForOrderPersist defaults online server-priced orders only when no machine pricing evidence is present.
func resolvePricingSourceForOrderPersist(explicit string, machinePricingEvidence bool) (string, error) {
	trimmed := strings.TrimSpace(explicit)
	if trimmed != "" {
		return validatePricingSourceForPersist(trimmed)
	}
	if machinePricingEvidence {
		return "", errors.New("postgres: pricing_source required when machine pricing snapshot is present")
	}
	return appcommerce.PricingSourceServerPriced, nil
}
