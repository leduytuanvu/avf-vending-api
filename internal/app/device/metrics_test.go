package device

import (
	"testing"

	"github.com/avf/avf-vending-api/internal/platform/observability/productionmetrics"
	"github.com/prometheus/client_golang/prometheus"
)

func TestDevicePackageLinkedMetricsGather(t *testing.T) {
	t.Parallel()
	productionmetrics.RecordMachineCheckIn("device_pkg_test")
	if _, err := prometheus.DefaultGatherer.Gather(); err != nil {
		t.Fatal(err)
	}
}
