package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type stubRevocationStore struct {
	jtiRevoked map[string]bool
	subRevoked map[string]bool
	jtiErr     error
	subErr     error
}

func (s *stubRevocationStore) RevokeJTI(context.Context, string, time.Duration) error { return nil }
func (s *stubRevocationStore) IsJTIRevoked(_ context.Context, jti string) (bool, error) {
	if s.jtiErr != nil {
		return false, s.jtiErr
	}
	return s.jtiRevoked[jti], nil
}
func (s *stubRevocationStore) RevokeSubject(context.Context, string, time.Duration) error { return nil }
func (s *stubRevocationStore) IsSubjectRevoked(_ context.Context, sub string) (bool, error) {
	if s.subErr != nil {
		return false, s.subErr
	}
	return s.subRevoked[sub], nil
}

type fixedValidator struct {
	p   Principal
	err error
}

func (f fixedValidator) ValidateAccessToken(_ context.Context, _ string) (Principal, error) {
	return f.p, f.err
}

func TestWrapWithRevocation_revokedJTIFailsClosed(t *testing.T) {
	t.Parallel()
	inner := fixedValidator{p: Principal{Subject: "acc-1", JTI: "jti-a"}}
	st := &stubRevocationStore{jtiRevoked: map[string]bool{"jti-a": true}}
	v := WrapWithRevocation(inner, st, false)
	_, err := v.ValidateAccessToken(context.Background(), "x")
	require.ErrorIs(t, err, ErrUnauthenticated)
}

func TestWrapWithRevocation_redisErrFailClosed(t *testing.T) {
	t.Parallel()
	inner := fixedValidator{p: Principal{Subject: "acc-1", JTI: "jti-b"}}
	st := &stubRevocationStore{jtiErr: errors.New("redis down")}
	v := WrapWithRevocation(inner, st, false)
	_, err := v.ValidateAccessToken(context.Background(), "x")
	require.ErrorIs(t, err, ErrUnauthenticated)
}

func TestWrapWithRevocation_redisErrFailOpen(t *testing.T) {
	t.Parallel()
	inner := fixedValidator{p: Principal{Subject: "acc-1", JTI: "jti-c"}}
	st := &stubRevocationStore{jtiErr: errors.New("redis down")}
	v := WrapWithRevocation(inner, st, true)
	p, err := v.ValidateAccessToken(context.Background(), "x")
	require.NoError(t, err)
	require.Equal(t, "jti-c", p.JTI)
}
