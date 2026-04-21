-- +goose Up
-- +goose StatementBegin

-- Multi-cabinet topology, machine assortments, append-only inventory ledger, and count sessions.
-- Idempotent: safe to re-apply on mixed DB states.

CREATE TABLE IF NOT EXISTS machine_cabinets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    cabinet_code text NOT NULL,
    title text NOT NULL DEFAULT '',
    sort_order int NOT NULL DEFAULT 0,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_machine_cabinets_cabinet_code_nonempty CHECK (btrim(cabinet_code) <> '')
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_machine_cabinets_machine_cabinet_code ON machine_cabinets (machine_id, cabinet_code);

CREATE INDEX IF NOT EXISTS ix_machine_cabinets_machine_sort ON machine_cabinets (machine_id, sort_order ASC, id ASC);

CREATE TABLE IF NOT EXISTS assortments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    name text NOT NULL,
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
    description text NOT NULL DEFAULT '',
    meta jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_assortments_name_nonempty CHECK (btrim(name) <> '')
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_assortments_org_id ON assortments (organization_id, id);

CREATE UNIQUE INDEX IF NOT EXISTS ux_assortments_org_name_lower ON assortments (organization_id, lower(name));

CREATE INDEX IF NOT EXISTS ix_assortments_organization_id ON assortments (organization_id);

CREATE TABLE IF NOT EXISTS assortment_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    assortment_id uuid NOT NULL REFERENCES assortments (id) ON DELETE CASCADE,
    product_id uuid NOT NULL,
    sort_order int NOT NULL DEFAULT 0,
    notes jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT fk_assortment_items_org_product FOREIGN KEY (organization_id, product_id) REFERENCES products (organization_id, id) ON DELETE CASCADE,
    CONSTRAINT fk_assortment_items_org_assortment FOREIGN KEY (organization_id, assortment_id) REFERENCES assortments (organization_id, id) ON DELETE CASCADE,
    CONSTRAINT ux_assortment_items_assortment_product UNIQUE (assortment_id, product_id)
);

CREATE INDEX IF NOT EXISTS ix_assortment_items_assortment_sort ON assortment_items (assortment_id, sort_order ASC, id ASC);

CREATE INDEX IF NOT EXISTS ix_assortment_items_product_id ON assortment_items (product_id);

CREATE TABLE IF NOT EXISTS machine_assortment_bindings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    assortment_id uuid NOT NULL REFERENCES assortments (id) ON DELETE RESTRICT,
    is_primary boolean NOT NULL DEFAULT false,
    valid_from timestamptz NOT NULL DEFAULT now(),
    valid_to timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT fk_machine_assortment_bindings_org_machine FOREIGN KEY (organization_id, machine_id) REFERENCES machines (organization_id, id) ON DELETE CASCADE,
    CONSTRAINT fk_machine_assortment_bindings_org_assortment FOREIGN KEY (organization_id, assortment_id) REFERENCES assortments (organization_id, id) ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_machine_assortment_bindings_one_active_primary ON machine_assortment_bindings (machine_id)
WHERE
    is_primary
    AND valid_to IS NULL;

CREATE INDEX IF NOT EXISTS ix_machine_assortment_bindings_machine_valid_from ON machine_assortment_bindings (machine_id, valid_from DESC);

CREATE INDEX IF NOT EXISTS ix_machine_assortment_bindings_assortment ON machine_assortment_bindings (assortment_id);

CREATE TABLE IF NOT EXISTS inventory_count_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    operator_session_id uuid REFERENCES machine_operator_sessions (id) ON DELETE SET NULL,
    status text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'closed', 'cancelled')),
    started_at timestamptz NOT NULL DEFAULT now(),
    ended_at timestamptz,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT fk_inventory_count_sessions_org_machine FOREIGN KEY (organization_id, machine_id) REFERENCES machines (organization_id, id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS ix_inventory_count_sessions_machine_started ON inventory_count_sessions (machine_id, started_at DESC);

CREATE INDEX IF NOT EXISTS ix_inventory_count_sessions_org_started ON inventory_count_sessions (organization_id, started_at DESC);

CREATE TABLE IF NOT EXISTS inventory_events (
    id bigserial PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    machine_cabinet_id uuid REFERENCES machine_cabinets (id) ON DELETE SET NULL,
    slot_code text,
    product_id uuid,
    event_type text NOT NULL CHECK (
        event_type IN (
            'sale',
            'restock',
            'adjustment',
            'audit',
            'waste',
            'transfer_in',
            'transfer_out',
            'count',
            'reconcile',
            'correction',
            'other'
        )
    ),
    quantity_delta int NOT NULL,
    quantity_after int,
    correlation_id uuid,
    operator_session_id uuid REFERENCES machine_operator_sessions (id) ON DELETE SET NULL,
    refill_session_id uuid REFERENCES refill_sessions (id) ON DELETE SET NULL,
    inventory_count_session_id uuid REFERENCES inventory_count_sessions (id) ON DELETE SET NULL,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT fk_inventory_events_org_machine FOREIGN KEY (organization_id, machine_id) REFERENCES machines (organization_id, id) ON DELETE CASCADE,
    CONSTRAINT fk_inventory_events_org_product FOREIGN KEY (organization_id, product_id) REFERENCES products (organization_id, id) ON DELETE SET NULL,
    CONSTRAINT ck_inventory_events_slot_code_nonempty CHECK (slot_code IS NULL OR btrim(slot_code) <> '')
);

CREATE INDEX IF NOT EXISTS ix_inventory_events_machine_occurred ON inventory_events (machine_id, occurred_at DESC);

CREATE INDEX IF NOT EXISTS ix_inventory_events_org_occurred ON inventory_events (organization_id, occurred_at DESC);

CREATE INDEX IF NOT EXISTS ix_inventory_events_machine_slot_occurred ON inventory_events (machine_id, slot_code, occurred_at DESC)
WHERE
    slot_code IS NOT NULL;

CREATE INDEX IF NOT EXISTS ix_inventory_events_machine_product_occurred ON inventory_events (machine_id, product_id, occurred_at DESC)
WHERE
    product_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS ix_inventory_events_correlation ON inventory_events (correlation_id, occurred_at DESC)
WHERE
    correlation_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS ix_inventory_events_count_session ON inventory_events (inventory_count_session_id, occurred_at DESC)
WHERE
    inventory_count_session_id IS NOT NULL;

COMMENT ON TABLE machine_cabinets IS 'Logical cabinets on a machine; cabinet_code is stable within machine_id.';

COMMENT ON TABLE assortments IS 'Named product bundles for machine-specific merchandising.';

COMMENT ON TABLE assortment_items IS 'Products belonging to an assortment; sort_order drives UI and default sequencing.';

COMMENT ON TABLE machine_assortment_bindings IS 'Links machines to assortments; at most one active primary binding per machine (valid_to IS NULL, is_primary).';

COMMENT ON TABLE inventory_events IS 'Append-only inventory ledger; application INSERT-only.';

COMMENT ON TABLE inventory_count_sessions IS 'Optional physical count visit context; link operator_session_id when known.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS inventory_events;

DROP TABLE IF EXISTS inventory_count_sessions;

DROP TABLE IF EXISTS machine_assortment_bindings;

DROP TABLE IF EXISTS assortment_items;

DROP TABLE IF EXISTS assortments;

DROP TABLE IF EXISTS machine_cabinets;

-- +goose StatementEnd
