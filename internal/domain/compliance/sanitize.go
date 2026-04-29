package compliance

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
)

var sensitiveKeySubstrings = []string{
	"password",
	"secret",
	"token",
	"authorization",
	"refresh",
	"accesskey",
	"apikey",
	"api_key",
	"webhook",
	"hmac",
	"pan",
	"card",
	"cvv",
	"cvc",
	"expiry",
	"exp_month",
	"exp_year",
	"security_code",
	"account_number",
	"iban",
}

var digitRunPattern = regexp.MustCompile(`\d[\d -]{11,}\d`)

// SanitizeJSONBytes returns a redacted copy of JSON bytes safe for audit storage.
// Non-JSON input is returned unchanged after trimming control chars.
func SanitizeJSONBytes(b []byte) []byte {
	if len(bytes.TrimSpace(b)) == 0 {
		return []byte("{}")
	}
	var v any
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return []byte(`{"_sanitize_error":"non_json_payload"}`)
	}
	sanitizeValue(v)
	out, err := json.Marshal(v)
	if err != nil {
		return []byte(`{"_sanitize_error":"marshal_failed"}`)
	}
	return out
}

func sanitizeValue(v any) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			lk := strings.ToLower(k)
			if keyLooksSensitive(lk) {
				t[k] = "[REDACTED]"
				continue
			}
			if s, ok := val.(string); ok {
				t[k] = redactSensitiveString(s)
				continue
			}
			sanitizeValue(val)
		}
	case []any:
		for i := range t {
			if s, ok := t[i].(string); ok {
				t[i] = redactSensitiveString(s)
				continue
			}
			sanitizeValue(t[i])
		}
	default:
		return
	}
}

func keyLooksSensitive(k string) bool {
	for _, s := range sensitiveKeySubstrings {
		if strings.Contains(k, s) {
			return true
		}
	}
	return false
}

func redactSensitiveString(s string) string {
	return digitRunPattern.ReplaceAllStringFunc(s, func(candidate string) string {
		digits := strings.NewReplacer(" ", "", "-", "").Replace(candidate)
		if len(digits) < 13 || len(digits) > 19 || !luhnValid(digits) {
			return candidate
		}
		return "[REDACTED]"
	})
}

func luhnValid(digits string) bool {
	var sum int
	double := false
	for i := len(digits) - 1; i >= 0; i-- {
		c := digits[i]
		if c < '0' || c > '9' {
			return false
		}
		n := int(c - '0')
		if double {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		double = !double
	}
	return sum > 0 && sum%10 == 0
}
