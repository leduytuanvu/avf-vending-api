package clockskew

import (
	"fmt"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
)

// Stable rejection reason codes for device-supplied timestamps.
const (
	ReasonFuture      = "clock_skew_future"
	ReasonPast        = "clock_skew_past"
	ReasonImplausible = "clock_skew_implausible"
)

// Violation describes a rejected device timestamp.
type Violation struct {
	Reason string
}

func (v Violation) Error() string {
	return v.Reason
}

// ValidateCheckIn bounds live check-in / heartbeat occurred_at against server now.
func ValidateCheckIn(occurredAt, receivedAt time.Time, cfg config.DeviceClockSkewConfig) error {
	return validateWindow(occurredAt, receivedAt, cfg.CheckInPastMax, cfg.CheckInFutureMax, 0, time.Time{})
}

// ValidateOfflineEvent bounds offline sync occurred_at. Past backlog is allowed; future and implausible values are rejected.
func ValidateOfflineEvent(occurredAt, receivedAt time.Time, machineCreatedAt time.Time, cfg config.DeviceClockSkewConfig) error {
	if cfg.OfflineMinYear > 0 && occurredAt.Year() < cfg.OfflineMinYear {
		return Violation{Reason: ReasonImplausible}
	}
	if !machineCreatedAt.IsZero() && occurredAt.Before(machineCreatedAt.UTC()) {
		return Violation{Reason: ReasonImplausible}
	}
	if receivedAt.Sub(occurredAt) < 0 {
		if -receivedAt.Sub(occurredAt) > cfg.OfflineFutureMax {
			return Violation{Reason: ReasonFuture}
		}
	}
	return nil
}

// ValidateConfigApply bounds config apply applied_at like a live check-in.
func ValidateConfigApply(appliedAt, receivedAt time.Time, cfg config.DeviceClockSkewConfig) error {
	return validateWindow(appliedAt, receivedAt, cfg.CheckInPastMax, cfg.CheckInFutureMax, 0, time.Time{})
}

// ValidateAdminInventory bounds operator-supplied inventory occurredAt.
func ValidateAdminInventory(occurredAt, receivedAt time.Time, cfg config.DeviceClockSkewConfig) error {
	return validateWindow(occurredAt, receivedAt, cfg.AdminInventoryPastMax, cfg.AdminInventoryFutureMax, 0, time.Time{})
}

func validateWindow(occurredAt, receivedAt time.Time, pastMax, futureMax time.Duration, minYear int, notBefore time.Time) error {
	occurredAt = occurredAt.UTC()
	receivedAt = receivedAt.UTC()
	if minYear > 0 && occurredAt.Year() < minYear {
		return Violation{Reason: ReasonImplausible}
	}
	if !notBefore.IsZero() && occurredAt.Before(notBefore.UTC()) {
		return Violation{Reason: ReasonImplausible}
	}
	if drift := receivedAt.Sub(occurredAt); drift > pastMax {
		return Violation{Reason: ReasonPast}
	}
	if drift := occurredAt.Sub(receivedAt); drift > futureMax {
		return Violation{Reason: ReasonFuture}
	}
	return nil
}

// DriftSeconds returns received_at minus occurred_at in seconds (positive when event is in the past).
func DriftSeconds(occurredAt, receivedAt time.Time) float64 {
	return receivedAt.Sub(occurredAt.UTC()).Seconds()
}

// FormatViolation returns a stable human-readable suffix for gRPC/HTTP errors.
func FormatViolation(v Violation) string {
	return fmt.Sprintf("device timestamp rejected: %s", v.Reason)
}
