package postgres

import (
	"context"
	"errors"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MessagingConsumerDeduper records first-seen NATS/JetStream message ids so at-least-once consumers can skip duplicates.
type MessagingConsumerDeduper struct {
	pool *pgxpool.Pool
}

func NewMessagingConsumerDeduper(pool *pgxpool.Pool) *MessagingConsumerDeduper {
	return &MessagingConsumerDeduper{pool: pool}
}

// TryClaim inserts a dedupe row; returns (true, nil) on first claim, (false, nil) on duplicate.
func (m *MessagingConsumerDeduper) TryClaim(ctx context.Context, consumerName, brokerSubject, brokerMsgID string) (firstClaim bool, err error) {
	if m == nil || m.pool == nil {
		return false, errors.New("postgres: nil messaging deduper")
	}
	if consumerName == "" || brokerSubject == "" || brokerMsgID == "" {
		return false, errors.New("postgres: consumer_name, broker_subject, broker_msg_id are required")
	}
	_, insErr := db.New(m.pool).InsertMessagingConsumerDedupe(ctx, db.InsertMessagingConsumerDedupeParams{
		ConsumerName:  consumerName,
		BrokerSubject: brokerSubject,
		BrokerMsgID:   brokerMsgID,
	})
	if insErr == nil {
		return true, nil
	}
	var pgErr *pgconn.PgError
	if errors.As(insErr, &pgErr) && pgErr.Code == "23505" {
		return false, nil
	}
	return false, insErr
}
