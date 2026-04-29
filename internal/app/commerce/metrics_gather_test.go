package commerce

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestCommerceMetricsGatherDoesNotPanic(t *testing.T) {
	t.Parallel()
	recordVendFinalized("success")
	recordVendFinalized("")
	_, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
}
