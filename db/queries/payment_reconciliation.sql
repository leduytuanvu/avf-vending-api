-- Upsert commerce reconciliation cases (payment/vend/refund operator queue).

-- Fixed positional params must precede sqlc.narg expansion ($4/$5 before nargs become $6+).

-- name: UpsertCommerceReconciliationCase :one

INSERT INTO commerce_reconciliation_cases (

    organization_id,

    case_type,

    status,

    severity,

    reason,

    metadata,

    order_id,

    payment_id,

    vend_session_id,

    refund_id,

    machine_id,

    provider,

    provider_event_id,

    correlation_key

) VALUES (

    $1,

    $2,

    'open',

    $3,

    $4,

    $5,

    sqlc.narg('order_id')::uuid,

    sqlc.narg('payment_id')::uuid,

    sqlc.narg('vend_session_id')::uuid,

    sqlc.narg('refund_id')::uuid,

    sqlc.narg('machine_id')::uuid,

    sqlc.narg('provider')::text,

    sqlc.narg('provider_event_id')::bigint,

    COALESCE(sqlc.narg('correlation_key')::text, '')

)

ON CONFLICT (

    organization_id,

    case_type,

    COALESCE(order_id, '00000000-0000-0000-0000-000000000000'::uuid),

    COALESCE(payment_id, '00000000-0000-0000-0000-000000000000'::uuid),

    COALESCE(vend_session_id, '00000000-0000-0000-0000-000000000000'::uuid),

    COALESCE(refund_id, '00000000-0000-0000-0000-000000000000'::uuid),

    COALESCE(provider_event_id, 0),

    correlation_key

) WHERE status IN ('open', 'reviewing', 'escalated')

DO UPDATE SET

    severity = EXCLUDED.severity,

    reason = EXCLUDED.reason,

    metadata = EXCLUDED.metadata,

    machine_id = COALESCE(EXCLUDED.machine_id, commerce_reconciliation_cases.machine_id),

    last_detected_at = now()

RETURNING *;

-- Operational drift probes for payment/order alignment (cron or admin dashboards).

-- name: ListReconciliationStalePaidOrdersNotCompleted :many

SELECT

    o.id AS order_id,

    o.organization_id,

    o.machine_id,

    o.status AS order_status,

    o.updated_at AS order_updated_at,

    p.id AS payment_id,

    p.state AS payment_state,

    p.provider,

    p.amount_minor,

    p.currency

FROM

    orders o

    INNER JOIN payments p ON p.order_id = o.id

        AND p.state = 'captured'

WHERE

    o.organization_id = $1

    AND o.status IN ('paid', 'vending')

    AND o.updated_at < (
        now() - ($2::bigint * interval '1 second'))

ORDER BY

    o.updated_at ASC

LIMIT $3;

-- Provider-visible capture while local PSP payment rows stay pre-terminal.

-- name: ListReconciliationProviderCapturedLocalPending :many

SELECT

    e.id AS provider_event_id,

    e.payment_id,

    e.provider,

    p.state AS payment_state,

    e.received_at,

    o.id AS order_id

FROM

    payment_provider_events e

    INNER JOIN payments p ON p.id = e.payment_id

    INNER JOIN orders o ON o.id = p.order_id

WHERE

    o.organization_id = $1

    AND trim(lower(coalesce(e.payload ->> 'normalized_payment_state', ''))) = 'captured'

    AND p.state IN ('created', 'authorized')

    AND lower(trim(p.provider)) NOT IN ('cash')

ORDER BY

    e.received_at DESC

LIMIT $2;

-- Captured PSP rows without webhook evidence rows.

-- name: ListReconciliationLocalCapturedWithoutProviderEvidence :many

SELECT

    p.id AS payment_id,

    p.order_id,

    p.provider,

    p.state,

    p.updated_at,

    p.amount_minor,

    p.currency

FROM

    payments p

    INNER JOIN orders o ON o.id = p.order_id

WHERE

    o.organization_id = $1

    AND p.state = 'captured'

    AND lower(trim(p.provider)) NOT IN ('cash')

    AND NOT EXISTS (

        SELECT

            1

        FROM

            payment_provider_events e

        WHERE

            e.payment_id = p.id

            AND e.ingress_status = 'applied'

            AND e.validation_status IN ('hmac_verified', 'unsigned_development')

    )

ORDER BY

    p.updated_at DESC

LIMIT $2;

-- Applied webhook rows where persisted provider_amount_minor / currency disagrees with the payment ledger row
-- (operations drift: manual fixes, provider bugs, or historical ingest). Excludes cash.

-- name: ListReconciliationAppliedWebhookAmountMismatch :many

SELECT

    e.id AS provider_event_id,

    e.payment_id,

    e.provider,

    e.provider_amount_minor AS webhook_amount_minor,

    p.amount_minor AS payment_amount_minor,

    btrim(e.currency::text) AS webhook_currency,

    btrim(p.currency::text) AS payment_currency,

    e.received_at

FROM

    payment_provider_events e

    INNER JOIN payments p ON p.id = e.payment_id

    INNER JOIN orders o ON o.id = p.order_id

WHERE

    o.organization_id = $1

    AND e.ingress_status = 'applied'

    AND lower(trim(p.provider)) <> 'cash'

    AND (

        (

            e.provider_amount_minor IS NOT NULL

            AND e.provider_amount_minor <> p.amount_minor

        )

        OR (

            e.currency IS NOT NULL

            AND btrim(e.currency::text) <> ''

            AND upper(btrim(e.currency::text)) <> upper(btrim(p.currency::text))

        )

    )

ORDER BY

    e.received_at DESC

LIMIT $2;

