// Package revocation stores short-lived JWT/session revocation markers in Redis (optional).
// It is not a source of truth: Postgres owns accounts and refresh tokens; this layer only accelerates access-token rejection.
package revocation

import (
	"context"
	"time"
)

// Store records revoked JWT JTIs and whole-subject revocations (e.g. admin disabled account).
type Store interface {
	// RevokeJTI marks a single access-token JTI as unusable until TTL elapses.
	RevokeJTI(ctx context.Context, jti string, ttl time.Duration) error
	// IsJTIRevoked reports whether the JTI was revoked.
	IsJTIRevoked(ctx context.Context, jti string) (bool, error)

	// RevokeSubject marks all access tokens for this JWT subject (interactive account UUID string) for TTL.
	RevokeSubject(ctx context.Context, subject string, ttl time.Duration) error
	// IsSubjectRevoked reports subject-level revocation (stronger than per-JTI).
	IsSubjectRevoked(ctx context.Context, subject string) (bool, error)
}

// NoopStore implements Store with no persistence.
type NoopStore struct{}

// NewNoopStore returns a no-op store (development or revocation disabled).
func NewNoopStore() *NoopStore { return &NoopStore{} }

func (NoopStore) RevokeJTI(context.Context, string, time.Duration) error { return nil }
func (NoopStore) IsJTIRevoked(context.Context, string) (bool, error)     { return false, nil }
func (NoopStore) RevokeSubject(context.Context, string, time.Duration) error {
	return nil
}
func (NoopStore) IsSubjectRevoked(context.Context, string) (bool, error) { return false, nil }
