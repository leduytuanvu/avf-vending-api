package outbox

import "errors"

// ErrPoisonReplayRequiresConfirm is returned when an operator attempts to reset a dead-letter
// outbox row without passing an explicit confirmation flag (CLI/API guardrail).
var ErrPoisonReplayRequiresConfirm = errors.New("outbox: replay dead-letter requires explicit operator confirmation")

// RequirePoisonReplayConfirmation enforces a deliberate flag for DLQ replay so poison messages
// are not retried accidentally.
func RequirePoisonReplayConfirmation(confirmed bool) error {
	if !confirmed {
		return ErrPoisonReplayRequiresConfirm
	}
	return nil
}
