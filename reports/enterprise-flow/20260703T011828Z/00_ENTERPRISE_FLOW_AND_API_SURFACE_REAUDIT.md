# Enterprise Flow and API Surface Re-Audit

**Timestamp:** 20260703T011828Z  
**Git SHA (baseline):** f191db94cdcbabe71eb956419fac3ac3a76a1f10  
**Repository:** avf-vending-api

## A. Enterprise Flow Audit

| # | Question | Answer |
|---|----------|--------|
| A1 | Lifecycle statuses for `machines.status` | `draft`, `provisioned`, `provisioning`, `active`, `online`, `offline`, `maintenance`, `suspended`, `retired`, `decommissioned`, `compromised` |
| A2 | Lifecycle routes | POST suspend/resume, archive/retire, mark-compromised, rotate/revoke credentials, revoke-sessions, transfer-site — `admin_fleet_write_http.go` |
| A3 | User JWT/RBAC | All `/v1/admin/*` require User JWT; destructive lifecycle requires `RequireFleetMachineLifecycle` |
| A4 | Machine JWT on admin lifecycle | **Blocked** via `RequireDenyMachinePrincipal` |
| A5 | Activation claim records who activated | **No** — device fingerprint hash only |
| A6 | Activation claim records operator_session_id | **No** |
| A7 | request_id/correlation_id/app_version/boot_id/reason on claims | **No** |
| A8 | machine_sessions vs machine_runtime_refresh_tokens | **machine_sessions canonical**; refresh_tokens table schema-only |
| A9 | Machine JWT credential_version + session validation | **Yes** — `machine_token_auth.go` |
| A10 | Token refresh rotates refresh tokens | **Yes** |
| A11 | Token refresh revokes old tokens | **Yes** (60s grace replay) |
| A12 | Lifecycle requires reason | **No** |
| A13 | Lifecycle audit/attribution | **Partial audit only**; no `machine_action_attributions` |
| A14 | Operator ended_reason stable semantics | **Partial** — free text; 2 domain constants |
| A15 | Technician actions create attributions | **Partial** — inventory/cash/workflows yes; lifecycle no |
| A16 | Reattach API after reinstall | **Missing** |
| A17 | Admin runtime session current/history API | **Missing** |
| A18 | Unified ops-overview/timeline | **Partial** — timeline exists (commands/commerce/check-ins); no ops-overview |
| A19 | Offline replay idempotent | **Yes** |
| A20 | Offline event types | **Yes** with different names (`commerce.*`, `inventory.*`, `telemetry.*`) |
| A21 | Cash-only offline gaps | App-side policy; backend needs aliases, reattach, attribution, lifecycle reason |

## B. REST Surface Audit

| # | Question | Answer |
|---|----------|--------|
| B22 | OpenAPI path count | **268** |
| B23 | OpenAPI operation count | **329** |
| B24 | Baseline match | **Yes** — 268/329 unchanged |
| B25 | Mounted missing OpenAPI | 10 routes: payment providers/ops (9) + media alias (1) |
| B26 | OpenAPI missing router | Enterprise planogram routes documented but unwired via `mountAdminCompanyFleetRoutes` |
| B27 | Deprecated/legacy/internal | Legacy machine REST flag-gated; ops listener outside OpenAPI |
| B28 | REST APIs to add | reattach-device, runtime-sessions (current/history/revoke), ops-overview; extend timeline |
| B29 | REST APIs to harden | All lifecycle POST routes (reason, attribution, standard response) |
| B30 | Extra/duplicated | rotate-credential/credentials/token-version aliases; disable/suspend aliases |

## C. gRPC Surface Audit

| # | Question | Answer |
|---|----------|--------|
| C31 | Proto service count | **23** |
| C32 | RPC method count | **88** |
| C33 | Baseline match | **Yes** — 23/88 |
| C34 | Proto RPCs not registered | 3 legacy `avf.v1.*` (superseded by `avf.internal.v1`) |
| C35 | Server methods without proto | None found |
| C36 | Public/no Bearer RPCs | ClaimActivation, RefreshMachineToken, ActivateMachine facade |
| C37 | Machine JWT RPCs | All other `avf.machine.v1.*` |
| C38 | Internal/admin User JWT RPCs | All `avf.internal.v1.*` |
| C39 | gRPC changes needed | Offline event type aliases; document 7 intentionally unimplemented RPCs |

## D. MQTT Surface Audit

| # | Question | Answer |
|---|----------|--------|
| D40 | Production topic layout | Enterprise: `{prefix}/machines/{machineId}/{rel}` |
| D41 | Enterprise device publish topics | 12 rel paths in `EnterpriseDevicePublishRelPaths` |
| D42 | Enterprise device subscribe topics | `{prefix}/machines/{machineId}/commands` |
| D43 | Backend command publish | Enterprise: `…/commands`; Legacy: `…/commands/dispatch` |
| D44 | Legacy opt-in | `PRODUCTION_ALLOW_LEGACY_MQTT_TOPIC_LAYOUT=true` required in production |
| D45 | Docs topics not in code | None critical |
| D46 | Code topics missing docs | Enterprise ingest patterns table; commands/receipt row; shadow/desired note |
| D47 | MQTT changes needed | Doc sync + test fix for commands/receipt |

## E. Final Audit

| # | Item | Plan |
|---|------|------|
| E48 | Schema migrations | `00016_enterprise_flow_accountability.sql` |
| E49 | REST routes required | reattach, runtime-sessions, ops-overview; lifecycle hardening |
| E50 | gRPC/proto changes | Offline aliases; doc updates |
| E51 | MQTT changes | Doc sync only |
| E52 | Tests | HTTP, gRPC, MQTT, security, surface validators |
| E53 | Compatibility risks | Required `reason` on lifecycle POST; timeline additive fields |
| E54 | Implementation plan | See `01_ENTERPRISE_FLOW_API_SURFACE_IMPLEMENTATION_PLAN.md` |
