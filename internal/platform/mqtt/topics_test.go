package mqtt

import (
	"testing"

	"github.com/google/uuid"
)

func TestOutboundCommandDispatchTopic(t *testing.T) {
	mid := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	cases := []struct {
		prefix string
		want   string
	}{
		{"avf/devices", "avf/devices/11111111-1111-1111-1111-111111111111/commands/dispatch"},
		{"avf/devices/", "avf/devices/11111111-1111-1111-1111-111111111111/commands/dispatch"},
		{"  custom/prefix/  ", "custom/prefix/11111111-1111-1111-1111-111111111111/commands/dispatch"},
	}
	for _, tc := range cases {
		got := OutboundCommandDispatchTopic(tc.prefix, mid)
		if got != tc.want {
			t.Fatalf("prefix %q: got %q want %q", tc.prefix, got, tc.want)
		}
	}
}
