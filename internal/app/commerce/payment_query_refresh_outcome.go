package commerce

// PaymentQueryRefreshOutcome summarizes a provider query refresh attempt for machine-visible diagnostics.
type PaymentQueryRefreshOutcome struct {
	Diagnostic string
	Skipped    bool
}
