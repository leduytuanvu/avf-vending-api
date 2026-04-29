// Package clickhouse provides an optional HTTP client for analytics-style cold paths.
// PostgreSQL remains the operational source of truth; nothing here participates in OLTP correctness.
//
// Behavior:
//
//   - [Open] with cfg.Enabled false returns [NewNoopClient] (default; no network I/O).
//   - [Open] with cfg.Enabled true builds an HTTP client to ClickHouse (:8123), runs [Client.Ping],
//     and supports [Client.InsertJSONEachRow] (JSONEachRow).
//   - [NewAsyncOutboxMirrorSink] is a bounded, best-effort mirror of successfully marked outbox
//     publishes; failures are retried up to a limit, counted in Prometheus, and logged. Enqueue is
//     non-blocking and may drop under backpressure.
//   - [NewAsyncProjectionSink] maps those same published outbox rows into typed sales/payment/vend/
//     inventory/telemetry/command analytics rows. It is also best-effort and never participates in
//     source transactions.
//
// Wiring lives in cmd/worker when ANALYTICS_* env vars enable the path; other binaries do not
// require ClickHouse. See ops/ANALYTICS_CLICKHOUSE.md for DDL and operations notes.
package clickhouse
