package payments

import "testing"

func TestSameWebhookAdapterFamily(t *testing.T) {
	t.Parallel()
	cases := []struct {
		event, stored string
		want          bool
	}{
		{"zalopay", "vietqr", true},
		{"vietqr", "zalopay", true},
		{"zalopay", "zalopay", true},
		{"momo", "momo", true},
		{"zalopay", "momo", false},
		{"momo", "vietqr", false},
		{"", "vietqr", false},
		{"zalopay", "", false},
	}
	for _, tc := range cases {
		if got := SameWebhookAdapterFamily(tc.event, tc.stored); got != tc.want {
			t.Fatalf("SameWebhookAdapterFamily(%q,%q)=%v want %v", tc.event, tc.stored, got, tc.want)
		}
	}
}
