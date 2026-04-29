package reliability

import domainreliability "github.com/avf/avf-vending-api/internal/domain/reliability"

// EffectiveOutboxMaxAttempts returns per-row ceiling when positive; otherwise recovery policy max.
func EffectiveOutboxMaxAttempts(ev domainreliability.OutboxEvent, policyMax int) int {
	if ev.MaxPublishAttempts > 0 {
		return int(ev.MaxPublishAttempts)
	}
	return policyMax
}
