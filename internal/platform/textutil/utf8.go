package textutil

import (
	"strings"
	"unicode/utf8"
)

// SanitizeProtoString ensures s is valid UTF-8 for protobuf string fields.
func SanitizeProtoString(s string) string {
	if s == "" {
		return s
	}
	if utf8.ValidString(s) {
		return s
	}
	return strings.ToValidUTF8(s, "")
}
