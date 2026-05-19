package mqtt

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// CommandTypeCatalogRefresh is the MQTT command ledger type for “pull latest catalog + media” triggers.
// Payload is small metadata only (version cursors + reason); never URLs or image bytes over MQTT.
const CommandTypeCatalogRefresh = "catalog.refresh"

// ReasonProductMediaUpdated is a stable reason label for operator/analytics filtering.
const ReasonProductMediaUpdated = "product_media_updated"

var (
	// ErrCatalogRefreshPayloadInvalid indicates dispatch or ACK JSON failed catalog.refresh schema checks.
	ErrCatalogRefreshPayloadInvalid = errors.New("mqtt: catalog.refresh payload invalid")
)

var catalogRefreshDispatchAllowedKeys = map[string]struct{}{
	"type":                 {},
	"catalogVersion":       {},
	"mediaManifestVersion": {},
	"reason":               {},
	"commandId":            {},
	"command_id":           {}, // tolerant snake_case echo (ignored by kiosk contract)
}

var catalogRefreshAckAllowedKeys = map[string]struct{}{
	"catalogVersion":       {},
	"mediaManifestVersion": {},
	"mediaSynced":          {},
	"commandId":            {},
	"command_id":           {},
	"type":                 {},
	"detail":               {},
}

// ValidateCatalogRefreshDispatchPayload enforces the outbound inner payload shape for ledger.command_type=catalog.refresh.
func ValidateCatalogRefreshDispatchPayload(commandType string, raw []byte) error {
	if strings.TrimSpace(commandType) != CommandTypeCatalogRefresh {
		return nil
	}
	if len(raw) == 0 {
		return fmt.Errorf("%w: empty payload", ErrCatalogRefreshPayloadInvalid)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return fmt.Errorf("%w: payload must be a JSON object", ErrCatalogRefreshPayloadInvalid)
	}
	for k := range m {
		if _, ok := catalogRefreshDispatchAllowedKeys[k]; !ok {
			return fmt.Errorf("%w: unknown field %q", ErrCatalogRefreshPayloadInvalid, k)
		}
	}
	typ, _ := m["type"].(string)
	if strings.TrimSpace(typ) != CommandTypeCatalogRefresh {
		return fmt.Errorf("%w: type must be %q", ErrCatalogRefreshPayloadInvalid, CommandTypeCatalogRefresh)
	}
	if _, err := requireMQTTVersionToken(m, "catalogVersion"); err != nil {
		return err
	}
	if _, err := requireMQTTVersionToken(m, "mediaManifestVersion"); err != nil {
		return err
	}
	reason, _ := m["reason"].(string)
	if strings.TrimSpace(reason) == "" {
		return fmt.Errorf("%w: reason is required", ErrCatalogRefreshPayloadInvalid)
	}
	if err := forbidSuspiciousCatalogRefreshValues(m); err != nil {
		return err
	}
	return nil
}

// ValidateCatalogRefreshAckPayload validates nested MQTT commands/ack payload for successful catalog.refresh receipts.
func ValidateCatalogRefreshAckPayload(commandType string, status string, raw []byte) error {
	if !strings.EqualFold(strings.TrimSpace(commandType), CommandTypeCatalogRefresh) {
		return nil
	}
	st := strings.TrimSpace(strings.ToLower(status))
	if st != "acked" {
		return nil
	}
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return fmt.Errorf("%w: ack payload must be a JSON object", ErrCatalogRefreshPayloadInvalid)
	}
	for k := range m {
		if _, ok := catalogRefreshAckAllowedKeys[k]; !ok {
			return fmt.Errorf("%w: unknown ack field %q", ErrCatalogRefreshPayloadInvalid, k)
		}
	}
	ms, ok := m["mediaSynced"].(bool)
	if !ok || !ms {
		return fmt.Errorf("%w: mediaSynced must be true for successful catalog.refresh ack", ErrCatalogRefreshPayloadInvalid)
	}
	if _, err := requireMQTTVersionToken(m, "catalogVersion"); err != nil {
		return err
	}
	if _, err := requireMQTTVersionToken(m, "mediaManifestVersion"); err != nil {
		return err
	}
	if err := forbidSuspiciousCatalogRefreshValues(m); err != nil {
		return err
	}
	return nil
}

func requireMQTTVersionToken(m map[string]any, key string) (string, error) {
	s, err := mqttVersionTokenFromAny(m[key])
	if err != nil || strings.TrimSpace(s) == "" {
		return "", fmt.Errorf("%w: %s is required", ErrCatalogRefreshPayloadInvalid, key)
	}
	return s, nil
}

func mqttVersionTokenFromAny(v any) (string, error) {
	if v == nil {
		return "", errors.New("missing")
	}
	switch t := v.(type) {
	case string:
		s := strings.TrimSpace(t)
		if len(s) > 2048 {
			return "", fmt.Errorf("value too long")
		}
		return s, nil
	case float64:
		if math.IsNaN(t) || math.IsInf(t, 0) {
			return "", fmt.Errorf("invalid number")
		}
		if t < 0 || t != math.Trunc(t) {
			return "", fmt.Errorf("catalog versions must be non-negative integers when numeric")
		}
		if t > math.MaxInt64 {
			return "", fmt.Errorf("integer overflow")
		}
		return strconv.FormatInt(int64(t), 10), nil
	case json.Number:
		i, err := t.Int64()
		if err != nil {
			return "", err
		}
		if i < 0 {
			return "", fmt.Errorf("negative")
		}
		return strconv.FormatInt(i, 10), nil
	default:
		return "", fmt.Errorf("unsupported type %T", v)
	}
}

func forbidSuspiciousCatalogRefreshValues(m map[string]any) error {
	for k, v := range m {
		switch s := v.(type) {
		case string:
			if looksLikeEmbeddedImageTransport(s) {
				return fmt.Errorf("%w: field %q must not carry image transport material", ErrCatalogRefreshPayloadInvalid, k)
			}
		default:
			if _, ok := v.(map[string]any); ok {
				return fmt.Errorf("%w: field %q must not be an object", ErrCatalogRefreshPayloadInvalid, k)
			}
			if _, ok := v.([]any); ok {
				return fmt.Errorf("%w: field %q must not be an array", ErrCatalogRefreshPayloadInvalid, k)
			}
		}
	}
	return nil
}

func looksLikeEmbeddedImageTransport(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	low := strings.ToLower(s)
	if strings.HasPrefix(low, "data:image/") && strings.Contains(low, ";base64,") {
		return true
	}
	if strings.HasPrefix(low, "base64,") {
		return true
	}
	// Heuristic: very long alphanumeric chunks often indicate accidental binary/base64 payloads.
	if len(s) > 256 && strings.Contains(low, "base64") {
		return true
	}
	return false
}
