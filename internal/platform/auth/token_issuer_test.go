package auth

import (
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSessionIssuer_IssueAndValidate(t *testing.T) {
	org := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	acct := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	iss, err := NewSessionIssuerFromHTTPAuth(config.HTTPAuthConfig{
		JWTSecret:        []byte("unit-test-secret"),
		JWTLeeway:        30 * time.Second,
		ExpectedIssuer:   "test-iss",
		ExpectedAudience: "test-aud",
		AccessTokenTTL:   time.Minute,
		RefreshTokenTTL:  time.Hour,
	})
	require.NoError(t, err)

	tok, exp, err := iss.IssueAccessJWT(acct, org, []string{RoleOrgAdmin})
	require.NoError(t, err)
	require.False(t, exp.IsZero())

	v := newHS256Validator([]byte("unit-test-secret"), nil, 30*time.Second)
	p, err := v.ValidateAccessToken(t.Context(), tok)
	require.NoError(t, err)
	require.Equal(t, acct.String(), p.Subject)
	require.Equal(t, org, p.OrganizationID)
	require.True(t, p.HasRole(RoleOrgAdmin))
}

func TestNewRefreshToken_RoundTripHash(t *testing.T) {
	raw, h, err := NewRefreshToken()
	require.NoError(t, err)
	require.NotEmpty(t, raw)
	require.Len(t, h, 32)
	require.Equal(t, h, HashRefreshToken(raw))
}
