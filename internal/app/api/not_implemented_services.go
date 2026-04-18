package api

import "context"

// unimplementedV1AdminCollections backs /v1/admin list routes until real repositories are wired.
// Each method returns *CapabilityError so httpserver maps responses to HTTP 501 (not_implemented).
type unimplementedV1AdminCollections struct{}

func (unimplementedV1AdminCollections) ListTechnicians(_ context.Context, _ AdminListScope) (*ListView, error) {
	return nil, &CapabilityError{
		Capability: "v1.admin.technicians.list",
		Message:    "technician directory listing is not implemented for this API revision",
	}
}

func (unimplementedV1AdminCollections) ListAssignments(_ context.Context, _ AdminListScope) (*ListView, error) {
	return nil, &CapabilityError{
		Capability: "v1.admin.assignments.list",
		Message:    "technician assignment listing is not implemented for this API revision",
	}
}

func (unimplementedV1AdminCollections) ListCommands(_ context.Context, _ AdminListScope) (*ListView, error) {
	return nil, &CapabilityError{
		Capability: "v1.admin.commands.list",
		Message:    "command listing is not implemented for this API revision",
	}
}

func (unimplementedV1AdminCollections) ListOTA(_ context.Context, _ AdminListScope) (*ListView, error) {
	return nil, &CapabilityError{
		Capability: "v1.admin.ota.list",
		Message:    "OTA campaign listing is not implemented for this API revision",
	}
}

// unimplementedV1CommerceLists backs /v1/payments and /v1/orders list routes until org-scoped listing exists.
// Each method returns *CapabilityError → HTTP 501 via writeV1ListError.
type unimplementedV1CommerceLists struct{}

func (unimplementedV1CommerceLists) ListPayments(_ context.Context, _ TenantCommerceScope) (*ListView, error) {
	return nil, &CapabilityError{
		Capability: "v1.payments.org_list",
		Message:    "organization-scoped payment listing is not implemented for this API revision",
	}
}

func (unimplementedV1CommerceLists) ListOrders(_ context.Context, _ TenantCommerceScope) (*ListView, error) {
	return nil, &CapabilityError{
		Capability: "v1.orders.org_list",
		Message:    "organization-scoped order listing is not implemented for this API revision",
	}
}
