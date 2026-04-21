package fleetadmin

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/listscope"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
		MachineID:             m.ID.String(),
		MachineName:           m.Name,
		OrganizationID:        m.OrganizationID.String(),
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
	}
}

func baseItemFromFleetDetailRow(m db.FleetAdminGetMachineDetailRow) AdminMachineListItem {
	return AdminMachineListItem{
		MachineID:             m.ID.String(),
		MachineName:           m.Name,
		OrganizationID:        m.OrganizationID.String(),
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
	}
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

func (s *Service) loadFleetEnrichment(ctx context.Context, orgID uuid.UUID, machineIDs []uuid.UUID) (
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

	aRows, err := s.q.FleetAdminListActiveTechnicianAssignmentsForMachines(ctx, db.FleetAdminListActiveTechnicianAssignmentsForMachinesParams{
		OrganizationID: orgID,
		Column2:        machineIDs,
	})
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

	oRows, err := s.q.FleetAdminListViewOperatorsForMachines(ctx, db.FleetAdminListViewOperatorsForMachinesParams{
		OrganizationID: orgID,
		Column2:        machineIDs,
	})
	if err != nil {
		return nil, nil, nil, err
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

func (s *Service) assertSiteInOrganization(ctx context.Context, orgID, siteID uuid.UUID) error {
	site, err := s.q.GetSiteByID(ctx, siteID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return listscope.ErrInvalidListQuery
		}
		return err
	}
	if site.OrganizationID != orgID {
		return listscope.ErrInvalidListQuery
	}
	return nil
}

// ListMachines implements api.MachinesAdminService.
func (s *Service) ListMachines(ctx context.Context, scope listscope.AdminFleet) (*MachinesListResponse, error) {
	if s == nil || s.q == nil {
		return nil, errors.New("fleetadmin: nil service")
	}
	if scope.OrganizationID == uuid.Nil {
		return nil, listscope.ErrAdminOrganizationRequired
	}
	if scope.SiteID != nil {
		if err := s.assertSiteInOrganization(ctx, scope.OrganizationID, *scope.SiteID); err != nil {
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
		OrganizationID: scope.OrganizationID,
		Column2:        filterSite,
		Column3:        sid,
		Column4:        filterMachine,
		Column5:        mid,
		Column6:        filterStatus,
		Column7:        strings.TrimSpace(scope.Status),
		Column8:        st,
		Column9:        en,
		Limit:          scope.Limit,
		Offset:         scope.Offset,
	}
	countArg := db.FleetAdminCountMachinesParams{
		OrganizationID: scope.OrganizationID,
		Column2:        filterSite,
		Column3:        sid,
		Column4:        filterMachine,
		Column5:        mid,
		Column6:        filterStatus,
		Column7:        strings.TrimSpace(scope.Status),
		Column8:        st,
		Column9:        en,
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
	tech, op, inv, err := s.loadFleetEnrichment(ctx, scope.OrganizationID, machineIDs)
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
func (s *Service) GetMachine(ctx context.Context, organizationID, machineID uuid.UUID) (*AdminMachineListItem, error) {
	if s == nil || s.q == nil {
		return nil, errors.New("fleetadmin: nil service")
	}
	if organizationID == uuid.Nil {
		return nil, listscope.ErrAdminOrganizationRequired
	}
	row, err := s.q.FleetAdminGetMachineDetail(ctx, db.FleetAdminGetMachineDetailParams{
		ID:             machineID,
		OrganizationID: organizationID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		return nil, err
	}
	item := baseItemFromFleetDetailRow(row)
	tech, op, inv, err := s.loadFleetEnrichment(ctx, organizationID, []uuid.UUID{machineID})
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
	if scope.OrganizationID == uuid.Nil {
		return nil, listscope.ErrAdminOrganizationRequired
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
		OrganizationID: scope.OrganizationID,
		Column2:        filterTech,
		Column3:        tid,
		Column4:        filterSearch,
		Column5:        search,
		Column6:        st,
		Column7:        en,
		Limit:          scope.Limit,
		Offset:         scope.Offset,
	}
	countArg := db.FleetAdminCountTechniciansParams{
		OrganizationID: scope.OrganizationID,
		Column2:        filterTech,
		Column3:        tid,
		Column4:        filterSearch,
		Column5:        search,
		Column6:        st,
		Column7:        en,
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
			OrganizationID:  t.OrganizationID.String(),
			DisplayName:     t.DisplayName,
			Email:           pgTextStringPtr(t.Email),
			Phone:           pgTextStringPtr(t.Phone),
			ExternalSubject: pgTextStringPtr(t.ExternalSubject),
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
	if scope.OrganizationID == uuid.Nil {
		return nil, listscope.ErrAdminOrganizationRequired
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
		OrganizationID: scope.OrganizationID,
		Column2:        filterTech,
		Column3:        tid,
		Column4:        filterMachine,
		Column5:        mid,
		Column6:        st,
		Column7:        en,
		Limit:          scope.Limit,
		Offset:         scope.Offset,
	}
	countArg := db.FleetAdminCountAssignmentsParams{
		OrganizationID: scope.OrganizationID,
		Column2:        filterTech,
		Column3:        tid,
		Column4:        filterMachine,
		Column5:        mid,
		Column6:        st,
		Column7:        en,
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
	if scope.OrganizationID == uuid.Nil {
		return nil, listscope.ErrAdminOrganizationRequired
	}
	st, en := timeRangeOrAll(scope.From, scope.To)
	filterMachine := scope.MachineID != nil && *scope.MachineID != uuid.Nil
	mid := uuid.Nil
	if filterMachine {
		mid = *scope.MachineID
	}
	filterStatus := strings.TrimSpace(scope.Status) != ""

	listArg := db.FleetAdminListCommandsParams{
		OrganizationID: scope.OrganizationID,
		Column2:        filterMachine,
		Column3:        mid,
		Column4:        filterStatus,
		Column5:        strings.TrimSpace(scope.Status),
		Column6:        st,
		Column7:        en,
		Limit:          scope.Limit,
		Offset:         scope.Offset,
	}
	countArg := db.FleetAdminCountCommandsParams{
		OrganizationID: scope.OrganizationID,
		Column2:        filterMachine,
		Column3:        mid,
		Column4:        filterStatus,
		Column5:        strings.TrimSpace(scope.Status),
		Column6:        st,
		Column7:        en,
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
			OrganizationID:      r.OrganizationID.String(),
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

// ListOTA implements api.OTAAdminService.
func (s *Service) ListOTA(ctx context.Context, scope listscope.AdminFleet) (*OTAListResponse, error) {
	if s == nil || s.q == nil {
		return nil, errors.New("fleetadmin: nil service")
	}
	if scope.OrganizationID == uuid.Nil {
		return nil, listscope.ErrAdminOrganizationRequired
	}
	st, en := timeRangeOrAll(scope.From, scope.To)
	filterStatus := strings.TrimSpace(scope.Status) != ""

	listArg := db.FleetAdminListOTACampaignsParams{
		OrganizationID: scope.OrganizationID,
		Column2:        filterStatus,
		Column3:        strings.TrimSpace(scope.Status),
		Column4:        st,
		Column5:        en,
		Limit:          scope.Limit,
		Offset:         scope.Offset,
	}
	countArg := db.FleetAdminCountOTACampaignsParams{
		OrganizationID: scope.OrganizationID,
		Column2:        filterStatus,
		Column3:        strings.TrimSpace(scope.Status),
		Column4:        st,
		Column5:        en,
	}
	rows, err := s.q.FleetAdminListOTACampaigns(ctx, listArg)
	if err != nil {
		return nil, err
	}
	total, err := s.q.FleetAdminCountOTACampaigns(ctx, countArg)
	if err != nil {
		return nil, err
	}
	items := make([]AdminOTAListItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, AdminOTAListItem{
			CampaignID:         r.CampaignID.String(),
			OrganizationID:     r.OrganizationID.String(),
			CampaignName:       r.CampaignName,
			Strategy:           r.Strategy,
			CampaignStatus:     r.CampaignStatus,
			CreatedAt:          r.CreatedAt.UTC(),
			ArtifactID:         r.ArtifactID.String(),
			ArtifactSemver:     pgTextStringPtr(r.ArtifactSemver),
			ArtifactStorageKey: r.ArtifactStorageKey,
		})
	}
	return &OTAListResponse{
		Items: items,
		Meta: listscope.CollectionMeta{
			Limit:    scope.Limit,
			Offset:   scope.Offset,
			Returned: len(items),
			Total:    total,
		},
	}, nil
}
