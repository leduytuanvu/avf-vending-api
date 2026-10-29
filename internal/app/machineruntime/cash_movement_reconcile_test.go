package machineruntime

import (
	"testing"

	"github.com/google/uuid"
)

func TestDeviceEventIDFromCashMovementIdempotencyKey(t *testing.T) {
	t.Parallel()
	mid := uuid.MustParse("01a0a7e5-3c68-7895-b526-bcb6504bccfb")
	boot := "8a9e2cfc-b468-49d7-81fc-0c9d3f806aec"
	cases := []struct {
		key  string
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
			want: "boot-1:010005",
		},
		{
			key:  "cash_movement:01a0a7e5-3c68-7895-b526-bcb6504bccfb:boot-1:BC0103:bill_credit",
			want: "boot-1:BC0103",
		},
		{
			key:  "cash_movement:v2:01a0a7e5-3c68-7895-b526-bcb6504bccfb:BC0103:bill_lifecycle:escrow_held",
			want: "BC0103",
		},
		{
			key:  "cash_movement:v3:01a0a7e5-3c68-7895-b526-bcb6504bccfb:BC0103:bill_credit:12345",
			want: "BC0103",
		},
		{
			key:  "cash_movement:01a0a7e5-3c68-7895-b526-bcb6504bccfb:boot-1:560204:9:bill_credit",
			want: "boot-1:560204:9",
		},
		{
			key:  "cash_movement:01a0a7e5-3c68-7895-b526-bcb6504bccfb:" + boot + ":560204:bill_credit",
			want: boot + ":560204",
		},
		{
			key:  "cash_movement:01a0a7e5-3c68-7895-b526-bcb6504bccfb:payout:wd-1:1:NOTE_CONFIRMED",
			want: "payout:wd-1:1:NOTE_CONFIRMED",
		},
		{
			key:  "cash_movement:01a0a7e5-3c68-7895-b526-bcb6504bccfb:boot-1:payout:wd-1:1:NOTE_CONFIRMED:payout",
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

func TestBootIDFromCashMovementIdempotencyKey(t *testing.T) {
	t.Parallel()
	mid := uuid.MustParse("01a0a7e5-3c68-7895-b526-bcb6504bccfb")
	boot := "8a9e2cfc-b468-49d7-81fc-0c9d3f806aec"
	got := BootIDFromCashMovementIdempotencyKey(mid, "cash_movement:"+mid.String()+":boot-1:560204:bill_credit")
	if got != "boot-1" {
		t.Fatalf("got %q", got)
	}
	got = BootIDFromCashMovementIdempotencyKey(mid, "cash_movement:"+mid.String()+":"+boot+":560204:bill_credit")
	if got != boot {
		t.Fatalf("got %q want %q", got, boot)
	}
	got = BootIDFromCashMovementIdempotencyKey(mid, "cash_movement:"+mid.String()+":010005")
	if got != "" {
		t.Fatalf("hex-only key should have empty boot, got %q", got)
	}
}
