package postgres

import (
	"encoding/json"
	"testing"
)

func TestParseMetricsPayloadSamples(t *testing.T) {
	b, _ := json.Marshal(map[string]any{
		"samples": map[string]float64{"cpu": 0.5, "mem": 1.25},
	})
	m := ParseMetricsPayload(b)
	if m["cpu"] != 0.5 || m["mem"] != 1.25 {
		t.Fatalf("%v", m)
	}
}

func TestParseIncidentPayload(t *testing.T) {
	b, _ := json.Marshal(map[string]any{
		"severity":   "high",
		"code":       "door_open",
		"title":      "Door",
		"dedupe_key": "d1",
	})
	sev, code, title, dedupe, err := ParseIncidentPayload(b)
	if err != nil || sev != "high" || code != "door_open" || title != "Door" || dedupe != "d1" {
		t.Fatalf("%s %s %s %s %v", sev, code, title, dedupe, err)
	}
}
