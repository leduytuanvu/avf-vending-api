// Package reports documents P2.1 admin BI and CSV export entrypoints.
//
// Reporting queries and CSV writers are implemented in [github.com/avf/avf-vending-api/internal/app/reporting].
// HTTP routes under GET /v1/admin/organizations/{organizationId}/reports/* are wired from [github.com/avf/avf-vending-api/internal/httpserver].
//
// See docs/api/reports.md for parameters, filters, and export semantics.
package reports
