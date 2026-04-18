package background

import "testing"

func TestClassifyProviderNormalizedState(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in     string
		want   string
		wantOK bool
	}{
		{"CAPTURED", "captured", true},
		{" settled ", "captured", true},
		{"paid", "captured", true},
		{"Declined", "failed", true},
		{"cancelled", "failed", true},
		{"pending", "", false},
		{"", "", false},
		{"processing", "", false},
	}
	for _, tc := range cases {
		got, ok := classifyProviderNormalizedState(tc.in)
		if ok != tc.wantOK || got != tc.want {
			t.Fatalf("classify(%q) = (%q,%v) want (%q,%v)", tc.in, got, ok, tc.want, tc.wantOK)
		}
	}
}
