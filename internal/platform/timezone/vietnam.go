// Package timezone asserts bundled IANA zones required for Vietnam business reporting.
package timezone

import (
	"fmt"
	_ "time/tzdata"

	"time"
)

const (
	// VietnamBusiness is the canonical reporting and site timezone for AVF deployments.
	VietnamBusiness = "Asia/Ho_Chi_Minh"
)

// AssertRequired loads reporting zones from the embedded tzdata bundle.
// Fails fast when the production image lacks zone files (T-P2-2 guard).
func AssertRequired() error {
	for _, name := range []string{VietnamBusiness, "UTC"} {
		if _, err := time.LoadLocation(name); err != nil {
			return fmt.Errorf("timezone: LoadLocation(%q): %w", name, err)
		}
	}
	return nil
}
