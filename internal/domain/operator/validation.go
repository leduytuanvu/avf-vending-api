package operator

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// MaxOperatorSessionTTL caps how long a temporary operator session may remain valid.
// Machine identity is stable; operator sessions are intentionally short-lived credentials on top.
const MaxOperatorSessionTTL = 90 * 24 * time.Hour

// DefaultStaleIdleReclaimForDifferentOperator is the maximum operator silence (no heartbeat/login
// resume touching last_activity_at) before another principal may start a session on the same machine.
// Heartbeats and same-actor login resume refresh this clock; see StartOperatorSession in the repository.
const DefaultStaleIdleReclaimForDifferentOperator = 15 * time.Minute

// ValidateSessionExpiryBounds rejects expires_at in the past or unreasonably far in the future.
func ValidateSessionExpiryBounds(exp *time.Time, now time.Time, maxTTL time.Duration) error {
	if exp == nil {
		return nil
	}
	if !exp.After(now) {
		return ErrInvalidSessionExpiry
	}
	if exp.Sub(now) > maxTTL {
		return ErrInvalidSessionExpiry
	}
	return nil
}

// ValidateActorConsistency enforces the same shape as ck_operator_session_actor_shape.
func ValidateActorConsistency(actorType string, technicianID *uuid.UUID, userPrincipal *string) error {
	switch actorType {
	case ActorTypeTechnician:
		if technicianID == nil || *technicianID == uuid.Nil {
			return ErrInvalidActor
		}
		if userPrincipal != nil && strings.TrimSpace(*userPrincipal) != "" {
			return ErrInvalidActor
		}
	case ActorTypeUser:
		if technicianID != nil {
			return ErrInvalidActor
		}
		if userPrincipal == nil || strings.TrimSpace(*userPrincipal) == "" {
			return ErrInvalidActor
		}
	default:
		return ErrInvalidActor
	}
	return nil
}

func validateAuthMethod(m string) error {
	switch m {
	case AuthMethodPIN, AuthMethodPassword, AuthMethodBadge, AuthMethodOIDC, AuthMethodDeviceCert, AuthMethodUnknown:
		return nil
	default:
		return ErrInvalidAuthMethod
	}
}

func validateAuthEventType(t string) error {
	switch t {
	case AuthEventLoginSuccess, AuthEventLoginFailure, AuthEventLogout, AuthEventSessionRefresh, AuthEventLockout, AuthEventUnknown:
		return nil
	default:
		return ErrInvalidAuthEventType
	}
}

func validateActionOriginType(t string) error {
	switch t {
	case ActionOriginOperatorSession, ActionOriginSystem, ActionOriginScheduled, ActionOriginAPI, ActionOriginRemoteSupport:
		return nil
	default:
		return ErrInvalidActionOriginType
	}
}

// ValidateAuthEventSemantics checks enum-like fields before persistence.
func ValidateAuthEventSemantics(eventType, authMethod string) error {
	if err := validateAuthEventType(eventType); err != nil {
		return err
	}
	return validateAuthMethod(authMethod)
}

// ValidateActionAttributionSemantics checks action_origin_type before persistence.
func ValidateActionAttributionSemantics(origin string) error {
	return validateActionOriginType(origin)
}
