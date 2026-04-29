package auth

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

// RawJWTClaims mirrors the JWT private claims this API accepts (HS256 and RS256 paths).
type RawJWTClaims struct {
	Sub            string          `json:"sub"`
	Roles          []string        `json:"roles"`
	OrgID          string          `json:"org_id"`
	SiteID         string          `json:"site_id"`
	MachineIDs     []string        `json:"machine_ids"`
	MachineID      string          `json:"machine_id"`
	TechnicianID   string          `json:"technician_id"`
	ActorType      string          `json:"actor_type"`
	ServiceAccount string          `json:"service_account"`
	AccountStatus  string          `json:"account_status"`
	Typ            string          `json:"typ"`
	Aud            json.RawMessage `json:"aud"`
	TokenVersion   int64           `json:"token_version"`
	Scopes         []string        `json:"scopes"`
	Jti            string          `json:"jti"`
	Exp            int64           `json:"exp"`
	TokenUse       string          `json:"token_use"`
	MFAEnrollment  bool            `json:"mfa_enrollment"`
}

// PrincipalFromJWTPayloadJSON maps validated JWT payload bytes into Principal (expiry enforced).
func PrincipalFromJWTPayloadJSON(payloadJSON []byte, leeway time.Duration) (Principal, error) {
	if leeway <= 0 {
		leeway = DefaultClockLeeway
	}
	var rc RawJWTClaims
	if err := json.Unmarshal(payloadJSON, &rc); err != nil {
		return Principal{}, ErrUnauthenticated
	}
	if rc.Exp > 0 {
		expAt := time.Unix(rc.Exp, 0).UTC()
		if time.Now().UTC().After(expAt.Add(leeway)) {
			return Principal{}, ErrUnauthenticated
		}
	}

	p := Principal{
		Subject:       strings.TrimSpace(rc.Sub),
		ActorType:     strings.TrimSpace(rc.ActorType),
		Roles:         rc.Roles,
		AccountStatus: strings.TrimSpace(rc.AccountStatus),
		ExpiresAt:     time.Unix(rc.Exp, 0).UTC(),
		JWTType:       strings.TrimSpace(rc.Typ),
		JWTAudience:   audienceStringFromRawJSON(rc.Aud),
		JTI:           strings.TrimSpace(rc.Jti),
		TokenUse:      strings.TrimSpace(rc.TokenUse),
		MFAEnrollment: rc.MFAEnrollment,
	}
	if p.Subject == "" {
		if strings.TrimSpace(rc.ServiceAccount) != "" {
			p.Subject = strings.TrimSpace(rc.ServiceAccount)
			if p.ActorType == "" {
				p.ActorType = ActorTypeService
			}
			if !p.HasRole(RoleService) {
				p.Roles = append(p.Roles, RoleService)
			}
		}
	}
	if p.Subject == "" {
		return Principal{}, ErrUnauthenticated
	}

	if id, err := uuid.Parse(strings.TrimSpace(rc.OrgID)); err == nil {
		p.OrganizationID = id
	}
	if id, err := uuid.Parse(strings.TrimSpace(rc.SiteID)); err == nil {
		p.SiteID = id
	}
	if id, err := uuid.Parse(strings.TrimSpace(rc.TechnicianID)); err == nil {
		p.TechnicianID = id
	}
	for _, s := range rc.MachineIDs {
		id, err := uuid.Parse(strings.TrimSpace(s))
		if err == nil && id != uuid.Nil {
			p.MachineIDs = appendMachineIDUnique(p.MachineIDs, id)
		}
	}
	if mid := strings.TrimSpace(rc.MachineID); mid != "" {
		if id, err := uuid.Parse(mid); err == nil && id != uuid.Nil {
			p.MachineIDs = appendMachineIDUnique(p.MachineIDs, id)
		}
	}
	if strings.HasPrefix(strings.ToLower(p.Subject), "machine:") {
		rest := strings.TrimSpace(p.Subject[len("machine:"):])
		if id, err := uuid.Parse(rest); err == nil && id != uuid.Nil {
			p.MachineIDs = appendMachineIDUnique(p.MachineIDs, id)
		}
	}

	return p, nil
}

func appendMachineIDUnique(ids []uuid.UUID, id uuid.UUID) []uuid.UUID {
	for _, x := range ids {
		if x == id {
			return ids
		}
	}
	return append(ids, id)
}

func audienceStringFromRawJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case []any:
		if len(t) == 0 {
			return ""
		}
		if s, ok := t[0].(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}
