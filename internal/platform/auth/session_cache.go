package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// RefreshSessionCache is a non-authoritative cache for refresh-token session metadata.
// Implementations must key by token hash only; never store plaintext refresh tokens.
type RefreshSessionCache interface {
	PutRefreshSession(ctx context.Context, tokenHash []byte, accountID uuid.UUID, expiresAt time.Time) error
	InvalidateRefreshSession(ctx context.Context, tokenHash []byte) error
	InvalidateAccountSessions(ctx context.Context, accountID uuid.UUID) error
	// IsRefreshRevoked reports JWT revocation tombstones written during InvalidateRefreshSession /
	// InvalidateAccountSessions so callers can fast-reject without Postgres round trips when Redis indicates revocation.
	IsRefreshRevoked(ctx context.Context, tokenHash []byte) bool
}

// LoginFailureCounter is a fast Redis-backed account lockout signal.
// PostgreSQL remains the source of truth for audit and durable account state.
type LoginFailureCounter interface {
	IncrementFailure(ctx context.Context, organizationID uuid.UUID, email string, threshold int32, ttl time.Duration) (locked bool, count int64, err error)
	// PeekFailureCount returns the current failure count when the sliding counter is still active.
	PeekFailureCount(ctx context.Context, organizationID uuid.UUID, email string) (count int64, err error)
	ClearFailures(ctx context.Context, organizationID uuid.UUID, email string) error
}
