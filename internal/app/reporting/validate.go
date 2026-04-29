package reporting

import (
	"fmt"
	"time"
)

// DefaultReportingWindow is the maximum synchronous reporting span when HTTPReportingSyncMaxSpan is unset (366 days inclusive span).
const DefaultReportingWindow = 366 * 24 * time.Hour

// ValidateReportingWindow enforces a non-empty half-open window [from, to) with a maximum span.
// When maxSpan <= 0, DefaultReportingWindow applies.
func ValidateReportingWindow(from, to time.Time, maxSpan time.Duration) error {
	if from.IsZero() || to.IsZero() {
		return fmt.Errorf("from and to are required (RFC3339)")
	}
	if !from.Before(to) {
		return fmt.Errorf("from must be strictly before to (half-open range)")
	}
	limit := maxSpan
	if limit <= 0 {
		limit = DefaultReportingWindow
	}
	if to.Sub(from) > limit {
		return fmt.Errorf("date range exceeds maximum reporting window (%s)", limit.String())
	}
	return nil
}
