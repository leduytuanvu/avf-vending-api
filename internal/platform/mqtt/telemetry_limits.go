package mqtt

import (
	"encoding/json"
	"fmt"
)

// TelemetryIngressLimits optional bounds applied by Dispatch before invoking DeviceIngest.
type TelemetryIngressLimits struct {
	MaxPayloadBytes int
	MaxPoints       int
	MaxTags         int
}

func countJSONComplexity(v any) (points, tags int) {
	switch t := v.(type) {
	case map[string]any:
		for _, vv := range t {
			p, g := countJSONComplexity(vv)
			points += p
			tags += g
		}
	case []any:
		for _, vv := range t {
			p, g := countJSONComplexity(vv)
			points += p
			tags += g
		}
	case float64, json.Number:
		points++
	case bool, nil:
	case string:
		tags++
	default:
		points++
	}
	return points, tags
}

// ValidateTelemetryPayloadComplexity bounds nested JSON size for telemetry inner payload.
func ValidateTelemetryPayloadComplexity(data []byte, maxPoints, maxTags int) error {
	if maxPoints <= 0 || maxTags <= 0 {
		return nil
	}
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("telemetry payload json: %w", err)
	}
	p, g := countJSONComplexity(root)
	if p > maxPoints {
		return fmt.Errorf("telemetry payload exceeds max points (%d > %d)", p, maxPoints)
	}
	if g > maxTags {
		return fmt.Errorf("telemetry payload exceeds max tags (%d > %d)", g, maxTags)
	}
	return nil
}
