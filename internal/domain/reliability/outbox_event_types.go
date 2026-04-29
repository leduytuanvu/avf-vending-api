package reliability

// Canonical domain event_type strings for transactional outbox rows (topic may vary by deployment).
// Callers should insert outbox_events in the same DB transaction as the state change.
const (
	OutboxEventOrderCreated          = "order.created"
	OutboxEventPaymentConfirmed      = "payment.confirmed"
	OutboxEventPaymentFailed         = "payment.failed"
	OutboxEventVendSucceeded         = "vend.succeeded"
	OutboxEventVendFailed            = "vend.failed"
	OutboxEventInventoryAdjusted     = "inventory.adjusted"
	OutboxEventMachineCommandCreated = "machine.command.created"
	OutboxEventMachineCommandAcked   = "machine.command.acked"
	OutboxEventMachineActivated      = "machine.activated"
	OutboxEventMediaUpdated          = "media.updated"
	OutboxEventCatalogChanged        = "catalog.changed"
)
