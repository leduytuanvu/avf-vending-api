package telemetry

import (
	"os"
	"strconv"
	"strings"
)

const (
	// DefaultMaxIngestBytes caps a single MQTT JSON payload accepted by ingest (before NATS publish).
	DefaultMaxIngestBytes = 65536
)

// MaxIngestPayloadBytes returns TELEMETRY_MAX_INGEST_BYTES or default.
func MaxIngestPayloadBytes() int {
	raw := strings.TrimSpace(os.Getenv("TELEMETRY_MAX_INGEST_BYTES"))
	if raw == "" {
		return DefaultMaxIngestBytes
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return DefaultMaxIngestBytes
	}
	const hardCap = 512 * 1024
	if n > hardCap {
		return hardCap
	}
	return n
}
