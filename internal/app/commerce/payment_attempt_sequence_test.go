package commerce

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestValidateStartPayment_rejectsNegativeAttemptSeq(t *testing.T) {
	t.Parallel()
	err := validateStartPayment(StartPaymentInput{
		OrderID:              uuid.New(),
		Provider:             "cash",
		PaymentState:         "captured",
		AmountMinor:          100,
		Currency:             "VND",
		IdempotencyKey:       "idem",
		OutboxTopic:          "payments",
		OutboxEventType:      "payment.captured",
		OutboxAggregateType:  "order",
		OutboxAggregateID:    uuid.New(),
		OutboxIdempotencyKey: "idem:outbox",
		AttemptSeq:           -1,
	})
	require.Error(t, err)
}

func TestValidateStartPayment_allowsOmittedAttemptSeq(t *testing.T) {
	t.Parallel()
	err := validateStartPayment(StartPaymentInput{
		OrderID:              uuid.New(),
		Provider:             "cash",
		PaymentState:         "captured",
		AmountMinor:          100,
		Currency:             "VND",
		IdempotencyKey:       "idem",
		OutboxTopic:          "payments",
		OutboxEventType:      "payment.captured",
		OutboxAggregateType:  "order",
		OutboxAggregateID:    uuid.New(),
		OutboxIdempotencyKey: "idem:outbox",
		AttemptSeq:           0,
	})
	require.NoError(t, err)
}
