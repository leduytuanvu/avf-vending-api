package reliability

import "testing"

func TestOutboxWillDeadLetterThisFailure(t *testing.T) {
	t.Parallel()
	cases := []struct {
		currentCount int32
		max          int
		want         bool
	}{
		{0, 3, false},
		{1, 3, false},
		{2, 3, true},
		{5, 10, false},
		{9, 10, true},
	}
	for _, tc := range cases {
		got := OutboxWillDeadLetterThisFailure(tc.currentCount, tc.max)
		if got != tc.want {
			t.Fatalf("OutboxWillDeadLetterThisFailure(count=%d,max=%d)=%v want %v", tc.currentCount, tc.max, got, tc.want)
		}
	}
}
