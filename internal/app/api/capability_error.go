package api

// CapabilityError signals an HTTP surface that is intentionally not implemented yet.
// Handlers map this to HTTP 501 with a stable JSON envelope.
type CapabilityError struct {
	Capability string
	Message    string
}

func (e *CapabilityError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return "capability not implemented: " + e.Capability
}
