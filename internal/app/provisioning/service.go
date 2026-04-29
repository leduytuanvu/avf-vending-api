package provisioning

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	appactivation "github.com/avf/avf-vending-api/internal/app/activation"
	appaudit "github.com/avf/avf-vending-api/internal/app/audit"
	appfleet "github.com/avf/avf-vending-api/internal/app/fleet"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const maxBulkMachines = 1000

// Service wires bulk provisioning without hiding fleet validation rules.
type Service struct {
	pool       *pgxpool.Pool
	fleet      *appfleet.Service
	activation *appactivation.Service
	audit      *appaudit.Service
}

type Deps struct {
	Pool       *pgxpool.Pool
	Fleet      *appfleet.Service
	Activation *appactivation.Service
	Audit      *appaudit.Service
}

func NewService(d Deps) *Service {
	if d.Pool == nil || d.Fleet == nil || d.Activation == nil {
		return nil
	}
	return &Service{pool: d.Pool, fleet: d.Fleet, activation: d.Activation, audit: d.Audit}
}

// BulkMachineRow is one machine definition inside a bulk request.
type BulkMachineRow struct {
	SerialNumber string
	Name         string
	Model        string
}

// BulkCreateInput describes POST …/provisioning/machines/bulk.
type BulkCreateInput struct {
	SiteID                  uuid.UUID
	HardwareProfileID       *uuid.UUID
	CabinetType             string
	Machines                []BulkMachineRow
	GenerateActivationCodes bool
	ExpiresInMinutes        int32
	MaxUses                 int32
	CreatedBy               pgtype.UUID
}

// BulkCreatedMachine captures plaintext activation codes returned once at creation time.
type BulkCreatedMachine struct {
	MachineID        uuid.UUID `json:"machineId"`
	SerialNumber     string    `json:"serialNumber"`
	ActivationCode   string    `json:"activationCode,omitempty"`
	ActivationCodeID string    `json:"activationCodeId,omitempty"`
}

// BulkCreateResult returns batch identifiers plus optional plaintext activation codes.
type BulkCreateResult struct {
	BatchID      uuid.UUID            `json:"batchId"`
	Status       string               `json:"status"`
	Machines     []BulkCreatedMachine `json:"machines"`
	MachineCount int                  `json:"machineCount"`
}

// BulkCreateMachines inserts machines plus provisioning metadata rows.
func (s *Service) BulkCreateMachines(ctx context.Context, organizationID uuid.UUID, in BulkCreateInput) (BulkCreateResult, error) {
	if s == nil || s.pool == nil || s.fleet == nil || s.activation == nil {
		return BulkCreateResult{}, ErrInvalidArgument
	}
	if organizationID == uuid.Nil {
		return BulkCreateResult{}, ErrInvalidArgument
	}
	if in.SiteID == uuid.Nil {
		return BulkCreateResult{}, ErrInvalidArgument
	}
	n := len(in.Machines)
	if n == 0 || n > maxBulkMachines {
		return BulkCreateResult{}, ErrInvalidArgument
	}
	for _, m := range in.Machines {
		if strings.TrimSpace(m.SerialNumber) == "" {
			return BulkCreateResult{}, ErrInvalidArgument
		}
	}

	expMin := in.ExpiresInMinutes
	if expMin <= 0 {
		expMin = 1440
	}
	maxUses := in.MaxUses
	if maxUses <= 0 {
		maxUses = 1
	}

	q := db.New(s.pool)
	meta := map[string]any{
		"cabinet_template": strings.TrimSpace(in.CabinetType),
	}
	metaBytes, _ := json.Marshal(meta)

	var hw pgtype.UUID
	if in.HardwareProfileID != nil && *in.HardwareProfileID != uuid.Nil {
		hw = pgtype.UUID{Bytes: *in.HardwareProfileID, Valid: true}
	}

	batchRow, err := q.InsertProvisioningBatch(ctx, db.InsertProvisioningBatchParams{
		OrganizationID:    organizationID,
		SiteID:            in.SiteID,
		HardwareProfileID: hw,
		CabinetType:       strings.TrimSpace(in.CabinetType),
		Status:            "pending",
		MachineCount:      int32(n),
		Metadata:          metaBytes,
		CreatedBy:         in.CreatedBy,
	})
	if err != nil {
		return BulkCreateResult{}, err
	}

	out := BulkCreateResult{
		BatchID:  batchRow.ID,
		Status:   batchRow.Status,
		Machines: make([]BulkCreatedMachine, 0, n),
	}

	for i, row := range in.Machines {
		dm, cerr := s.fleet.CreateMachine(ctx, appfleet.CreateMachineInput{
			OrganizationID:    organizationID,
			SiteID:            in.SiteID,
			HardwareProfileID: in.HardwareProfileID,
			SerialNumber:      strings.TrimSpace(row.SerialNumber),
			Code:              "",
			Model:             strings.TrimSpace(row.Model),
			CabinetType:       strings.TrimSpace(in.CabinetType),
			Name:              strings.TrimSpace(row.Name),
			Status:            "provisioning",
		})
		if cerr != nil {
			_, _ = q.UpdateProvisioningBatchStatus(ctx, db.UpdateProvisioningBatchStatusParams{
				ID:             batchRow.ID,
				OrganizationID: organizationID,
				Status:         "failed",
				MachineCount:   int32(len(out.Machines)),
			})
			return BulkCreateResult{}, cerr
		}

		var actID pgtype.UUID
		item := BulkCreatedMachine{
			MachineID:    dm.ID,
			SerialNumber: strings.TrimSpace(row.SerialNumber),
		}

		if in.GenerateActivationCodes {
			ar, aerr := s.activation.CreateCode(ctx, appactivation.CreateInput{
				MachineID:        dm.ID,
				OrganizationID:   organizationID,
				ExpiresInMinutes: expMin,
				MaxUses:          maxUses,
				Notes:            fmt.Sprintf("bulk provisioning batch %s", batchRow.ID),
			})
			if aerr != nil {
				_, _ = q.UpdateProvisioningBatchStatus(ctx, db.UpdateProvisioningBatchStatusParams{
					ID:             batchRow.ID,
					OrganizationID: organizationID,
					Status:         "failed",
					MachineCount:   int32(len(out.Machines)),
				})
				return BulkCreateResult{}, aerr
			}
			actID = pgtype.UUID{Bytes: ar.ID, Valid: true}
			item.ActivationCode = ar.PlaintextCode
			item.ActivationCodeID = ar.ID.String()
		}

		_, ierr := q.InsertProvisioningBatchMachine(ctx, db.InsertProvisioningBatchMachineParams{
			BatchID:          batchRow.ID,
			OrganizationID:   organizationID,
			MachineID:        dm.ID,
			SerialNumber:     strings.TrimSpace(row.SerialNumber),
			ActivationCodeID: actID,
			RowNo:            int32(i),
		})
		if ierr != nil {
			_, _ = q.UpdateProvisioningBatchStatus(ctx, db.UpdateProvisioningBatchStatusParams{
				ID:             batchRow.ID,
				OrganizationID: organizationID,
				Status:         "failed",
				MachineCount:   int32(len(out.Machines)),
			})
			return BulkCreateResult{}, ierr
		}

		out.Machines = append(out.Machines, item)
	}

	final, err := q.UpdateProvisioningBatchStatus(ctx, db.UpdateProvisioningBatchStatusParams{
		ID:             batchRow.ID,
		OrganizationID: organizationID,
		Status:         "completed",
		MachineCount:   int32(len(out.Machines)),
	})
	if err != nil {
		return BulkCreateResult{}, err
	}
	out.Status = final.Status
	out.MachineCount = len(out.Machines)

	if s.audit != nil {
		md, _ := json.Marshal(map[string]any{"batch_id": batchRow.ID.String(), "machine_count": len(out.Machines)})
		rid := batchRow.ID.String()
		_ = s.audit.Record(ctx, compliance.EnterpriseAuditRecord{
			OrganizationID: organizationID,
			ActorType:      compliance.ActorUser,
			Action:         compliance.ActionFleetProvisioningBulkCreated,
			ResourceType:   "machine_provisioning_batches",
			ResourceID:     &rid,
			Metadata:       md,
		})
	}

	return out, nil
}

// GetBatchDetail returns provisioning metadata plus activation-code visibility for operators.
func (s *Service) GetBatchDetail(ctx context.Context, organizationID, batchID uuid.UUID) (db.MachineProvisioningBatch, []db.ListProvisioningBatchMachinesRow, error) {
	if s == nil || s.pool == nil {
		return db.MachineProvisioningBatch{}, nil, ErrInvalidArgument
	}
	q := db.New(s.pool)
	batch, err := q.GetProvisioningBatchByID(ctx, db.GetProvisioningBatchByIDParams{
		ID:             batchID,
		OrganizationID: organizationID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.MachineProvisioningBatch{}, nil, ErrNotFound
		}
		return db.MachineProvisioningBatch{}, nil, err
	}
	items, err := q.ListProvisioningBatchMachines(ctx, db.ListProvisioningBatchMachinesParams{
		BatchID:        batchID,
		OrganizationID: organizationID,
	})
	if err != nil {
		return db.MachineProvisioningBatch{}, nil, err
	}
	return batch, items, nil
}
