// Package featureflags evaluates org/site/machine scoped flags and rollouts.
//
// Kiosk-visible hints are typically returned on HTTP GET /v1/setup/machines/{machineId}/bootstrap
// (field runtimeHints when configured). There is no dedicated “flags only” RPC under avf.machine.v1;
// pair bootstrap hints with MachineCatalogService snapshot config_version / catalog_version refreshes.
package featureflags
