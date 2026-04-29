package redis

import (
	"testing"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestNewClient_emptyAddrReturnsNil(t *testing.T) {
	t.Parallel()
	c, err := NewClient(&config.RedisConfig{})
	require.NoError(t, err)
	require.Nil(t, c)
}
