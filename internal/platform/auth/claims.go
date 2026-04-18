package auth

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

// RawJWTClaims mirrors the JWT private claims this API accepts (HS256 and RS256 paths).
type RawJWTClaims struct {
	Sub            string   `json:"sub"`
	Roles          []string `json:"roles"`
	OrgID          string   `json:"org_id"`
	SiteID         string   `json:"site_id"`
	MachineIDs     []string `json:"machine_ids"`
	TechnicianID   string   `json:"technician_id"`
	ActorType      string   `json:"actor_type"`
	ServiceAccount string   `json:"service_account"`
	Exp            int64    `json:"exp"`
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
		Subject:   strings.TrimSpace(rc.Sub),
		ActorType: strings.TrimSpace(rc.ActorType),
		Roles:     rc.Roles,
		ExpiresAt: time.Unix(rc.Exp, 0).UTC(),
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
			p.MachineIDs = append(p.MachineIDs, id)
		}
	}

	return p, nil
}
