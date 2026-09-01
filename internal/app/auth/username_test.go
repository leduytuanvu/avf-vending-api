package auth_test

import (
	"testing"

	appauth "github.com/avf/avf-vending-api/internal/app/auth"
	"github.com/stretchr/testify/require"
)

func TestNormalizeUsernameForBootstrap(t *testing.T) {
	u, err := appauth.NormalizeUsernameForBootstrap("Admin")
	require.NoError(t, err)
	require.Equal(t, "admin", u)

	_, err = appauth.NormalizeUsernameForBootstrap("ab")
	require.ErrorIs(t, err, appauth.ErrInvalidUsername)

	_, err = appauth.NormalizeUsernameForBootstrap("root")
	require.ErrorIs(t, err, appauth.ErrInvalidUsername)
}
