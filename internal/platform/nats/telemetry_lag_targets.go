package nats

// TelemetryDurablePair identifies a JetStream durable consumer used for telemetry projection lag metrics.
type TelemetryDurablePair struct {
	Stream  string
	Durable string
}

// TelemetryDurablePairs lists all telemetry projection consumers (keep in sync with EnsureTelemetryDurableConsumers).
func TelemetryDurablePairs() []TelemetryDurablePair {
	return []TelemetryDurablePair{
		{Stream: StreamTelemetryHeartbeat, Durable: "avf-w-telemetry-heartbeat"},
		{Stream: StreamTelemetryState, Durable: "avf-w-telemetry-state"},
		{Stream: StreamTelemetryMetrics, Durable: "avf-w-telemetry-metrics"},
		{Stream: StreamTelemetryIncidents, Durable: "avf-w-telemetry-incidents"},
		{Stream: StreamTelemetryCommandReceipts, Durable: "avf-w-telemetry-command-receipts"},
		{Stream: StreamTelemetryDiagnosticBundleReady, Durable: "avf-w-telemetry-diagnostic"},
	}
}
