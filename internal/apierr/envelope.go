package apierr

// V1 builds the standard JSON API error document:
//
//	{"error":{"code":"...","message":"...","details":{...},"requestId":"..."}}
//
// details is normalized to a non-nil JSON object (empty map when nil).
func V1(requestID, code, message string, details map[string]any) map[string]any {
	d := details
	if d == nil {
		d = map[string]any{}
	}
	return map[string]any{
		"error": map[string]any{
			"code":      code,
			"message":   message,
			"details":   d,
			"requestId": requestID,
		},
	}
}
