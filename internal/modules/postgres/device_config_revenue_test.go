package postgres_test

import (
	"os"
	"strings"
	"testing"
)

func TestRevenueQueriesStillFilterSimulatedOrders(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("../../../db/queries/reports.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "AND o.simulated = false") {
		t.Fatal("reports.sql must keep simulated=false revenue guard")
	}
}
