package reporting

import (
	"testing"
	"time"
)

func TestValidateReportingWindow(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	if err := ValidateReportingWindow(from, to); err != nil {
		t.Fatalf("valid window: %v", err)
	}
	if err := ValidateReportingWindow(to, from); err == nil {
		t.Fatal("expected error when from >= to")
	}
	longTo := from.Add(400 * 24 * time.Hour)
	if err := ValidateReportingWindow(from, longTo); err == nil {
		t.Fatal("expected error when span > 366 days")
	}
}
