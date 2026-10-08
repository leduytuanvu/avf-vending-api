package machineruntime

import (
	"testing"

	"github.com/google/uuid"
)

func TestDeviceEventIDFromCashMovementIdempotencyKey(t *testing.T) {
	t.Parallel()
	mid := uuid.MustParse("01a0a7e5-3c68-7895-b526-bcb6504bccfb")
	cases := []struct {
		key string
		want string
	}{
		{
			key:  "cash_movement:01a0a7e5-3c68-7895-b526-bcb6504bccfb:010005",
			want: "010005",
		},
		{
			key:  "cash_movement:v2:01a0a7e5-3c68-7895-b526-bcb6504bccfb:010005",
			want: "010005",
		},
		{
			key:  "cash_movement:01a0a7e5-3c68-7895-b526-bcb6504bccfb:boot-1:010005",
			want: "010005",
		},
		{
			key:  "cash_movement:01a0a7e5-3c68-7895-b526-bcb6504bccfb:payout:wd-1:1:NOTE_CONFIRMED",
			want: "payout:wd-1:1:NOTE_CONFIRMED",
		},
		{key: "offline-sale:order", want: ""},
	}
	for _, tc := range cases {
		got := DeviceEventIDFromCashMovementIdempotencyKey(mid, tc.key)
		if got != tc.want {
			t.Fatalf("key %q: got %q want %q", tc.key, got, tc.want)
		}
	}
}
