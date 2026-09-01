package httpserver

import (
	"net/url"
	"testing"

	platformtimezone "github.com/avf/avf-vending-api/internal/platform/timezone"
	"github.com/stretchr/testify/require"
)

func TestParseReportingTimezone_defaultsVietnam(t *testing.T) {
	q := url.Values{}
	require.Equal(t, platformtimezone.VietnamBusiness, parseReportingTimezone(q))
	q.Set("timezone", "UTC")
	require.Equal(t, "UTC", parseReportingTimezone(q))
}
