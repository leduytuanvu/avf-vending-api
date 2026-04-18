package nats

import (
	"strings"
	"unicode"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
)

const (
	// StreamOutbox is the JetStream stream holding internal outbox traffic.
	StreamOutbox = "AVF_INTERNAL_OUTBOX"
	// StreamDLQ holds dead-letter copies for operators and replay tooling.
	StreamDLQ = "AVF_INTERNAL_DLQ"

	// SubjectPrefixOutbox is prepended to each logical topic segment.
	SubjectPrefixOutbox = "avf.internal.outbox."
	// SubjectPrefixDLQ is prepended to a short failure reason token.
	SubjectPrefixDLQ = "avf.internal.dlq."

	// SubjectPatternOutbox is the stream wildcard for outbox subjects.
	SubjectPatternOutbox = "avf.internal.outbox.>"
	// SubjectPatternDLQ is the stream wildcard for DLQ subjects.
	SubjectPatternDLQ = "avf.internal.dlq.>"
)

// LogicalTopic returns the routing suffix for an outbox row (topic preferred, else event type).
func LogicalTopic(ev domaincommerce.OutboxEvent) string {
	t := strings.TrimSpace(ev.Topic)
	if t != "" {
		return t
	}
	t = strings.TrimSpace(ev.EventType)
	if t != "" {
		return t
	}
	return "unknown"
}

// OutboxPublishSubject is the JetStream subject used for a single outbox event.
func OutboxPublishSubject(ev domaincommerce.OutboxEvent) string {
	return SubjectPrefixOutbox + SanitizeTopicPath(LogicalTopic(ev))
}

// DLQSubject builds a DLQ subject for a failure reason (one segment).
func DLQSubject(reason string) string {
	return SubjectPrefixDLQ + SanitizeTopicPath(reason)
}

// SanitizeTopicPath maps a dotted logical name into NATS-safe tokens (no spaces, wildcards, or empty segments).
func SanitizeTopicPath(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return "unknown"
	}
	var b strings.Builder
	prevDot := false
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			prevDot = false
		case r == '.' || r == '/' || r == ':' || r == '_' || r == '-':
			if !prevDot {
				b.WriteByte('.')
				prevDot = true
			}
		default:
			if !prevDot {
				b.WriteByte('_')
				prevDot = true
			}
		}
	}
	out := strings.Trim(b.String(), ".")
	if out == "" {
		return "unknown"
	}
	return out
}
