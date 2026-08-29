package grpcserver

import (
	"context"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/layoutassignment"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/pgxutil"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type bootstrapLayoutBundle struct {
	serverRow       db.MachineLayoutAssignment
	hasServer       bool
	localRow        db.MachineLayoutAssignment
	hasLocal        bool
	state           db.MachineLayoutState
	hasState        bool
	localMirror     db.MachineLocalLayoutMirror
	hasLocalMirror  bool
}

func loadBootstrapLayoutBundle(ctx context.Context, deps MachineGRPCServicesDeps, machineID uuid.UUID) bootstrapLayoutBundle {
	var out bootstrapLayoutBundle
	if deps.Pool == nil {
		return out
	}
	q := pgxutil.NewQueries(deps.Pool)
	if row, err := q.GetCurrentMachineLayoutAssignment(ctx, db.GetCurrentMachineLayoutAssignmentParams{
		MachineID: machineID,
		Source:    layoutassignment.SourceServer,
	}); err == nil {
		out.serverRow = row
		out.hasServer = true
	}
	if row, err := q.GetCurrentMachineLayoutAssignment(ctx, db.GetCurrentMachineLayoutAssignmentParams{
		MachineID: machineID,
		Source:    layoutassignment.SourceLocal,
	}); err == nil {
		out.localRow = row
		out.hasLocal = true
	}
	if st, err := q.GetMachineLayoutState(ctx, machineID); err == nil {
		out.state = st
		out.hasState = true
	} else if err != pgx.ErrNoRows {
		// ignore transient errors; bootstrap still serves without layout state
	}
	if mirror, err := q.GetMachineLocalLayoutMirror(ctx, machineID); err == nil {
		out.localMirror = mirror
		out.hasLocalMirror = true
	}
	return out
}

func (b bootstrapLayoutBundle) serverGridRows() int32 {
	if b.hasServer && b.serverRow.GridRows > 0 {
		return b.serverRow.GridRows
	}
	return 0
}

func (b bootstrapLayoutBundle) serverGridCols() int32 {
	if b.hasServer && b.serverRow.GridCols > 0 {
		return b.serverRow.GridCols
	}
	return 0
}

func mapLayoutSourceProto(source string) machinev1.LayoutSource {
	switch strings.ToUpper(strings.TrimSpace(source)) {
	case layoutassignment.SourceServer:
		return machinev1.LayoutSource_LAYOUT_SOURCE_SERVER
	case layoutassignment.SourceLocal:
		return machinev1.LayoutSource_LAYOUT_SOURCE_LOCAL
	default:
		return machinev1.LayoutSource_LAYOUT_SOURCE_UNSPECIFIED
	}
}

func layoutSourceFromProto(s machinev1.LayoutSource) string {
	switch s {
	case machinev1.LayoutSource_LAYOUT_SOURCE_SERVER:
		return layoutassignment.SourceServer
	case machinev1.LayoutSource_LAYOUT_SOURCE_LOCAL:
		return layoutassignment.SourceLocal
	default:
		return ""
	}
}

func mapAssignmentRowProto(row db.MachineLayoutAssignment) *machinev1.MachineLayoutAssignment {
	out := &machinev1.MachineLayoutAssignment{
		Source:       mapLayoutSourceProto(row.Source),
		AssignmentId: row.ID.String(),
		Revision:     row.Revision,
		GridRows:     row.GridRows,
		GridCols:     row.GridCols,
		Fingerprint:  row.Fingerprint,
		PublishedAt:  timestamppb.New(row.EffectiveFrom.UTC()),
	}
	if row.LayoutID.Valid {
		out.LayoutId = uuid.UUID(row.LayoutID.Bytes).String()
	}
	if row.LayoutVersionID.Valid {
		out.LayoutVersionId = uuid.UUID(row.LayoutVersionID.Bytes).String()
	}
	return out
}

func mapLocalMirrorProto(m db.MachineLocalLayoutMirror) *machinev1.MachineLayoutAssignment {
	return &machinev1.MachineLayoutAssignment{
		Source:       machinev1.LayoutSource_LAYOUT_SOURCE_LOCAL,
		AssignmentId: m.LocalLayoutID.String(),
		Revision:     m.Revision,
		GridRows:     m.GridRows,
		GridCols:     m.GridCols,
		Fingerprint:  m.Fingerprint,
		PublishedAt:  timestamppb.New(m.ReportedAt.UTC()),
	}
}

func applyBootstrapLayoutFields(resp *machinev1.GetBootstrapResponse, bundle bootstrapLayoutBundle) {
	if resp == nil {
		return
	}
	if bundle.hasServer {
		resp.ServerLayout = mapAssignmentRowProto(bundle.serverRow)
	}
	if bundle.hasLocalMirror {
		resp.LocalLayout = mapLocalMirrorProto(bundle.localMirror)
	} else if bundle.hasLocal {
		resp.LocalLayout = mapAssignmentRowProto(bundle.localRow)
	}
	if bundle.hasState && bundle.state.DesiredSource.Valid {
		resp.DesiredSource = mapLayoutSourceProto(bundle.state.DesiredSource.String)
	}
	if bundle.hasState && bundle.state.DesiredFingerprint.Valid {
		resp.LayoutFingerprint = strings.TrimSpace(bundle.state.DesiredFingerprint.String)
	} else if bundle.hasServer {
		resp.LayoutFingerprint = strings.TrimSpace(bundle.serverRow.Fingerprint)
	}
}

func layoutChangedSinceClient(desiredFP string, clientFP string) bool {
	d := strings.TrimSpace(desiredFP)
	c := strings.TrimSpace(clientFP)
	if d == "" {
		return false
	}
	return c != d
}

func desiredLayoutFingerprint(bundle bootstrapLayoutBundle) string {
	if bundle.hasState && bundle.state.DesiredFingerprint.Valid {
		return strings.TrimSpace(bundle.state.DesiredFingerprint.String)
	}
	if bundle.hasServer {
		return strings.TrimSpace(bundle.serverRow.Fingerprint)
	}
	return ""
}

// layoutStateReportedAt returns the latest device report time when available.
func layoutStateReportedAt(bundle bootstrapLayoutBundle) *time.Time {
	if bundle.hasState && bundle.state.ReportedAt.Valid {
		t := bundle.state.ReportedAt.Time
		return &t
	}
	if bundle.hasLocalMirror {
		t := bundle.localMirror.ReportedAt
		return &t
	}
	return nil
}
