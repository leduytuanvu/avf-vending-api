package reliability

import "time"

// OutboxPublishBackoffAfterFailure returns the wall-clock delay before the next dispatch attempt
// should be scheduled, given publish_attempt_count *after* this failure is recorded.
//
// Schedule is deterministic: base, 2*base, 4*base, ... capped at max (exponential backoff).
func OutboxPublishBackoffAfterFailure(attemptAfterThisFailure int32, base, max time.Duration) time.Duration {
	if base <= 0 {
		return 0
	}
	if max < base {
		max = base
	}
	if attemptAfterThisFailure < 1 {
		attemptAfterThisFailure = 1
	}
	d := base
	for i := int32(1); i < attemptAfterThisFailure; i++ {
		next := d * 2
		if next >= max {
			return max
		}
		d = next
	}
	if d > max {
		return max
	}
	return d
}
