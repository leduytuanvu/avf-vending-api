package postgres

import "testing"

func TestPaymentTransitionAllowed(t *testing.T) {
	cases := []struct {
		from, to string
		want     bool
	}{
		{"created", "authorized", true},
		{"created", "captured", true},
		{"created", "failed", true},
		{"authorized", "captured", true},
		{"authorized", "failed", true},
		{"captured", "captured", true},
		{"created", "created", true},
		{"captured", "authorized", false},
		{"failed", "captured", false},
		{"refunded", "captured", false},
	}
	for _, tc := range cases {
		if got := paymentTransitionAllowed(tc.from, tc.to); got != tc.want {
			t.Fatalf("%s -> %s: got %v want %v", tc.from, tc.to, got, tc.want)
		}
	}
}
