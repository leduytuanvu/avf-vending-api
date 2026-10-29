package commerce

import "testing"

func TestQualifyCashMovementDeviceEventID(t *testing.T) {
	t.Parallel()
	cases := []struct {
		id, boot, want string
	}{
		{id: "560204", boot: "boot-1", want: "boot-1:560204"},
		{id: "560204:9", boot: "boot-1", want: "boot-1:560204:9"},
		{id: "boot-1:560204", boot: "boot-1", want: "boot-1:560204"},
		{id: "payout:wd:1:NOTE_CONFIRMED", boot: "boot-1", want: "payout:wd:1:NOTE_CONFIRMED"},
		{id: "bill_reject:ep:5CFF0A", boot: "boot-1", want: "bill_reject:ep:5CFF0A"},
		{id: "560204", boot: "", want: "560204"},
		{id: "", boot: "boot-1", want: ""},
		{
			id:   "560204",
			boot: "8a9e2cfc-b468-49d7-81fc-0c9d3f806aec",
			want: "8a9e2cfc-b468-49d7-81fc-0c9d3f806aec:560204",
		},
	}
	for _, tc := range cases {
		got := QualifyCashMovementDeviceEventID(tc.id, tc.boot)
		if got != tc.want {
			t.Fatalf("Qualify(%q, %q)=%q want %q", tc.id, tc.boot, got, tc.want)
		}
	}
	same := QualifyCashMovementDeviceEventID("390106:1", "boot-1")
	replay := QualifyCashMovementDeviceEventID("390106:1", "boot-1")
	if same != replay {
		t.Fatalf("replay identity drifted: %q vs %q", same, replay)
	}
	otherBoot := QualifyCashMovementDeviceEventID("560204", "boot-2")
	if QualifyCashMovementDeviceEventID("560204", "boot-1") == otherBoot {
		t.Fatal("different boots must not share identity")
	}
}
