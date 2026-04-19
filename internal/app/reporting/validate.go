package reporting

import (
	"fmt"
	"time"
)

// MaxReportingWindow caps reporting ranges to avoid unbounded scans (366 days inclusive span).
const MaxReportingWindow = 366 * 24 * time.Hour

// ValidateReportingWindow enforces a non-empty half-open window [from, to) with a maximum span.
func ValidateReportingWindow(from, to time.Time) error {
	if from.IsZero() || to.IsZero() {
		return fmt.Errorf("from and to are required (RFC3339)")
	}
	if !from.Before(to) {
		return fmt.Errorf("from must be strictly before to (half-open range)")
	}
	if to.Sub(from) > MaxReportingWindow {
		return fmt.Errorf("date range exceeds maximum of 366 days")
	}
	return nil
}
