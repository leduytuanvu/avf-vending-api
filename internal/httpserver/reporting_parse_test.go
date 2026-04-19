package httpserver

import (
	"net/url"
	"testing"
	"time"
)

func TestParseRequiredRFC3339Range(t *testing.T) {
	q := url.Values{}
	if _, _, err := parseRequiredRFC3339Range(q); err == nil {
		t.Fatal("expected error when from/to missing")
	}
	q.Set("from", "2026-01-01T00:00:00Z")
	q.Set("to", "2026-01-02T00:00:00Z")
	from, to, err := parseRequiredRFC3339Range(q)
	if err != nil {
		t.Fatal(err)
	}
	if !from.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) || !to.Equal(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected range: %v %v", from, to)
	}
}
