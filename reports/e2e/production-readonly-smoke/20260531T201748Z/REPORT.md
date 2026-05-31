# Production E2E — production-readonly-smoke

- **SMOKE_VERDICT:** PASS
- **READINESS_VERDICT:** GO-CANARY-ONLY
- **Strict canary:** true
- **Exit code:** 0
- **Generated:** 2026-05-31T20:18:05.993949+00:00

| Probe | Outcome | Latency ms | Detail |
|---|---|---:|---|
| http/health/live | PASS | 317 | status=200 |
| http/health/ready | PASS | 436 | status=200 |
| http/version | PASS | 301 | status=200 |
| version.payment_runtime | PASS | 310 | payment_mode=cash_only |
| version.payment_runtime.cash_only_contract | PASS | 0 | cash-only contract ok |
| admin.auth | PASS | 0 | token acquired |
| admin/v1/admin/machines?limit=3 | PASS | 625 | status=200 |
| admin/v1/admin/payment/providers | PASS | 287 | status=200 |
| grpc.activation_dry_run | PASS | 703 | rejected invalid activation code (expected) |
| grpc.token_refresh | SKIP | 0 | MACHINE_REFRESH_TOKEN unset |
| grpc.bootstrap | PASS | 2810 | ok |
| grpc.bootstrap.mqtt_config | PASS | 0 | broker+prefix present |
| mqtt.config_match | PASS | 0 | bootstrap MQTT config present |
| mqtt.topic_layout | PASS | 0 | layout=enterprise |
| mqtt.tls_required | PASS | 0 | tls_required=true |
| mqtt.client_id_policy | PASS | 0 | policy=avf-machine-{machine_id} |
| grpc.bootstrap.payment_methods | PASS | 0 | {"cashEnabled":true,"paymentMode":"cash_only","cardQrProviderStatus":"unavailable","qrCardUnavailableReason":"provider_unavailable"} |
| grpc.catalog | PASS | 2760 | ok |
| grpc.media_manifest | PASS | 722 | ok |
| grpc.media_delta | PASS | 756 | ok |
| grpc.inventory | PASS | 667 | ok |
| grpc.planogram | PASS | 669 | ok |
| safety.no_create_order | PASS | 0 | not invoked |
| safety.no_payment_session | PASS | 0 | not invoked |
| safety.no_cash_confirm | PASS | 0 | not invoked |
| safety.no_start_vend | PASS | 0 | not invoked |
| safety.no_mqtt_command_publish | PASS | 0 | not invoked |
| grpc.telemetry_smoke | PASS | 1355 | test-machine heartbeat accepted |
