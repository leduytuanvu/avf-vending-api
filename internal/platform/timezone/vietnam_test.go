package timezone_test

import (
	"testing"

	platformtimezone "github.com/avf/avf-vending-api/internal/platform/timezone"
	"github.com/stretchr/testify/require"
)

func TestAssertRequired_loadsVietnamAndUTC(t *testing.T) {
	require.NoError(t, platformtimezone.AssertRequired())
	require.Equal(t, "Asia/Ho_Chi_Minh", platformtimezone.VietnamBusiness)
}
