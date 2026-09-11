package layoutassignment

import (
	"encoding/json"
)

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
