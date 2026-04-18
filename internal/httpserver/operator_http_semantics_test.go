package httpserver

import (
	"errors"
	"net/http"
	"testing"

	domainoperator "github.com/avf/avf-vending-api/internal/domain/operator"
)

func TestParseOperatorLogoutFinalStatus(t *testing.T) {
	cases := []struct {
		raw  string
		want string
		err  bool
	}{
		{"", domainoperator.SessionStatusEnded, false},
		{"  ended  ", domainoperator.SessionStatusEnded, false},
		{"ENDED", domainoperator.SessionStatusEnded, false},
		{"revoked", domainoperator.SessionStatusRevoked, false},
		{"REVOKED", domainoperator.SessionStatusRevoked, false},
		{"ACTIVE", "", true},
	}
	for _, tc := range cases {
		got, err := parseOperatorLogoutFinalStatus(tc.raw)
		if tc.err {
			if !errors.Is(err, domainoperator.ErrInvalidSessionEndStatus) {
				t.Fatalf("%q: want ErrInvalidSessionEndStatus, got %v", tc.raw, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%q: %v", tc.raw, err)
		}
		if got != tc.want {
			t.Fatalf("%q: got %q want %q", tc.raw, got, tc.want)
		}
	}
}

func TestParseOperatorListLimit(t *testing.T) {
	cases := []struct {
		query string
		want  int32
		err   bool
	}{
		{"", operatorListLimitDefault, false},
		{"limit=", operatorListLimitDefault, false},
		{"limit=10", 10, false},
		{"limit=501", operatorListLimitMax, false},
		{"limit=500", 500, false},
		{"limit=0", 0, true},
		{"limit=-1", 0, true},
		{"limit=abc", 0, true},
	}
	for _, tc := range cases {
		req, err := http.NewRequest(http.MethodGet, "/?"+tc.query, nil)
		if err != nil {
			t.Fatal(err)
		}
		got, lerr := parseOperatorListLimit(req)
		if tc.err {
			if lerr == nil {
				t.Fatalf("query %q: want error", tc.query)
			}
			continue
		}
		if lerr != nil {
			t.Fatalf("query %q: %v", tc.query, lerr)
		}
		if got != tc.want {
			t.Fatalf("query %q: got %d want %d", tc.query, got, tc.want)
		}
	}
}
