// Package nats provides JetStream connection helpers, internal stream/subject layout, outbox
// publishing, DLQ helpers, and pull-consumer utilities for internal messaging.
//
// Status: production outbox foundation.
//   - Live in this repo: cmd/worker registers an outbox publisher when NATS_URL is set; streams are
//     ensured at startup. Deployed profiles set NATS_REQUIRED/OUTBOX_PUBLISHER_REQUIRED so startup
//     fails instead of silently running DB-only outbox dispatch.
//   - Not live in this repo: no cmd/* process runs a JetStream consumer loop against these subjects
//     yet—consumer helpers exist for external services or future wiring.
//
// Subject layout (single tenant segment, then logical topic):
//
//	avf.internal.outbox.<logical_topic>   — durable outbox fan-out (stream AVF_INTERNAL_OUTBOX)
//	avf.internal.dlq.<reason_slug>        — poison / manual replay sink (stream AVF_INTERNAL_DLQ)
//
// Logical topics come from the outbox row (topic or event_type); segments are sanitized for NATS.
//
// Correlation travels in message headers (X-Correlation-Id, X-Outbox-Id, aggregate ids) so consumers
// stay decoupled from envelope parsing. JetStream Nats-Msg-Id is set from idempotency_key when present.
//
// Retry / DLQ shape: publishers do not delete DB rows; consumers use explicit ack, bounded redelivery
// (see ConsumerRetryDefaults), and may forward to PublishDLQ on terminal failure.
//
// Environment: set NATS_URL (e.g. nats://127.0.0.1:4222) for the worker outbox publisher. Local
// development may omit it only when NATS_REQUIRED=false and OUTBOX_PUBLISHER_REQUIRED=false.
package nats

// Well-known environment keys (worker reads NATS_URL directly to stay within cmd/worker scope).
const EnvNATSURL = "NATS_URL"
