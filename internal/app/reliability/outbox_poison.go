package reliability

// Outbox poison-message policy (worker / PlanOutboxRepublishBatch + OutboxDispatchTick):
//
// 1) Rows are only eligible for transport publish when DecideOutboxReplay returns ShouldRepublish
//    (respects outbox_min_age, next_publish_after backoff, and never republishes dead-lettered rows).
//
// 2) Each failed JetStream publish increments publish_attempt_count and schedules next_publish_after
//    using exponential backoff (OutboxPublishBackoffAfterFailure).
//
// 3) When the publish attempt count *after* recording this failure would reach OutboxMaxPublishAttempts,
//    RecordOutboxPublishFailure sets dead_lettered_at (Postgres quarantine). The row leaves the
//    unpublished listing permanently; operators must repair or replay from DLQ / audit tables.
//
// 4) Optional: after quarantine, the worker may publish one copy to AVF_INTERNAL_DLQ (see
//    platform/nats.PublishOutboxWorkerDeadLetter) for alerting and manual replay tooling.
//
// JetStream dedupe on the live outbox subject uses Nats-Msg-Id from the outbox row; that strategy
// must not be altered for successful publishes. DLQ publishes use a separate Nats-Msg-Id prefix.

// OutboxWillDeadLetterThisFailure reports whether the failure we are about to record should quarantine
// the row (no further publish attempts). attemptCountBeforeIncrement is the current publish_attempt_count
// on the row; after RecordOutboxPublishFailure the count will be attemptCountBeforeIncrement+1.
func OutboxWillDeadLetterThisFailure(attemptCountBeforeIncrement int32, maxPublishAttempts int) bool {
	next := int(attemptCountBeforeIncrement) + 1
	return next >= maxPublishAttempts
}
