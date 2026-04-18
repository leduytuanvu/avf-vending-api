package postgres

import (
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/domain/fleet"
	domainoperator "github.com/avf/avf-vending-api/internal/domain/operator"
	"github.com/avf/avf-vending-api/internal/domain/org"
	"github.com/avf/avf-vending-api/internal/domain/reliability"
	"github.com/avf/avf-vending-api/internal/domain/retail"
	"github.com/avf/avf-vending-api/internal/gen/db"
)

func mapOrganization(row db.Organization) org.Organization {
	return org.Organization{
		ID:        row.ID,
		Name:      row.Name,
		Slug:      row.Slug,
		Status:    row.Status,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

func mapSite(row db.Site) org.Site {
	return org.Site{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		RegionID:       row.RegionID,
		Name:           row.Name,
		CreatedAt:      row.CreatedAt,
	}
}

func mapMachine(row db.Machine) fleet.Machine {
	return fleet.Machine{
		ID:                row.ID,
		OrganizationID:    row.OrganizationID,
		SiteID:            row.SiteID,
		HardwareProfileID: row.HardwareProfileID,
		SerialNumber:      row.SerialNumber,
		Name:              row.Name,
		Status:            row.Status,
		CommandSequence:   row.CommandSequence,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}
}

func mapTechnician(row db.Technician) fleet.Technician {
	return fleet.Technician{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		DisplayName:    row.DisplayName,
		CreatedAt:      row.CreatedAt,
	}
}

func mapProduct(row db.Product) retail.Product {
	return retail.Product{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		Sku:            row.Sku,
		Name:           row.Name,
		Description:    row.Description,
		Active:         row.Active,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
}

func mapAuditLog(row db.AuditLog) compliance.AuditLog {
	return compliance.AuditLog{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		ActorType:      row.ActorType,
		ActorID:        row.ActorID,
		Action:         row.Action,
		ResourceType:   row.ResourceType,
		ResourceID:     row.ResourceID,
		Payload:        row.Payload,
		IP:             row.Ip,
		CreatedAt:      row.CreatedAt,
	}
}

func mapReliabilityOutbox(row db.OutboxEvent) reliability.OutboxEvent {
	return reliability.OutboxEvent{
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

func mapOperatorSession(row db.MachineOperatorSession) domainoperator.Session {
	return domainoperator.Session{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		MachineID:      row.MachineID,
		ActorType:      row.ActorType,
		TechnicianID:   row.TechnicianID,
		UserPrincipal:  row.UserPrincipal,
		Status:         row.Status,
		StartedAt:      row.StartedAt,
		EndedAt:        row.EndedAt,
		ExpiresAt:      row.ExpiresAt,
		ClientMetadata: row.ClientMetadata,
		LastActivityAt: row.LastActivityAt,
		EndedReason:    row.EndedReason,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
}

func mapOperatorAuthEvent(row db.MachineOperatorAuthEvent) domainoperator.AuthEvent {
	return domainoperator.AuthEvent{
		ID:                row.ID,
		OperatorSessionID: row.OperatorSessionID,
		MachineID:         row.MachineID,
		EventType:         row.EventType,
		AuthMethod:        row.AuthMethod,
		OccurredAt:        row.OccurredAt,
		CorrelationID:     row.CorrelationID,
		Metadata:          row.Metadata,
	}
}

func mapOperatorActionAttribution(row db.MachineActionAttribution) domainoperator.ActionAttribution {
	return domainoperator.ActionAttribution{
		ID:                row.ID,
		OperatorSessionID: row.OperatorSessionID,
		MachineID:         row.MachineID,
		ActionOriginType:  row.ActionOriginType,
		ResourceType:      row.ResourceType,
		ResourceID:        row.ResourceID,
		OccurredAt:        row.OccurredAt,
		Metadata:          row.Metadata,
		CorrelationID:     row.CorrelationID,
	}
}
