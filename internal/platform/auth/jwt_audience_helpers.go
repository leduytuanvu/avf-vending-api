package auth

import (
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// jwtMapClaimsAudienceAllowed returns true when token audiences overlap allowed (case-insensitive).
// When allowed is empty, any token is accepted (caller should not require aud).
func jwtMapClaimsAudienceAllowed(claims jwt.MapClaims, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	tokAud, err := claims.GetAudience()
	if err != nil || len(tokAud) == 0 {
		return false
	}
	for _, want := range allowed {
		w := strings.TrimSpace(want)
		if w == "" {
			continue
		}
		for _, got := range tokAud {
			if strings.EqualFold(strings.TrimSpace(got), w) {
				return true
			}
		}
	}
	return false
}
