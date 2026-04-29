package reporting

import (
	"testing"
	"time"
)

func TestValidateReportingWindow(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	if err := ValidateReportingWindow(from, to, DefaultReportingWindow); err != nil {
		t.Fatalf("valid window: %v", err)
	}
	if err := ValidateReportingWindow(to, from, DefaultReportingWindow); err == nil {
		t.Fatal("expected error when from >= to")
	}
	longTo := from.Add(400 * 24 * time.Hour)
	if err := ValidateReportingWindow(from, longTo, DefaultReportingWindow); err == nil {
		t.Fatal("expected error when span > 366 days")
	}
}

func TestValidateReportingWindow_customMaxSpan(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(48 * time.Hour)
	maxSpan := 24 * time.Hour
	if err := ValidateReportingWindow(from, to, maxSpan); err == nil {
		t.Fatal("expected error when span exceeds custom max")
	}
	if err := ValidateReportingWindow(from, from.Add(12*time.Hour), maxSpan); err != nil {
		t.Fatalf("expected ok inside custom span: %v", err)
	}
}
