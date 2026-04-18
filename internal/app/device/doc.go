// Package device hosts transport-agnostic orchestration for machine shadow documents and command ledger flows.
//
// Publishing commands still flows through domaindevice.CommandShadowWorkflow (Postgres-backed in
// internal/modules/postgres). Shadow reads, reported-state writes, latest-command reads, and machine
// presence are behind small ports in this package so MQTT/HTTP stay thin.
//
// TODO: Wire ports in internal/modules/postgres — GetMachineShadow (exists), Upsert reported_state,
// GetLatestCommandByMachine (new sqlc queries as needed), and map machines.updated_at for presence.
package device
