-- Cash forensic ledger: payout, lifecycle, hardware observations, adjustments.
-- +goose Up
-- +goose StatementBegin

ALTER TABLE cash_acceptance_events
    ADD COLUMN IF NOT EXISTS boot_id text,
    ADD COLUMN IF NOT EXISTS occurred_at_device timestamptz;

UPDATE cash_acceptance_events
SET occurred_at_device = accepted_at
WHERE occurred_at_device IS NULL;

CREATE INDEX IF NOT EXISTS ix_cash_acceptance_events_machine_time
    ON cash_acceptance_events (machine_id, accepted_at DESC, id);

CREATE TABLE IF NOT EXISTS cash_payout_events (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v7(),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE RESTRICT,
    order_id uuid REFERENCES orders (id) ON DELETE SET NULL,
    withdrawal_id text NOT NULL,
    note_sequence int NOT NULL DEFAULT 0,
    event_type text NOT NULL CHECK (
        event_type IN (
            'BEGUN',
            'NOTE_DISPENSE_REQUESTED',
            'NOTE_COMMAND_ACCEPTED',
            'NOTE_CONFIRMED',
            'NOTE_MONITOR_FAILED',
            'NOTE_RECONCILIATION_STARTED',
            'NOTE_FINALITY_PENDING',
            'NOTE_DELIVERED_AFTER_FAULT',
            'NOTE_FAILED',
            'NOTE_NOT_DELIVERED',
            'NOTE_AMBIGUOUS',
            'COMPLETED',
            'PARTIAL',
            'PARTIAL_AFTER_FAULT',
            'FAILED',
            'AMBIGUOUS',
            'RECOVERY'
        )
    ),
    device_event_id text NOT NULL,
    denomination_minor bigint NOT NULL DEFAULT 0 CHECK (denomination_minor >= 0),
    amount_minor bigint NOT NULL DEFAULT 0 CHECK (amount_minor >= 0),
    recycler_count_before int,
    recycler_count_after int,
    outcome_finality text NOT NULL DEFAULT 'requested' CHECK (
        outcome_finality IN (
            'requested',
            'command_accepted',
            'confirmed',
            'ambiguous',
            'failed',
            'not_delivered'
        )
    ),
    currency char(3) NOT NULL,
    occurred_at_device timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    raw_metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT ux_cash_payout_events_machine_device UNIQUE (machine_id, device_event_id),
    CONSTRAINT ux_cash_payout_events_withdrawal_note_type UNIQUE (
        machine_id,
        withdrawal_id,
        note_sequence,
        event_type
    )
);

CREATE INDEX IF NOT EXISTS ix_cash_payout_events_machine_time
    ON cash_payout_events (machine_id, occurred_at_device DESC, id);

CREATE INDEX IF NOT EXISTS ix_cash_payout_events_order
    ON cash_payout_events (order_id)
    WHERE order_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS cash_bill_lifecycle_events (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v7(),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE RESTRICT,
    order_id uuid REFERENCES orders (id) ON DELETE SET NULL,
    device_event_id text NOT NULL,
    lifecycle_type text NOT NULL CHECK (
        lifecycle_type IN (
            'escrow_held',
            'returned_customer',
            'transfer_to_escrow',
            'transfer_to_cashbox',
            'record_ambiguous',
            'fault'
        )
    ),
    denomination_minor bigint NOT NULL DEFAULT 0 CHECK (denomination_minor >= 0),
    raw_record_hex text NOT NULL DEFAULT '',
    currency char(3) NOT NULL DEFAULT 'VND',
    occurred_at_device timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    raw_metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT ux_cash_bill_lifecycle_events_machine_device UNIQUE (machine_id, device_event_id)
);

CREATE INDEX IF NOT EXISTS ix_cash_bill_lifecycle_events_machine_time
    ON cash_bill_lifecycle_events (machine_id, occurred_at_device DESC, id);

CREATE TABLE IF NOT EXISTS cash_hardware_observations (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v7(),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE RESTRICT,
    device_event_id text NOT NULL,
    observed_at_device timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    recycler_denomination_minor bigint NOT NULL CHECK (recycler_denomination_minor > 0),
    recycler_count int NOT NULL CHECK (recycler_count >= 0),
    cashbox_count int,
    source text NOT NULL CHECK (
        source IN ('poll', 'payout', 'startup_recovery', 'collection')
    ),
    currency char(3) NOT NULL,
    raw_metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT ux_cash_hardware_observations_machine_device UNIQUE (machine_id, device_event_id)
);

CREATE INDEX IF NOT EXISTS ix_cash_hardware_observations_machine_time
    ON cash_hardware_observations (machine_id, observed_at_device DESC, id);

CREATE TABLE IF NOT EXISTS cash_adjustments (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v7(),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE RESTRICT,
    amount_minor bigint NOT NULL,
    bucket text NOT NULL CHECK (bucket IN ('cashbox', 'recycler', 'unallocated_wallet')),
    reason text NOT NULL,
    operator_account_id uuid REFERENCES platform_auth_accounts (id) ON DELETE SET NULL,
    idempotency_key text NOT NULL,
    currency char(3) NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ux_cash_adjustments_idempotency UNIQUE (idempotency_key)
);

CREATE INDEX IF NOT EXISTS ix_cash_adjustments_machine_time
    ON cash_adjustments (machine_id, created_at DESC);

ALTER TABLE commerce_reconciliation_cases DROP CONSTRAINT IF EXISTS commerce_reconciliation_cases_case_type_check;

ALTER TABLE commerce_reconciliation_cases ADD CONSTRAINT commerce_reconciliation_cases_case_type_check CHECK (
    case_type IN (
        'payment_paid_vend_not_started',
        'payment_paid_vend_failed',
        'vend_started_no_terminal_ack',
        'refund_pending_too_long',
        'webhook_provider_mismatch',
        'duplicate_provider_event',
        'duplicate_payment',
        'webhook_amount_currency_mismatch',
        'webhook_after_terminal_order',
        'settlement_amount_mismatch',
        'duplicate_payment_ambiguous_winner',
        'late_capture_refund_required',
        'legacy_cash_confirm_unknown_consent',
        'cash_gross_mismatch',
        'change_liability_unresolved',
        'unclaimed_capture',
        'cash_payout_ambiguous'
    )
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE commerce_reconciliation_cases DROP CONSTRAINT IF EXISTS commerce_reconciliation_cases_case_type_check;

ALTER TABLE commerce_reconciliation_cases ADD CONSTRAINT commerce_reconciliation_cases_case_type_check CHECK (
    case_type IN (
        'payment_paid_vend_not_started',
        'payment_paid_vend_failed',
        'vend_started_no_terminal_ack',
        'refund_pending_too_long',
        'webhook_provider_mismatch',
        'duplicate_provider_event',
        'duplicate_payment',
        'webhook_amount_currency_mismatch',
        'webhook_after_terminal_order',
        'settlement_amount_mismatch',
        'duplicate_payment_ambiguous_winner',
        'late_capture_refund_required',
        'legacy_cash_confirm_unknown_consent',
        'cash_gross_mismatch',
        'change_liability_unresolved'
    )
);

DROP TABLE IF EXISTS cash_adjustments;
DROP TABLE IF EXISTS cash_hardware_observations;
DROP TABLE IF EXISTS cash_bill_lifecycle_events;
DROP TABLE IF EXISTS cash_payout_events;

DROP INDEX IF EXISTS ix_cash_acceptance_events_machine_time;

ALTER TABLE cash_acceptance_events
    DROP COLUMN IF EXISTS boot_id,
    DROP COLUMN IF EXISTS occurred_at_device;

-- +goose StatementEnd
