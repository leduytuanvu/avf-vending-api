package layoutassignment

import (
	"encoding/json"
)

// ParseMirrorSlots decodes the JSON array stored on machine_local_layout_mirror.slots.
func ParseMirrorSlots(raw []byte) []LocalMirrorSlotView {
	return parseMirrorSlots(raw)
}

func parseMirrorSlots(raw []byte) []LocalMirrorSlotView {
	if len(raw) == 0 || !json.Valid(raw) {
		return nil
	}
	var slots []LocalMirrorSlotView
	if err := json.Unmarshal(raw, &slots); err != nil {
		return nil
	}
	return slots
}
