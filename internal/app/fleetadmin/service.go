package fleetadmin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/listscope"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// Service backs read-only admin fleet operational lists.
type Service struct {
	q *db.Queries
}

// NewService returns a fleet admin list service backed by sqlc queries.
func NewService(q *db.Queries) (*Service, error) {
	if q == nil {
		return nil, errors.New("fleetadmin: nil queries")
	}
	return &Service{q: q}, nil
}

func timeRangeOrAll(from, to *time.Time) (time.Time, time.Time) {
	start := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC)
	if from != nil {
		start = from.UTC()
	}
	if to != nil {
		end = to.UTC()
	}
	return start, end
}

func pgTextStringPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	s := t.String
	return &s
}

func sqlcScalarString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return fmt.Sprint(t)
	}
}

func pgUUIDStringPtr(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	s := uuid.UUID(u.Bytes).String()
	return &s
}

func pgTimestamptzTimePtr(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	tt := ts.Time.UTC()
	return &tt
}

func pgTimestamptzRFC3339NanoString(ts pgtype.Timestamptz) *string {
	if !ts.Valid {
		return nil
	}
	s := ts.Time.UTC().Format(time.RFC3339Nano)
	return &s
}

func textFromPgtypeText(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

func tsPgtypeTimestamptzToRFC3339Nano(ts pgtype.Timestamptz) string {
	if !ts.Valid {
		return ""
	}
	return ts.Time.UTC().Format(time.RFC3339Nano)
}

func baseItemFromFleetListRow(m db.FleetAdminListMachinesRow) AdminMachineListItem {
	return AdminMachineListItem{
		MachineID:           m.ID.String(),
		MachineName:         m.Name,
		Code:                m.Code,
		Model:               sqlcScalarString(m.Model),
		SiteID:              m.SiteID.String(),
		SiteName:            m.SiteName,
		HardwareProfileID:   pgUUIDStringPtr(m.HardwareProfileID),
		SerialNumber:        m.SerialNumber,
		Name:                m.Name,
		Status:              m.Status,
		CommandSequence:     m.CommandSequence,
		CreatedAt:           m.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:           m.UpdatedAt.UTC().Format(time.RFC3339Nano),
		AndroidID:           pgTextStringPtr(m.AndroidID),
		SimSerial:           pgTextStringPtr(m.SimSerial),
		SimIccid:            pgTextStringPtr(m.SimIccid),
		AppVersion:          pgTextStringPtr(m.AppVersion),
		FirmwareVersion:     pgTextStringPtr(m.FirmwareVersion),
		LastHeartbeatAt:     pgTimestamptzRFC3339NanoString(m.LastHeartbeatAt),
		EffectiveTimezone:   m.EffectiveTimezone,
		AssignedTechnicians: nil,
		CurrentOperator:     nil,
		InventorySummary:    AdminMachineInventorySummary{},
	}
}

func baseItemFromFleetDetailRow(m db.FleetAdminGetMachineDetailRow) AdminMachineListItem {
	return AdminMachineListItem{
		MachineID:             m.ID.String(),
		MachineName:           m.Name,
		Code:                  m.Code,
		Model:                 sqlcScalarString(m.Model),
		SiteID:                m.SiteID.String(),
		SiteName:              m.SiteName,
		HardwareProfileID:     pgUUIDStringPtr(m.HardwareProfileID),
		SerialNumber:          m.SerialNumber,
		Name:                  m.Name,
		Status:                m.Status,
		CommandSequence:       m.CommandSequence,
		CreatedAt:             m.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:             m.UpdatedAt.UTC().Format(time.RFC3339Nano),
		AndroidID:             pgTextStringPtr(m.AndroidID),
		SimSerial:             pgTextStringPtr(m.SimSerial),
		SimIccid:              pgTextStringPtr(m.SimIccid),
		AppVersion:            pgTextStringPtr(m.AppVersion),
		FirmwareVersion:       pgTextStringPtr(m.FirmwareVersion),
		LastHeartbeatAt:       pgTimestamptzRFC3339NanoString(m.LastHeartbeatAt),
		EffectiveTimezone:     m.EffectiveTimezone,
		AssignedTechnicians:   nil,
		CurrentOperator:       nil,
		InventorySummary:      AdminMachineInventorySummary{},
		EffectiveDeviceConfig: jsonbRawOrNil(m.EffectiveDeviceConfig),
		DeviceConfigFieldAck:  jsonbRawOrNil(m.DeviceConfigFieldAck),
	}
}

func jsonbRawOrNil(raw []byte) json.RawMessage {
	if len(raw) == 0 || string(raw) == "{}" || string(raw) == "null" {
		return nil
	}
	return json.RawMessage(raw)
}

func (s *Service) applyMachineEnrichment(
	item *AdminMachineListItem,
	tech map[uuid.UUID][]AdminAssignedTechnician,
	op map[uuid.UUID]*AdminCurrentOperator,
	inv map[uuid.UUID]AdminMachineInventorySummary,
	machineID uuid.UUID,
) {
	assign := tech[machineID]
	if assign == nil {
		assign = []AdminAssignedTechnician{}
	}
	item.AssignedTechnicians = assign
	item.CurrentOperator = op[machineID]
	item.InventorySummary = inv[machineID]
}

func operatorViewMissing(err error) bool {
	if err == nil {
		return false
	}
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == "42P01" && strings.Contains(pe.Message, "v_machine_current_operator") {
		return true
	}
	msg := err.Error()
	// Match pgx/sql error text even when SQLSTATE is omitted from err.Error().
	if strings.Contains(msg, "v_machine_current_operator") && strings.Contains(msg, "does not exist") {
		return true
	}
	return strings.Contains(msg, "v_machine_current_operator") && strings.Contains(msg, "42P01")
}

func (s *Service) loadFleetEnrichment(ctx context.Context, scopeID uuid.UUID, machineIDs []uuid.UUID) (
	map[uuid.UUID][]AdminAssignedTechnician,
	map[uuid.UUID]*AdminCurrentOperator,
	map[uuid.UUID]AdminMachineInventorySummary,
	error,
) {
	techByMachine := make(map[uuid.UUID][]AdminAssignedTechnician)
	opByMachine := make(map[uuid.UUID]*AdminCurrentOperator)
	invByMachine := make(map[uuid.UUID]AdminMachineInventorySummary)
	for _, id := range machineIDs {
		invByMachine[id] = AdminMachineInventorySummary{}
	}
	if len(machineIDs) == 0 {
		return techByMachine, opByMachine, invByMachine, nil
	}

	aRows, err := s.q.FleetAdminListActiveTechnicianAssignmentsForMachines(ctx, machineIDs)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, r := range aRows {
		techByMachine[r.MachineID] = append(techByMachine[r.MachineID], AdminAssignedTechnician{
			TechnicianID: r.TechnicianID.String(),
			DisplayName:  r.TechnicianDisplayName,
			Role:         r.Role,
			ValidFrom:    r.ValidFrom.UTC().Format(time.RFC3339Nano),
			ValidTo:      pgTimestamptzRFC3339NanoString(r.ValidTo),
		})
	}

	oRows, err := s.q.FleetAdminListViewOperatorsForMachines(ctx, machineIDs)
	if err != nil {
		if operatorViewMissing(err) {
			oRows = nil
		} else {
			return nil, nil, nil, err
		}
	}
	for _, r := range oRows {
		if !r.OperatorSessionID.Valid {
			opByMachine[r.MachineID] = nil
			continue
		}
		sid := uuid.UUID(r.OperatorSessionID.Bytes).String()
		var techID *string
		if r.TechnicianID.Valid {
			x := uuid.UUID(r.TechnicianID.Bytes).String()
			techID = &x
		}
		opByMachine[r.MachineID] = &AdminCurrentOperator{
			SessionID:             sid,
			ActorType:             textFromPgtypeText(r.ActorType),
			TechnicianID:          techID,
			TechnicianDisplayName: pgTextStringPtr(r.TechnicianDisplayName),
			UserPrincipal:         pgTextStringPtr(r.UserPrincipal),
			SessionStartedAt:      tsPgtypeTimestamptzToRFC3339Nano(r.SessionStartedAt),
			SessionStatus:         textFromPgtypeText(r.SessionStatus),
			SessionExpiresAt:      pgTimestamptzRFC3339NanoString(r.SessionExpiresAt),
		}
	}

	iRows, err := s.q.InventoryAdminSummarizeSlotsForMachines(ctx, machineIDs)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, r := range iRows {
		invByMachine[r.MachineID] = AdminMachineInventorySummary{
			TotalSlots:      r.TotalSlots,
			OccupiedSlots:   r.OccupiedSlots,
			LowStockSlots:   r.LowStockSlots,
			OutOfStockSlots: r.OutOfStockSlots,
		}
	}
	return techByMachine, opByMachine, invByMachine, nil
}

func (s *Service) assertSiteInCompany(ctx context.Context, scopeID, siteID uuid.UUID) error {
	_, err := s.q.GetSiteByID(ctx, siteID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return listscope.ErrInvalidListQuery
		}
		return err
	}
	if uuid.Nil != scopeID {
		return listscope.ErrInvalidListQuery
	}
	return nil
}

// ListMachines implements api.MachinesAdminService.
func (s *Service) ListMachines(ctx context.Context, scope listscope.AdminFleet) (*MachinesListResponse, error) {
	if s == nil || s.q == nil {
		return nil, errors.New("fleetadmin: nil service")
	}
	if scope.SiteID != nil {
		if err := s.assertSiteInCompany(ctx, uuid.Nil, *scope.SiteID); err != nil {
			return nil, err
		}
	}
	st, en := timeRangeOrAll(scope.From, scope.To)
	filterSite := scope.SiteID != nil && *scope.SiteID != uuid.Nil
	sid := uuid.Nil
	if filterSite {
		sid = *scope.SiteID
	}
	filterMachine := scope.MachineID != nil && *scope.MachineID != uuid.Nil
	mid := uuid.Nil
	if filterMachine {
		mid = *scope.MachineID
	}
	filterStatus := strings.TrimSpace(scope.Status) != ""

	listArg := db.FleetAdminListMachinesParams{
		Column1: filterSite,
		Column2: sid,
		Column3: filterMachine,
		Column4: mid,
		Column5: filterStatus,
		Column6: strings.TrimSpace(scope.Status),
		Column7: st,
		Column8: en,
		Limit:   scope.Limit,
		Offset:  scope.Offset,
	}
	countArg := db.FleetAdminCountMachinesParams{
		Column1: filterSite,
		Column2: sid,
		Column3: filterMachine,
		Column4: mid,
		Column5: filterStatus,
		Column6: strings.TrimSpace(scope.Status),
		Column7: st,
		Column8: en,
	}
	rows, err := s.q.FleetAdminListMachines(ctx, listArg)
	if err != nil {
		return nil, err
	}
	total, err := s.q.FleetAdminCountMachines(ctx, countArg)
	if err != nil {
		return nil, err
	}
	machineIDs := make([]uuid.UUID, 0, len(rows))
	for _, m := range rows {
		machineIDs = append(machineIDs, m.ID)
	}
	tech, op, inv, err := s.loadFleetEnrichment(ctx, uuid.Nil, machineIDs)
	if err != nil {
		return nil, err
	}
	items := make([]AdminMachineListItem, 0, len(rows))
	for _, m := range rows {
		item := baseItemFromFleetListRow(m)
		s.applyMachineEnrichment(&item, tech, op, inv, m.ID)
		items = append(items, item)
	}
	return &MachinesListResponse{
		Items: items,
		Meta: listscope.CollectionMeta{
			Limit:    scope.Limit,
			Offset:   scope.Offset,
			Returned: len(items),
			Total:    total,
		},
	}, nil
}

// GetMachine returns one fully enriched machine for GET /v1/admin/machines/{machineId}.
func (s *Service) GetMachine(ctx context.Context, companyID, machineID uuid.UUID) (*AdminMachineListItem, error) {
	if s == nil || s.q == nil {
		return nil, errors.New("fleetadmin: nil service")
	}
	row, err := s.q.FleetAdminGetMachineDetail(ctx, machineID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		return nil, err
	}
	item := baseItemFromFleetDetailRow(row)
	tech, op, inv, err := s.loadFleetEnrichment(ctx, companyID, []uuid.UUID{machineID})
	if err != nil {
		return nil, err
	}
	s.applyMachineEnrichment(&item, tech, op, inv, machineID)
	return &item, nil
}

// ListTechnicians implements api.TechniciansAdminService.
func (s *Service) ListTechnicians(ctx context.Context, scope listscope.AdminFleet) (*TechniciansListResponse, error) {
	if s == nil || s.q == nil {
		return nil, errors.New("fleetadmin: nil service")
	}
	st, en := timeRangeOrAll(scope.From, scope.To)
	filterTech := scope.TechnicianID != nil && *scope.TechnicianID != uuid.Nil
	tid := uuid.Nil
	if filterTech {
		tid = *scope.TechnicianID
	}
	search := strings.TrimSpace(scope.Search)
	filterSearch := search != "" // technicians list: display_name / email contains

	listArg := db.FleetAdminListTechniciansParams{
		Column1: filterTech,
		Column2: tid,
		Column3: filterSearch,
		Column4: search,
		Column5: st,
		Column6: en,
		Limit:   scope.Limit,
		Offset:  scope.Offset,
	}
	countArg := db.FleetAdminCountTechniciansParams{
		Column1: filterTech,
		Column2: tid,
		Column3: filterSearch,
		Column4: search,
		Column5: st,
		Column6: en,
	}
	rows, err := s.q.FleetAdminListTechnicians(ctx, listArg)
	if err != nil {
		return nil, err
	}
	total, err := s.q.FleetAdminCountTechnicians(ctx, countArg)
	if err != nil {
		return nil, err
	}
	items := make([]AdminTechnicianListItem, 0, len(rows))
	for _, t := range rows {
		items = append(items, AdminTechnicianListItem{
			TechnicianID:    t.ID.String(),
			DisplayName:     t.DisplayName,
			Email:           pgTextStringPtr(t.Email),
			Phone:           pgTextStringPtr(t.Phone),
			ExternalSubject: pgTextStringPtr(t.ExternalSubject),
			Status:          t.Status,
			CreatedAt:       t.CreatedAt.UTC(),
		})
	}
	return &TechniciansListResponse{
		Items: items,
		Meta: listscope.CollectionMeta{
			Limit:    scope.Limit,
			Offset:   scope.Offset,
			Returned: len(items),
			Total:    total,
		},
	}, nil
}

// ListAssignments implements api.AssignmentsAdminService.
func (s *Service) ListAssignments(ctx context.Context, scope listscope.AdminFleet) (*AssignmentsListResponse, error) {
	if s == nil || s.q == nil {
		return nil, errors.New("fleetadmin: nil service")
	}
	st, en := timeRangeOrAll(scope.From, scope.To)
	filterTech := scope.TechnicianID != nil && *scope.TechnicianID != uuid.Nil
	tid := uuid.Nil
	if filterTech {
		tid = *scope.TechnicianID
	}
	filterMachine := scope.MachineID != nil && *scope.MachineID != uuid.Nil
	mid := uuid.Nil
	if filterMachine {
		mid = *scope.MachineID
	}
	listArg := db.FleetAdminListAssignmentsParams{
		Column1: filterTech,
		Column2: tid,
		Column3: filterMachine,
		Column4: mid,
		Column5: st,
		Column6: en,
		Limit:   scope.Limit,
		Offset:  scope.Offset,
	}
	countArg := db.FleetAdminCountAssignmentsParams{
		Column1: filterTech,
		Column2: tid,
		Column3: filterMachine,
		Column4: mid,
		Column5: st,
		Column6: en,
	}
	rows, err := s.q.FleetAdminListAssignments(ctx, listArg)
	if err != nil {
		return nil, err
	}
	total, err := s.q.FleetAdminCountAssignments(ctx, countArg)
	if err != nil {
		return nil, err
	}
	items := make([]AdminAssignmentListItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, AdminAssignmentListItem{
			AssignmentID:          r.AssignmentID.String(),
			TechnicianID:          r.TechnicianID.String(),
			TechnicianDisplayName: r.TechnicianDisplayName,
			MachineID:             r.MachineID.String(),
			MachineName:           r.MachineName,
			MachineSerialNumber:   r.MachineSerialNumber,
			Role:                  r.Role,
			ValidFrom:             r.ValidFrom.UTC(),
			ValidTo:               pgTimestamptzTimePtr(r.ValidTo),
			CreatedAt:             r.CreatedAt.UTC(),
		})
	}
	return &AssignmentsListResponse{
		Items: items,
		Meta: listscope.CollectionMeta{
			Limit:    scope.Limit,
			Offset:   scope.Offset,
			Returned: len(items),
			Total:    total,
		},
	}, nil
}

// ListCommands implements api.CommandsAdminService.
func (s *Service) ListCommands(ctx context.Context, scope listscope.AdminFleet) (*CommandsListResponse, error) {
	if s == nil || s.q == nil {
		return nil, errors.New("fleetadmin: nil service")
	}
	st, en := timeRangeOrAll(scope.From, scope.To)
	filterMachine := scope.MachineID != nil && *scope.MachineID != uuid.Nil
	mid := uuid.Nil
	if filterMachine {
		mid = *scope.MachineID
	}
	filterStatus := strings.TrimSpace(scope.Status) != ""

	listArg := db.FleetAdminListCommandsParams{
		Column1: filterMachine,
		Column2: mid,
		Column3: filterStatus,
		Column4: strings.TrimSpace(scope.Status),
		Column5: st,
		Column6: en,
		Limit:   scope.Limit,
		Offset:  scope.Offset,
	}
	countArg := db.FleetAdminCountCommandsParams{
		Column1: filterMachine,
		Column2: mid,
		Column3: filterStatus,
		Column4: strings.TrimSpace(scope.Status),
		Column5: st,
		Column6: en,
	}
	rows, err := s.q.FleetAdminListCommands(ctx, listArg)
	if err != nil {
		return nil, err
	}
	total, err := s.q.FleetAdminCountCommands(ctx, countArg)
	if err != nil {
		return nil, err
	}
	items := make([]AdminCommandListItem, 0, len(rows))
	for _, r := range rows {
		st := strings.TrimSpace(r.LatestAttemptStatus)
		items = append(items, AdminCommandListItem{
			CommandID:           r.CommandID.String(),
			MachineID:           r.MachineID.String(),
			MachineName:         r.MachineName,
			MachineSerialNumber: r.MachineSerialNumber,
			Sequence:            r.Sequence,
			CommandType:         r.CommandType,
			CreatedAt:           r.CreatedAt.UTC(),
			AttemptCount:        r.AttemptCount,
			LatestAttemptStatus: st,
			CorrelationID:       pgUUIDStringPtr(r.CorrelationID),
		})
	}
	return &CommandsListResponse{
		Items: items,
		Meta: listscope.CollectionMeta{
			Limit:    scope.Limit,
			Offset:   scope.Offset,
			Returned: len(items),
			Total:    total,
		},
	}, nil
}
