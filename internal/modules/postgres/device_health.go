package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

var deviceHealthAllowlist = map[string]struct{}{
	"cpu_pct":            {},
	"gpu_pct":            {},
	"soc_thermal_status": {},
	"battery_c":          {},
	"storage_free_mb":    {},
	"storage_used_pct":   {},
	"heap_used_kb":       {},
	"heap_max_kb":        {},
	"ui_jank_skipped":    {},
	"net_online":         {},
	"net_tier":           {},
	"net_dbm":            {},
	"net_flap_count":     {},
	"net_quality":        {},
	"device_condition":   {},
	"sampled_at_ms":      {},
}

var deviceHealthNumericKeys = map[string]struct{}{
	"cpu_pct":          {},
	"gpu_pct":          {},
	"battery_c":        {},
	"storage_free_mb":  {},
	"storage_used_pct": {},
	"heap_used_kb":     {},
	"heap_max_kb":      {},
	"ui_jank_skipped":  {},
	"net_dbm":          {},
	"net_flap_count":   {},
	"sampled_at_ms":    {},
}

// ParseDeviceHealthCustomFields extracts allowlisted keys from HealthSnapshotPayload.customFields.
// Unknown keys are dropped. Empty input yields {}.
func ParseDeviceHealthCustomFields(payload []byte) []byte {
	out := map[string]any{}
	if len(payload) == 0 {
		return []byte("{}")
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(payload, &root); err != nil {
		return []byte("{}")
	}
	custom := root["customFields"]
	if len(custom) == 0 {
		custom = root["custom_fields"]
	}
	if len(custom) == 0 {
		return []byte("{}")
	}
	var fields map[string]any
	if err := json.Unmarshal(custom, &fields); err != nil {
		return []byte("{}")
	}
	for k, v := range fields {
		key := strings.TrimSpace(k)
		if _, ok := deviceHealthAllowlist[key]; !ok {
			continue
		}
		if coerced, ok := coerceDeviceHealthValue(key, v); ok {
			out[key] = coerced
		}
	}
	b, err := json.Marshal(out)
	if err != nil {
		return []byte("{}")
	}
	return b
}

// FilterDeviceHealthMetrics keeps allowlisted keys from a flat metrics_state object.
func FilterDeviceHealthMetrics(metrics []byte) []byte {
	if len(metrics) == 0 {
		return []byte("{}")
	}
	var fields map[string]any
	if err := json.Unmarshal(metrics, &fields); err != nil {
		return []byte("{}")
	}
	out := map[string]any{}
	for k, v := range fields {
		key := strings.TrimSpace(k)
		if _, ok := deviceHealthAllowlist[key]; !ok {
			continue
		}
		if coerced, ok := coerceDeviceHealthValue(key, v); ok {
			out[key] = coerced
		}
	}
	b, err := json.Marshal(out)
	if err != nil {
		return []byte("{}")
	}
	return b
}

func coerceDeviceHealthValue(key string, v any) (any, bool) {
	if v == nil {
		return nil, false
	}
	if key == "net_online" {
		switch t := v.(type) {
		case bool:
			return t, true
		case string:
			s := strings.TrimSpace(strings.ToLower(t))
			if s == "true" || s == "1" {
				return true, true
			}
			if s == "false" || s == "0" {
				return false, true
			}
		case float64:
			return t != 0, true
		}
		return nil, false
	}
	if _, numeric := deviceHealthNumericKeys[key]; numeric {
		switch t := v.(type) {
		case float64:
			return t, true
		case json.Number:
			f, err := t.Float64()
			if err != nil {
				return nil, false
			}
			return f, true
		case string:
			s := strings.TrimSpace(t)
			if s == "" {
				return nil, false
			}
			f, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return nil, false
			}
			return f, true
		}
		return nil, false
	}
	switch t := v.(type) {
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return nil, false
		}
		return s, true
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return nil, false
		}
		s := strings.Trim(string(b), `"`)
		if s == "" || s == "null" {
			return nil, false
		}
		return s, true
	}
}

func emptyJSONObject(b []byte) bool {
	s := strings.TrimSpace(string(b))
	return s == "" || s == "{}" || s == "null"
}

// MergeMachineMetricsState jsonb-merges allowlisted device health into machine_current_snapshot.metrics_state.
func (s *Store) MergeMachineMetricsState(ctx context.Context, machineID uuid.UUID, patch []byte) error {
	if s == nil || s.pool == nil {
		return errors.New("postgres: nil store")
	}
	if emptyJSONObject(patch) {
		return nil
	}
	loc, err := s.GetMachineOrgSite(ctx, machineID)
	if err != nil {
		return err
	}
	const q = `
INSERT INTO machine_current_snapshot (
	machine_id, site_id, reported_state, metrics_state, updated_at
) VALUES ($1, $2, '{}'::jsonb, $3::jsonb, now())
ON CONFLICT (machine_id) DO UPDATE SET
	metrics_state = COALESCE(machine_current_snapshot.metrics_state, '{}'::jsonb) || EXCLUDED.metrics_state,
	updated_at = now()
`
	_, err = s.pool.Exec(ctx, q, machineID, loc.SiteID, patch)
	return err
}
