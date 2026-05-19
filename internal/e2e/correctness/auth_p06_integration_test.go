package correctness

import (
	"bytes"
	"context"
	"github.com/avf/avf-vending-api/internal/platform/id"
	"testing"
	"time"

	appauth "github.com/avf/avf-vending-api/internal/app/auth"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/gen/db"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestP06_E2E_Auth_DisabledUserCannotLogin(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	queries := db.New(pool)
	issuer, err := plauth.NewSessionIssuerFromHTTPAuth(config.HTTPAuthConfig{
		JWTSecret:        bytes.Repeat([]byte("p"), 32),
		JWTLeeway:        30 * time.Second,
		ExpectedIssuer:   "p06-iss",
		ExpectedAudience: "p06-aud",
		AccessTokenTTL:   time.Minute,
		RefreshTokenTTL:  time.Hour,
	})
	require.NoError(t, err)
	svc, err := appauth.NewService(appauth.Deps{Queries: queries, Issuer: issuer, Pool: pool})
	require.NoError(t, err)

	id := id.NewUUIDV7()
	email := "p06-disabled-" + id.String()[:8] + "@test.example.com"
	hash, err := bcrypt.GenerateFromPassword([]byte("password12345"), bcrypt.MinCost)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO platform_auth_accounts (id, email, password_hash, roles, status)
VALUES ($1,$2,$3,$4,$5)`, id, email, string(hash), []string{"viewer"}, "disabled")
	require.NoError(t, err)

	_, err = svc.Login(ctx, appauth.LoginRequest{Email: email, Password: "password12345"})
	require.ErrorIs(t, err, appauth.ErrInvalidCredentials)
}
