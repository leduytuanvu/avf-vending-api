package payments

import "errors"

var (
	// ErrQueryPaymentStatusNotSupported indicates the provider adapter cannot call a remote status API (reconciler should noop or use HTTP probe fallback).
	ErrQueryPaymentStatusNotSupported = errors.New("payments: query payment status not supported for provider")
	// ErrNotImplemented marks outbound PSP operations that are not wired for this provider build.
	ErrNotImplemented = errors.New("payments: not implemented")
	// ErrSandboxProviderInProduction blocks mock/sandbox PSP keys when APP_ENV=production.
	ErrSandboxProviderInProduction = errors.New("payments: sandbox or mock payment provider is not allowed in production")
	// ErrPaymentProviderRequired is returned when COMMERCE_PAYMENT_PROVIDER is unset in a restricted environment.
	ErrPaymentProviderRequired = errors.New("payments: COMMERCE_PAYMENT_PROVIDER is required")
	// ErrUnknownProvider means the provider key is not registered in the process registry.
	ErrUnknownProvider = errors.New("payments: unknown payment provider")
	// ErrProviderKeyMismatch means the client declared a provider that does not match COMMERCE_PAYMENT_PROVIDER.
	ErrProviderKeyMismatch = errors.New("payments: client provider does not match server COMMERCE_PAYMENT_PROVIDER")
	// ErrLiveProviderNotWired is returned by placeholder adapters until credentials are configured.
	ErrLiveProviderNotWired = errors.New("payments: live provider adapter is not wired for this deployment")
	// ErrInvalidCardSessionProvider blocks cash and other invalid defaults for PSP session flows.
	ErrInvalidCardSessionProvider = errors.New("payments: invalid provider for card/QR payment session")
)
