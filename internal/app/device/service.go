package device

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	domaindevice "github.com/avf/avf-vending-api/internal/domain/device"
	"github.com/google/uuid"
)

// Service coordinates shadow reads/writes, command enqueue, and time-based assessments.
type Service struct {
	workflow domaindevice.CommandShadowWorkflow
	shadow   ShadowReader
	reported ReportedShadowWriter
	cmds     CommandLedgerReader
	machines MachinePresenceReader
	clk      Clock
}

type wallClock struct{}

func (wallClock) Now() time.Time { return time.Now() }

// NewService constructs an orchestrator. Workflow must be non-nil; Clock defaults to wall time.
func NewService(d Deps) *Service {
	if d.Workflow == nil {
		panic("device.NewService: nil CommandShadowWorkflow")
	}
	clk := d.Clock
	if clk == nil {
		clk = wallClock{}
	}
	return &Service{
		workflow: d.Workflow,
		shadow:   d.Shadow,
		reported: d.Reported,
		cmds:     d.Commands,
		machines: d.Machines,
		clk:      clk,
	}
}

var _ Orchestrator = (*Service)(nil)

// EnqueueCommand validates input and appends a ledger row while replacing desired_state.
func (s *Service) EnqueueCommand(ctx context.Context, in domaindevice.AppendCommandInput) (domaindevice.AppendCommandResult, error) {
	if err := validateAppendInput(in); err != nil {
		return domaindevice.AppendCommandResult{}, err
	}
	in.CommandType = strings.TrimSpace(in.CommandType)
	in.IdempotencyKey = strings.TrimSpace(in.IdempotencyKey)
	if len(in.Payload) > 0 && !json.Valid(in.Payload) {
		return domaindevice.AppendCommandResult{}, errors.Join(ErrInvalidArgument, errors.New("payload must be valid JSON"))
	}
	if len(in.DesiredState) > 0 && !json.Valid(in.DesiredState) {
		return domaindevice.AppendCommandResult{}, errors.Join(ErrInvalidArgument, errors.New("desired_state must be valid JSON"))
	}
	return s.workflow.AppendCommandUpdateShadow(ctx, in)
}

// EnqueueCommandMergeDesired shallow-merges desiredPatchJSON onto the current desired document, then enqueues.
func (s *Service) EnqueueCommandMergeDesired(ctx context.Context, base domaindevice.AppendCommandInput, desiredPatchJSON []byte) (domaindevice.AppendCommandResult, error) {
	if s.shadow == nil {
		return domaindevice.AppendCommandResult{}, ErrNotConfigured
	}
	if err := validateAppendInput(base); err != nil {
		return domaindevice.AppendCommandResult{}, err
	}
	base.CommandType = strings.TrimSpace(base.CommandType)
	base.IdempotencyKey = strings.TrimSpace(base.IdempotencyKey)
	if len(base.Payload) > 0 && !json.Valid(base.Payload) {
		return domaindevice.AppendCommandResult{}, errors.Join(ErrInvalidArgument, errors.New("payload must be valid JSON"))
	}

	sh, err := s.shadow.GetShadow(ctx, base.MachineID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			sh = ShadowDocument{
				MachineID:     base.MachineID,
				DesiredState:  []byte("{}"),
				ReportedState: []byte("{}"),
			}
		} else {
			return domaindevice.AppendCommandResult{}, err
		}
	}

	merged, err := mergeJSONObjectBytes(sh.DesiredState, desiredPatchJSON)
	if err != nil {
		return domaindevice.AppendCommandResult{}, errors.Join(ErrInvalidArgument, err)
	}
	if !json.Valid(merged) {
		return domaindevice.AppendCommandResult{}, errors.Join(ErrInvalidArgument, errors.New("merged desired_state invalid"))
	}

	base.DesiredState = merged
	return s.workflow.AppendCommandUpdateShadow(ctx, base)
}

// GetShadow returns the shadow document when a reader is configured.
func (s *Service) GetShadow(ctx context.Context, machineID uuid.UUID) (ShadowDocument, error) {
	if err := validateMachineID(machineID); err != nil {
		return ShadowDocument{}, err
	}
	if s.shadow == nil {
		return ShadowDocument{}, ErrNotConfigured
	}
	return s.shadow.GetShadow(ctx, machineID)
}

// RecordReportedState validates JSON, optionally checks shadow.version, and persists reported_state.
func (s *Service) RecordReportedState(ctx context.Context, in RecordReportedInput) (ShadowDocument, error) {
	if err := validateMachineID(in.MachineID); err != nil {
		return ShadowDocument{}, err
	}
	if s.shadow == nil || s.reported == nil {
		return ShadowDocument{}, ErrNotConfigured
	}
	if len(in.Reported) == 0 {
		in.Reported = []byte("{}")
	}
	if !json.Valid(in.Reported) {
		return ShadowDocument{}, errors.Join(ErrInvalidArgument, errors.New("reported must be valid JSON"))
	}

	if in.ExpectedVersion != nil {
		sh, err := s.shadow.GetShadow(ctx, in.MachineID)
		if err != nil {
			return ShadowDocument{}, err
		}
		if sh.Version != *in.ExpectedVersion {
			return ShadowDocument{}, ErrVersionMismatch
		}
	}

	return s.reported.ReplaceReportedState(ctx, in.MachineID, in.Reported)
}

// AssessCommandDispatch compares the latest command creation time to dispatchWaitTimeout (advisory).
func (s *Service) AssessCommandDispatch(ctx context.Context, machineID uuid.UUID, dispatchWaitTimeout time.Duration) (CommandDispatchAssessment, error) {
	if err := validateMachineID(machineID); err != nil {
		return CommandDispatchAssessment{}, err
	}
	if s.cmds == nil {
		return CommandDispatchAssessment{}, ErrNotConfigured
	}
	if dispatchWaitTimeout <= 0 {
		return CommandDispatchAssessment{}, errors.Join(ErrInvalidArgument, errors.New("dispatchWaitTimeout must be positive"))
	}

	cmd, err := s.cmds.GetLatestCommand(ctx, machineID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return CommandDispatchAssessment{HasCommand: false}, nil
		}
		return CommandDispatchAssessment{}, err
	}

	now := s.clk.Now()
	elapsed := now.Sub(cmd.CreatedAt)
	out := CommandDispatchAssessment{
		HasCommand: true,
		Sequence:   cmd.Sequence,
		CreatedAt:  cmd.CreatedAt,
	}
	if elapsed > dispatchWaitTimeout {
		out.PendingTooLong = true
		out.OverTimeoutBy = elapsed - dispatchWaitTimeout
	}
	return out, nil
}

// AssessMachineReachability treats staleAfter relative to machines.updated_at together with row status.
func (s *Service) AssessMachineReachability(ctx context.Context, machineID uuid.UUID, staleAfter time.Duration) (MachineReachabilityAssessment, error) {
	if err := validateMachineID(machineID); err != nil {
		return MachineReachabilityAssessment{}, err
	}
	if s.machines == nil {
		return MachineReachabilityAssessment{}, ErrNotConfigured
	}
	if staleAfter <= 0 {
		return MachineReachabilityAssessment{}, errors.Join(ErrInvalidArgument, errors.New("staleAfter must be positive"))
	}

	mp, err := s.machines.GetMachinePresence(ctx, machineID)
	if err != nil {
		return MachineReachabilityAssessment{}, err
	}

	now := s.clk.Now()
	stale := now.Sub(mp.UpdatedAt) > staleAfter
	reason := ""
	switch {
	case mp.Status != "online":
		reason = "machine status is not online"
	case stale:
		reason = "machine row activity older than stale threshold"
	default:
		reason = "ok"
	}

	return MachineReachabilityAssessment{
		MachineID:            mp.MachineID,
		Status:               mp.Status,
		LastRecordActivityAt: mp.UpdatedAt,
		Stale:                stale || mp.Status != "online",
		Reason:               reason,
	}, nil
}

func validateMachineID(id uuid.UUID) error {
	if id == uuid.Nil {
		return errors.Join(ErrInvalidArgument, errors.New("machine_id must be set"))
	}
	return nil
}

func validateAppendInput(in domaindevice.AppendCommandInput) error {
	if err := validateMachineID(in.MachineID); err != nil {
		return err
	}
	if strings.TrimSpace(in.CommandType) == "" {
		return errors.Join(ErrInvalidArgument, errors.New("command_type is required"))
	}
	if strings.TrimSpace(in.IdempotencyKey) == "" {
		return errors.Join(ErrInvalidArgument, errors.New("idempotency_key is required"))
	}
	return nil
}

func mergeJSONObjectBytes(base, patch []byte) ([]byte, error) {
	base = bytes.TrimSpace(base)
	if len(base) == 0 {
		base = []byte("{}")
	}
	patch = bytes.TrimSpace(patch)
	if len(patch) == 0 {
		return base, nil
	}

	var bObj map[string]any
	if err := json.Unmarshal(base, &bObj); err != nil {
		return nil, err
	}
	var pObj map[string]any
	if err := json.Unmarshal(patch, &pObj); err != nil {
		return nil, err
	}
	for k, v := range pObj {
		bObj[k] = v
	}
	return json.Marshal(bObj)
}
