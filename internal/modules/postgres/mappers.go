package postgres

import (
	"github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/gen/db"
)

func mapOrder(row db.Order) commerce.Order {
	return commerce.Order{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		MachineID:      row.MachineID,
		Status:         row.Status,
		Currency:       row.Currency,
		SubtotalMinor:  row.SubtotalMinor,
		TaxMinor:       row.TaxMinor,
		TotalMinor:     row.TotalMinor,
		IdempotencyKey: row.IdempotencyKey,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
}

func mapVend(row db.VendSession) commerce.VendSession {
	return commerce.VendSession{
		ID:                    row.ID,
		OrderID:               row.OrderID,
		MachineID:             row.MachineID,
		SlotIndex:             row.SlotIndex,
		ProductID:             row.ProductID,
		State:                 row.State,
		FinalCommandAttemptID: row.FinalCommandAttemptID,
		CreatedAt:             row.CreatedAt,
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
		FinalCommandAttemptID: row.FinalCommandAttemptID,
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
		IdempotencyKey:       row.IdempotencyKey,
		ReconciliationStatus: row.ReconciliationStatus,
		SettlementStatus:     row.SettlementStatus,
		SettlementBatchID:    row.SettlementBatchID,
		CreatedAt:            row.CreatedAt,
	}
}

func mapOutbox(row db.OutboxEvent) commerce.OutboxEvent {
	return commerce.OutboxEvent{
		ID:                   row.ID,
		OrganizationID:       row.OrganizationID,
		Topic:                row.Topic,
		EventType:            row.EventType,
		Payload:              row.Payload,
		AggregateType:        row.AggregateType,
		AggregateID:          row.AggregateID,
		IdempotencyKey:       row.IdempotencyKey,
		CreatedAt:            row.CreatedAt,
		PublishedAt:          row.PublishedAt,
		PublishAttemptCount:  row.PublishAttemptCount,
		LastPublishError:     row.LastPublishError,
		LastPublishAttemptAt: row.LastPublishAttemptAt,
		NextPublishAfter:     row.NextPublishAfter,
		DeadLetteredAt:       row.DeadLetteredAt,
	}
}
