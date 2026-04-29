package otaadmin

import "errors"

var (
	ErrNotFound             = errors.New("otaadmin: not found")
	ErrInvalidArgument      = errors.New("otaadmin: invalid argument")
	ErrInvalidTransition    = errors.New("otaadmin: invalid status transition")
	ErrTargetsLocked        = errors.New("otaadmin: campaign targets are immutable after rollout starts")
	ErrNeedsApproval        = errors.New("otaadmin: campaign must be approved before start")
	ErrRollbackArtifact     = errors.New("otaadmin: rollback requires rollback artifact")
	ErrNoTargets            = errors.New("otaadmin: no target machines configured")
	ErrMachinesNotInOrg     = errors.New("otaadmin: one or more machines are not in this organization")
	ErrRolloutNotActive     = errors.New("otaadmin: rollout is not active for this operation")
	ErrNothingLeftToRollout = errors.New("otaadmin: no remaining machines in rollout queue")
)
