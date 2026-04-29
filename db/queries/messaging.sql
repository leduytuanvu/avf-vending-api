-- name: InsertMessagingConsumerDedupe :one
INSERT INTO messaging_consumer_dedupe (
    consumer_name,
    broker_subject,
    broker_msg_id
)
VALUES ($1, $2, $3)
RETURNING
    id,
    consumer_name,
    broker_subject,
    broker_msg_id,
    processed_at;
