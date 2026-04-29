package mqttprom

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestAddCommandAttemptsExpiredMirrorsCanonicalExpiryMetric(t *testing.T) {
	t.Parallel()
	before := gatherSumCounter(t, "commands_expired_total")
	AddCommandAttemptsExpired(2)
	after := gatherSumCounter(t, "commands_expired_total")
	if after-before != 2 {
		t.Fatalf("commands_expired_total delta got=%g want=2 (before=%g after=%g)", after-before, before, after)
	}
}

func gatherSumCounter(t *testing.T, family string) float64 {
	t.Helper()
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, mf := range mfs {
		if mf.GetName() != family {
			continue
		}
		var sum float64
		for _, m := range mf.Metric {
			if m.Counter != nil {
				sum += m.Counter.GetValue()
			}
		}
		return sum
	}
	var names strings.Builder
	for _, mf := range mfs {
		names.WriteString(mf.GetName())
		names.WriteByte(',')
	}
	t.Fatalf("family %q not found (have: %s)", family, names.String())
	return 0
}
