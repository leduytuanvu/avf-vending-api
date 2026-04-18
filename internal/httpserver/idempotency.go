package httpserver

import (
	"fmt"
	"net/http"
	"strings"
)

const (
	headerIdempotencyKey    = "Idempotency-Key"
	headerIdempotencyKeyAlt = "X-Idempotency-Key"
)

func requireWriteIdempotencyKey(r *http.Request) (string, error) {
	raw := strings.TrimSpace(r.Header.Get(headerIdempotencyKey))
	if raw == "" {
		raw = strings.TrimSpace(r.Header.Get(headerIdempotencyKeyAlt))
	}
	if raw == "" {
		return "", fmt.Errorf("missing idempotency key header (%s or %s)", headerIdempotencyKey, headerIdempotencyKeyAlt)
	}
	return raw, nil
}
