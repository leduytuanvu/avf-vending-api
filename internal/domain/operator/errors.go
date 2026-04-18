package operator

import "errors"

var (
	ErrInvalidActor                        = errors.New("operator: actor_type does not match technician_id/user_principal")
	ErrOrganizationMismatch                = errors.New("operator: organization context does not match machine or technician")
	ErrActiveSessionExists                 = errors.New("operator: machine already has an active session")
	ErrNoActiveSession                     = errors.New("operator: no active session for machine")
	ErrSessionNotFound                     = errors.New("operator: session not found")
	ErrSessionMachineMismatch              = errors.New("operator: session does not belong to this machine")
	ErrSessionNotActive                    = errors.New("operator: session is not active")
	ErrTimeoutNotApplicable                = errors.New("operator: session is not eligible for expiry timeout")
	ErrInvalidAuthMethod                   = errors.New("operator: unknown auth_method")
	ErrInvalidAuthEventType                = errors.New("operator: unknown auth event_type")
	ErrInvalidActionOriginType             = errors.New("operator: unknown action_origin_type")
	ErrInvalidSessionEndStatus             = errors.New("operator: end status must be ENDED or REVOKED")
	ErrTechnicianNotAssignedToMachine      = errors.New("operator: technician has no active assignment to this machine")
	ErrTechnicianAssignmentCheckerRequired = errors.New("operator: technician login requires a configured assignment checker")
	ErrInvalidSessionExpiry                = errors.New("operator: expires_at must be in the future and within the maximum session lifetime")
	ErrMachineContextRequired              = errors.New("operator: machine_id is required for this operation")
	ErrAdminTakeoverUnauthorized           = errors.New("operator: admin takeover is not permitted for this principal")
)
