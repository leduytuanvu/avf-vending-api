package postgres

import (
	"time"

	"github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func mapOrder(row db.Order) commerce.Order {
	return commerce.Order{
		ID:                 row.ID,
		MachineID:          row.MachineID,
		Status:             row.Status,
		Currency:           row.Currency,
		SubtotalMinor:      row.SubtotalMinor,
		TaxMinor:           row.TaxMinor,
		TotalMinor:         row.TotalMinor,
		IdempotencyKey:     pgTextToStringPtr(row.IdempotencyKey),
		Simulated:          row.Simulated,
		SimulationRunID:    pgTextToStringPtr(row.SimulationRunID),
		SimulationScenario: pgTextToStringPtr(row.SimulationScenario),
		FakeBill:           row.FakeBill,
		FakeBoard:          row.FakeBoard,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
	}
}

func mapVend(row db.VendSession) commerce.VendSession {
	return mapVendFields(
		row.ID, row.OrderID, row.MachineID, row.ProductID, row.SlotIndex, row.State,
		row.FinalCommandAttemptID, row.Simulated, row.SimulationRunID, row.SimulationScenario, row.CreatedAt,
	)
}

func mapVendLockRow(row db.LockVendSessionByOrderAndSlotForUpdateRow) commerce.VendSession {
	return mapVendFields(
		row.ID, row.OrderID, row.MachineID, row.ProductID, row.SlotIndex, row.State,
		row.FinalCommandAttemptID, row.Simulated, row.SimulationRunID, row.SimulationScenario, row.CreatedAt,
	)
}

func mapVendUpdateRow(row db.UpdateVendSessionStateByOrderSlotRow) commerce.VendSession {
	return mapVendFields(
		row.ID, row.OrderID, row.MachineID, row.ProductID, row.SlotIndex, row.State,
		row.FinalCommandAttemptID, row.Simulated, row.SimulationRunID, row.SimulationScenario, row.CreatedAt,
	)
}

func mapVendGetRow(row db.GetVendSessionByOrderAndSlotRow) commerce.VendSession {
	return mapVendFields(
		row.ID, row.OrderID, row.MachineID, row.ProductID, row.SlotIndex, row.State,
		row.FinalCommandAttemptID, row.Simulated, row.SimulationRunID, row.SimulationScenario, row.CreatedAt,
	)
}

func mapVendLineSequenceRow(row db.GetVendSessionByOrderAndLineSequenceRow) commerce.VendSession {
	return mapVendFields(
		row.ID, row.OrderID, row.MachineID, row.ProductID, row.SlotIndex, row.State,
		row.FinalCommandAttemptID, row.Simulated, row.SimulationRunID, row.SimulationScenario, row.CreatedAt,
	)
}

func mapVendLineSequenceUpdateRow(row db.UpdateVendSessionStateByOrderLineSequenceRow) commerce.VendSession {
	return mapVendFields(
		row.ID, row.OrderID, row.MachineID, row.ProductID, row.SlotIndex, row.State,
		row.FinalCommandAttemptID, row.Simulated, row.SimulationRunID, row.SimulationScenario, row.CreatedAt,
	)
}

func mapVendFirstRow(row db.GetFirstVendSessionByOrderRow) commerce.VendSession {
	return mapVendFields(
		row.ID, row.OrderID, row.MachineID, row.ProductID, row.SlotIndex, row.State,
		row.FinalCommandAttemptID, row.Simulated, row.SimulationRunID, row.SimulationScenario, row.CreatedAt,
	)
}

func mapVendInsertRow(row db.InsertVendSessionRow) commerce.VendSession {
	return mapVendFields(
		row.ID, row.OrderID, row.MachineID, row.ProductID, row.SlotIndex, row.State,
		row.FinalCommandAttemptID, row.Simulated, row.SimulationRunID, row.SimulationScenario, row.CreatedAt,
	)
}

func mapVendFields(
	id, orderID, machineID, productID uuid.UUID,
	slotIndex int32,
	state string,
	finalCommandAttemptID pgtype.UUID,
	simulated bool,
	simulationRunID, simulationScenario pgtype.Text,
	createdAt time.Time,
) commerce.VendSession {
	return commerce.VendSession{
		ID:                    id,
		OrderID:               orderID,
		MachineID:             machineID,
		SlotIndex:             slotIndex,
		ProductID:             productID,
		State:                 state,
		FinalCommandAttemptID: pgUUIDToPtr(finalCommandAttemptID),
		Simulated:             simulated,
		SimulationRunID:       pgTextToStringPtr(simulationRunID),
		SimulationScenario:    pgTextToStringPtr(simulationScenario),
		CreatedAt:             createdAt,
	}
}

func mapVendFromStuckReconcileRow(row db.ListVendSessionsStuckForReconciliationRow) commerce.VendSession {
	return commerce.VendSession{
		ID:                    row.ID,
		OrderID:               row.OrderID,
		MachineID:             row.MachineID,
		SlotIndex:             row.SlotIndex,
		ProductID:             row.ProductID,
		State:                 row.State,
		FinalCommandAttemptID: pgUUIDToPtr(row.FinalCommandAttemptID),
		Simulated:             row.Simulated,
		SimulationRunID:       pgTextToStringPtr(row.SimulationRunID),
		SimulationScenario:    pgTextToStringPtr(row.SimulationScenario),
		CreatedAt:             row.CreatedAt,
	}
}

func mapPayment(row db.Payment) commerce.Payment {
	return commerce.Payment{
		ID:                   row.ID,
		OrderID:              row.OrderID,
		Provider:             row.Provider,
		State:                row.State,
		AmountMinor:          row.AmountMinor,
		Currency:             row.Currency,
		IdempotencyKey:       pgTextToStringPtr(row.IdempotencyKey),
		ReconciliationStatus: row.ReconciliationStatus,
		SettlementStatus:     row.SettlementStatus,
		SettlementBatchID:    pgUUIDToPtr(row.SettlementBatchID),
		Simulated:            row.Simulated,
		SimulationRunID:      pgTextToStringPtr(row.SimulationRunID),
		SimulationScenario:   pgTextToStringPtr(row.SimulationScenario),
		FakeBill:             row.FakeBill,
		FakeBoard:            row.FakeBoard,
		CreatedAt:            row.CreatedAt,
	}
}

func mapOutbox(row db.OutboxEvent) commerce.OutboxEvent {
	return commerce.OutboxEvent{
		ID:                   row.ID,
		Topic:                row.Topic,
		EventType:            row.EventType,
		Payload:              row.Payload,
		AggregateType:        row.AggregateType,
		AggregateID:          row.AggregateID,
		IdempotencyKey:       pgTextToStringPtr(row.IdempotencyKey),
		CreatedAt:            row.CreatedAt,
		PublishedAt:          pgTimestamptzToTimePtr(row.PublishedAt),
		PublishAttemptCount:  row.PublishAttemptCount,
		LastPublishError:     pgTextToStringPtr(row.LastPublishError),
		LastPublishAttemptAt: pgTimestamptzToTimePtr(row.LastPublishAttemptAt),
		NextPublishAfter:     pgTimestamptzToTimePtr(row.NextPublishAfter),
		DeadLetteredAt:       pgTimestamptzToTimePtr(row.DeadLetteredAt),
		Status:               row.Status,
		LockedBy:             pgTextToStringPtr(row.LockedBy),
		LockedUntil:          pgTimestamptzToTimePtr(row.LockedUntil),
		UpdatedAt:            row.UpdatedAt,
		MaxPublishAttempts:   row.MaxPublishAttempts,
		Simulated:            row.Simulated,
		SimulationRunID:      pgTextToStringPtr(row.SimulationRunID),
		SimulationScenario:   pgTextToStringPtr(row.SimulationScenario),
	}
}
