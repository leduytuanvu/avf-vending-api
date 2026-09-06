package postgres

import (
	"testing"
)

func TestCabinetCodesToTry(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{in: "", want: []string{"CAB-A"}},
		{in: "A", want: []string{"A", "CAB-A"}},
		{in: "MAIN", want: []string{"MAIN", "CAB-A"}},
		{in: "CAB-A", want: []string{"CAB-A"}},
	}
	for _, tc := range cases {
		got := cabinetCodesToTry(tc.in)
		if len(got) != len(tc.want) {
			t.Fatalf("cabinetCodesToTry(%q)=%v want %v", tc.in, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("cabinetCodesToTry(%q)=%v want %v", tc.in, got, tc.want)
			}
		}
	}
}

func TestLayoutKeysToTry(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{in: "", want: []string{"default", "grid-10x6", "grid-4x6"}},
		{in: "default", want: []string{"default", "grid-10x6", "grid-4x6"}},
		{in: "grid-10x6", want: []string{"grid-10x6"}},
	}
	for _, tc := range cases {
		got := layoutKeysToTry(tc.in)
		if len(got) != len(tc.want) {
			t.Fatalf("layoutKeysToTry(%q)=%v want %v", tc.in, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("layoutKeysToTry(%q)=%v want %v", tc.in, got, tc.want)
			}
		}
	}
}
