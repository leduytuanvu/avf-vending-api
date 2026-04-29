# Field test cases — production pilot and fleet rollout (10 → 1000 machines)

**Purpose:** Executable cases for **staging**, **pilots**, and **go-live evidence**. Each row is one **measurable** verification aligned with **[`production-final-contract.md`](../architecture/production-final-contract.md)** (Admin REST, machine gRPC, MQTT commands, backend-owned payments, object storage + kiosk cache, legacy machine HTTP **off** in production).

**Audiences:** Android team (gRPC + cache behavior), backend (API + webhooks + outbox), QA field (steps/expected), ops/on-call (reconciliation + rollback), release (evidence pack).

## Priority (do not conflate)

| Priority | Meaning |
| -------- | ------- |
| **P0** | Safety / tenancy / backup / rollback posture — blocker if failing. |
| **P1** | Pilot and normal widen tranche — required before expanding machine count. |
| **P2** | Fleet-scale extras — storm + monitoring per **[`production-release-readiness.md`](../runbooks/production-release-readiness.md)** when claiming **scale-100+**. |

## How to use (Google Sheets / Excel)

1. Copy the **Markdown table** below **or** paste the **TSV** block from [Appendix — TSV](#appendix--tsv-for-sheets) into cell **A1** (tab-separated). Columns are exactly: **Case ID | Priority | Setup | Steps | Expected | Actual | Pass/Fail | Evidence | Owner**.
2. Fill **Actual**, **Pass/Fail**, **Evidence** (ticket URL, workflow run, log artifact id — **no secrets**), **Owner** (name · role).
3. **Setup** must name environment (`staging` / `pilot-10` / `prod`), **machine id**, build/API version, and broker profile (TLS endpoint).

**Policy:** Production CI smoke remains **GET-only** per **[`production-smoke-tests.md`](../operations/production-smoke-tests.md)** — it does **not** satisfy **FT-PAY-***, **FT-VND-***, or **FT-MQT-*** mutating rows.

---

## Master matrix

| Case ID | Priority | Setup | Steps | Expected | Actual | Pass/Fail | Evidence | Owner |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| FT-ADM-01 | P1 | Staging or pilot. Admin user + non-admin user same org. Base URL + `POST /v1/auth/login` available. | (1) Admin: `POST /v1/auth/login` (body per OpenAPI). (2) `GET /v1/auth/me` with admin access token. (3) Non-admin token: attempt `POST /v1/admin/products` (or other write documented as admin-only in OpenAPI). (4) Admin: `GET /v1/admin/audit/events?limit=5` if enabled. | (1)–(2) **200**; **me** shows admin role. (3) **403** or **404** per policy; response body **no** cross-tenant data. (4) Audit returns **200** with recent events (or **404** if route disabled — record which). | | | | |
| FT-ADM-02 | P1 | Admin JWT. Org with catalog admin rights. | (1) `POST /v1/admin/products` (minimal valid body per OpenAPI). (2) `GET /v1/admin/products?search=` or get-by-id per OpenAPI. | **201/200** create; **GET** returns created SKU/product id; fields match payload. | | | | |
| FT-ADM-03 | P1 | Admin JWT. Object storage wired (staging/pilot). Product id from FT-ADM-02 or equivalent. | (1) Upload/attach media per admin OpenAPI (multipart or presign flow — follow **[`product-media-cache-invalidation.md`](../runbooks/product-media-cache-invalidation.md)** for your profile). (2) `GET` product detail — note image URLs + **`contentHash`** / variant metadata. | Media attached; **GET** shows **HTTPS** URLs or signed URLs with **`expires_at`**; **no** raw bucket secrets in JSON. | | | | |
| FT-MAC-01 | P1 | Pilot machine activated; **Machine JWT** on device. **gRPC** `grpcs://…` reachable. Legacy machine HTTP **off** in prod. | (1) `MachineAuthService/ClaimActivation` (if new device) **or** refresh token if already bound. (2) `MachineBootstrapService/GetBootstrap` with valid meta. (3) Optionally `CheckIn` / `AckConfigVersion` if your flow requires. | **200 OK** equivalent: bootstrap shows **machineId**, **organizationId**, topology/planogram hints per proto; **no** Admin JWT used on gRPC. | | | | |
| FT-CAT-01 | P1 | Machine JWT. Known `catalog_version` empty on first run. | (1) `MachineCatalogService/GetCatalogSnapshot` with flags your UI uses (`include_unavailable` documented). (2) Record **`catalog_version`**, **`generated_at`**, **`Meta.server_time`**. (3) `GetCatalogDelta` with **`basis_catalog_version`** from (2). (4) Compare slot count + one known SKU price to admin source of truth. | (1) Snapshot **OK** with non-empty **`catalog_version`**. (3) **`BasisMatches`** / **NOT_MODIFIED** when unchanged; when changed, delta or full snapshot consistent with admin pricing. (4) **No** stale price vs admin for sampled SKU. | | | | |
| FT-MED-02 | P1 | Completed FT-CAT-01 or FT-ADM-03. Kiosk has **local disk cache** enabled. | (1) From snapshot/manifest, pick **thumb** URL + **`checksum_sha256`**. (2) Download bytes (follow redirects if any). (3) Compute SHA-256 locally; compare. (4) Toggle airplane mode; confirm UI still renders **cached** image for same variant key. (5) Bump media server-side (or wait for `media_fingerprint` change); confirm app **invalidates** old file and re-fetches. | (3) Hash **matches** server metadata. (4) Offline UI uses **on-disk** cache (no silent placeholder). (5) After version bump, new file loads or UI marks stale per product policy. | | | | |
| FT-PAY-01 | P1 | Sandbox PSP or **approved** small-value live test. Webhook reachable from PSP test harness. Machine JWT. | (1) `CreateOrder` with line items from catalog (amounts **from server** pricing). (2) `CreatePaymentSession` with **same** totals as order; record **`qr_payload_or_url`** from response **only**. (3) Complete payment at PSP **without** client forging URL. (4) PSP webhook hits AVF with valid **HMAC** (see **[`payment-webhook-debug.md`](../runbooks/payment-webhook-debug.md)**). (5) Poll `GetOrder` / `GetOrderStatus` until **paid**. (6) Replay same webhook once. | Order reaches **paid** exactly once; duplicate webhook **no** second capture; ledger/timelines consistent. | | | | |
| FT-PAY-02 | P1 | Cash path enabled for test SKU; machine JWT. | (1) `CreateOrder` (cash). (2) `ConfirmCashPayment` / `CreateCashCheckout` + `ConfirmCashReceived` per **[`machine-grpc.md`](../api/machine-grpc.md)** naming. (3) Complete vend success path. (4) Retry (2) with **same** idempotency scope. | Cash order **completed**; **no** PSP session; duplicate confirm **replay-safe**. | | | | |
| FT-VND-01 | P1 | Order in **paid** / ready-to-vend state; hardware or sim can report success. | (1) `StartVend` with idempotency key **A**. (2) Hardware **success**. (3) `ConfirmVendSuccess` / `ReportVendSuccess` with idempotency key **B** per proto. (4) Repeat (3) once with **same** keys. | Inventory decrements **once**; order **completed**; duplicate success RPC is **replay** (no double decrement). | | | | |
| FT-PAY-03 | P1 | Same as FT-PAY-01 through **paid**; hardware/sim allows **forced vend failure**. | (1) `StartVend` after paid. (2) Simulate mechanical failure. (3) `ReportVendFailure` with stable **idempotency_key** (see **[`kiosk-app-flow.md`](../api/kiosk-app-flow.md)**). (4) Open admin/commerce timeline or `GetOrder`; (5) execute **refund** path per org policy (`POST` refund route or Temporal/admin — per OpenAPI). | Order shows **vend failed**; refund **initiated** or **completed** per PSP; **no** double refund on retry with same keys. | | | | |
| FT-OFF-01 | P1 | Machine JWT; **offline queue** enabled on app. | (1) Airplane mode **on**. (2) Perform **one** catalog-mutation-critical action **if** supported offline (e.g. queue telemetry or offline-tolerated event per contract); **or** queue `PushOfflineEvents` payload per **[`machine-grpc.md`](../api/machine-grpc.md)**. (3) Restore network; call `PushOfflineEvents` with monotonic **`offline_sequence`**. (4) Send **duplicate** same `client_event_id`. | (3) Server **accepts** ordered sequence; (4) duplicate returns **REPLAYED** / idempotent **no double posting**; gap in sequence returns **Aborted** with clear error. | | | | |
| FT-PAY-04 | P1 | Paid path in progress **or** session created; network controllable (Faraday / router ACL). | (1) Start PSP flow through `CreatePaymentSession`. (2) After user starts pay at PSP, **drop** all egress on device **except** nothing for 30–60s (simulate mid-flight). (3) Restore network. (4) Poll `GetOrder` / PSP dashboard. (5) Retry sale flow with **new** idempotency key only if order **cancelled** — document policy. | **No** orphan **paid** without matching server order state; on restore, UI shows **recoverable** state (paid → proceed vend, or stuck → operator path); **no** duplicate **paid** capture for one customer intent without ticket. | | | | |
| FT-VND-03 | P1 | Order in **vending**; power loss sim **or** hard kill app process mid-`StartVend`. | (1) `StartVend`. (2) **Cut power** to Android board **or** `adb shell am force-stop` during vend. (3) Cold boot; open app; read last local **`idempotency_key`** / order id from Room. (4) Call `GetOrder` / `GetOrderStatus`. (5) Complete or fail vend with **same** keys as playbook. | Server **no** phantom success; after reboot, **one** terminal vend outcome; inventory matches physical reality after reconciliation or operator adjust. | | | | |
| FT-MQT-01 | P1 | MQTT **TLS** broker; device subscribed per **[`mqtt-contract.md`](../api/mqtt-contract.md)**; command ledger enabled **staging/pilot** (not CI abusing prod). | (1) From **admin** path, dispatch command matching deployment (topic + correlation id). (2) Device receives payload; sends **application ACK** with matching **`command_id`**, **`machine_id`**, **`sequence`**. (3) Verify Postgres/command UI shows **acked**. | Command transitions **pending → acked** (or equivalent); timeline/ledger row present; **no** orphan pending > SLA. | | | | |
| FT-MQT-02 | P1 | Same as FT-MQT-01 after successful ACK. | (1) Re-send **identical** ACK envelope (same `dedupe_key` / correlation fields as first success). (2) Observe broker + server metrics/logs. | Second ACK **idempotent**; **no** duplicate side effect; duplicate logged or counted per metrics. | | | | |
| FT-TEL-01 | P1/P2 | Telemetry path configured (**`PushTelemetryBatch`** and/or MQTT ingest per deployment). | (1) Publish **normal** batch with valid `dedupe_key`. (2) Publish **critical** envelope. (3) Publish duplicate dedupe. (4) Run **`ReconcileEvents`** with batch of ids if in scope. | Counts and DB/mirror state show **no duplicate side effects**; reconcile returns **success** for processed ids. | | | | |
| FT-OBX-01 | P1/P2 | Worker + NATS + outbox **enabled**; non-prod load or **pilot** volume. | (1) Trigger event that enqueues **outbox** row (e.g. payment session started — per ops playbook). (2) Confirm JetStream/consumer receives publish. (3) Simulate **transient** downstream failure (if safe in staging). (4) Restore; verify row **marked published** or DLQ policy per **[`outbox-dlq-debug.md`](../runbooks/outbox-dlq-debug.md)**. | **No** stuck `pending` beyond SLO after recovery; DLQ **actionable** if configured; metrics **`avf_worker_outbox_*`** sane. | | | | |
| FT-PAY-05 | P1 | Paid orders from FT-PAY-01/03; reconciler or admin **read** paths available. | (1) `GET` order reconciliation endpoint per OpenAPI **or** admin commerce list. (2) Compare **3-way**: kiosk displayed total, server `total_minor`, PSP settlement line (dashboard export). (3) Run **intentional** webhook replay (FT-PAY-01 step 6) and confirm reconciliation **unchanged**. | All three **match** for sampled orders; replay **does not** change financial summary. | | | | |
| FT-RLB-01 | P0/P2 | Release engineer access; **previous** digest-pinned `APP_IMAGE_REF` + `GOOSE_IMAGE_REF` documented. | (1) Record current + previous image digests from last **production-deployment-manifest** artifact **or** ticket. (2) Walk **[`production-cutover-rollback.md`](../runbooks/production-cutover-rollback.md)** rollback section for **2-VPS** topology (table-top acceptable with sign-off). (3) If rehearsal: execute rollback on **staging** mirror with same script entrypoints. | Runbook steps **complete**; owners named; **no** reliance on undeclared `latest` tags; DB rollback **explicitly out of scope** for image rollback. | | | | |
| FT-BKP-01 | P0/P2 | DBA / managed DB owner. | (1) Record **backup evidence id** or PITR window per **[`production-backup-restore-dr.md`](../runbooks/production-backup-restore-dr.md)** before migration. (2) If policy demands: restore to **non-prod** clone and run **FT-CAT-01** smoke read-only. | Ticket holds **backup id**; restore drill **pass** or waiver **signed**. | | | | |

---

## Appendix — TSV for Sheets

Paste into **A1** (tab-separated one line per row after header):

```text
Case ID	Priority	Setup	Steps	Expected	Actual	Pass/Fail	Evidence	Owner
FT-ADM-01	P1	Admin + non-admin users; REST base URL	Login admin; GET me; non-admin POST admin product; GET audit	403/404; no tenant leak				
FT-ADM-02	P1	Admin JWT	POST product; GET product	Created id visible			
FT-ADM-03	P1	Admin JWT; object storage	Upload media; GET product	HTTPS URLs; hash metadata			
FT-MAC-01	P1	Machine JWT; grpcs endpoint	Claim or refresh; GetBootstrap	Bootstrap payload OK			
FT-CAT-01	P1	Machine JWT	GetCatalogSnapshot; record version; GetCatalogDelta	BasisMatches when unchanged			
FT-MED-02	P1	Catalog + disk cache	Download; verify SHA-256; offline render; bump media	New hash after bump			
FT-PAY-01	P1	PSP + webhook secret	CreateOrder; CreatePaymentSession; pay; webhook; poll; replay webhook	Paid once; idempotent webhook			
FT-PAY-02	P1	Machine JWT cash path	Create cash order; confirm cash; vend; retry confirm	Replay-safe cash			
FT-VND-01	P1	Paid order	StartVend; success RPC; duplicate success	One inventory move; replay			
FT-PAY-03	P1	Paid + vend fail	StartVend; fail; ReportVendFailure; refund path	Refund or ticket; no double refund			
FT-OFF-01	P1	Offline queue	Queue events; PushOfflineEvents; duplicate	REPLAYED on dup			
FT-PAY-04	P1	Network drop mid-pay	CreatePaymentSession; drop net; restore; poll	Recoverable state; no dup paid			
FT-VND-03	P1	Mid-vend kill	StartVend; power loss; reboot; reconcile	One terminal outcome			
FT-MQT-01	P1	MQTT TLS + ledger	Dispatch; device ACK	Ledger acked			
FT-MQT-02	P1	After ACK	Duplicate ACK	Idempotent			
FT-TEL-01	P1/P2	Telemetry enabled	Batch; critical; dup; reconcile	No dup side effects			
FT-OBX-01	P1/P2	Worker outbox	Enqueue; publish; fail/recover	No stuck pending past SLO			
FT-PAY-05	P1	PSP + server totals	GET reconciliation; compare PSP	export; replay webhook unchanged	3-way match			
FT-RLB-01	P0/P2	Digest pairs documented	Walk rollback runbook	Rollback path validated			
FT-BKP-01	P0/P2	DBA	Backup id before migrate; optional restore drill	Evidence recorded			
```

---

## Related

- **[`production-final-contract.md`](../architecture/production-final-contract.md)** — normative architecture
- **[`field-pilot-checklist.md`](../operations/field-pilot-checklist.md)** — pilot sequencing
- **[`field-rollout-checklist.md`](../operations/field-rollout-checklist.md)** — widen tranches
- **[`local-e2e.md`](local-e2e.md)** — automated DB suites (not field-only proof)
- **[`kiosk-app-implementation-checklist.md`](../api/kiosk-app-implementation-checklist.md)** — Android acceptance
