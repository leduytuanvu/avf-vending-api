-- +goose Up
-- +goose StatementBegin

CREATE TABLE organizations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    slug text NOT NULL,
    status text NOT NULL CHECK (status IN ('active', 'suspended')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ux_organizations_slug_lower ON organizations (lower(slug));

CREATE TABLE regions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    name text NOT NULL,
    code text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ux_regions_org_code ON regions (organization_id, lower(code));
CREATE INDEX ix_regions_organization_id ON regions (organization_id);

CREATE TABLE sites (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    region_id uuid REFERENCES regions (id) ON DELETE SET NULL,
    name text NOT NULL,
    address jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_sites_organization_id ON sites (organization_id);
CREATE INDEX ix_sites_region_id ON sites (region_id);

CREATE TABLE machine_hardware_profiles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid REFERENCES organizations (id) ON DELETE CASCADE,
    name text NOT NULL,
    spec jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_machine_hardware_profiles_organization_id ON machine_hardware_profiles (organization_id);

CREATE TABLE machines (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    site_id uuid NOT NULL REFERENCES sites (id) ON DELETE RESTRICT,
    hardware_profile_id uuid REFERENCES machine_hardware_profiles (id) ON DELETE SET NULL,
    serial_number text NOT NULL,
    name text NOT NULL DEFAULT '',
    status text NOT NULL CHECK (status IN ('provisioning', 'online', 'offline', 'maintenance', 'retired')),
    command_sequence bigint NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ux_machines_org_serial UNIQUE (organization_id, serial_number)
);

CREATE INDEX ix_machines_site_id ON machines (site_id);
CREATE INDEX ix_machines_hardware_profile_id ON machines (hardware_profile_id);

CREATE TABLE technicians (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    display_name text NOT NULL,
    email text,
    phone text,
    external_subject text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ux_technicians_org_email_lower ON technicians (organization_id, lower(email))
    WHERE email IS NOT NULL AND btrim(email) <> '';

CREATE INDEX ix_technicians_organization_id ON technicians (organization_id);

CREATE TABLE technician_machine_assignments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    technician_id uuid NOT NULL REFERENCES technicians (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    role text NOT NULL,
    valid_from timestamptz NOT NULL DEFAULT now(),
    valid_to timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_tma_technician_id ON technician_machine_assignments (technician_id);
CREATE INDEX ix_tma_machine_id ON technician_machine_assignments (machine_id);

CREATE TABLE products (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    sku text NOT NULL,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    attrs jsonb NOT NULL DEFAULT '{}'::jsonb,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ux_products_org_sku UNIQUE (organization_id, sku)
);

CREATE INDEX ix_products_organization_id ON products (organization_id);

CREATE TABLE price_books (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    name text NOT NULL,
    currency char(3) NOT NULL,
    effective_from timestamptz NOT NULL,
    effective_to timestamptz,
    is_default boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_price_books_organization_id ON price_books (organization_id);

CREATE TABLE price_book_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    price_book_id uuid NOT NULL REFERENCES price_books (id) ON DELETE CASCADE,
    product_id uuid NOT NULL REFERENCES products (id) ON DELETE RESTRICT,
    unit_price_minor bigint NOT NULL CHECK (unit_price_minor >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ux_price_book_items_book_product UNIQUE (price_book_id, product_id)
);

CREATE INDEX ix_price_book_items_product_id ON price_book_items (product_id);

CREATE TABLE planograms (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    name text NOT NULL,
    revision int NOT NULL DEFAULT 1,
    status text NOT NULL CHECK (status IN ('draft', 'published', 'archived')),
    meta jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ux_planograms_org_name_revision UNIQUE (organization_id, name, revision)
);

CREATE INDEX ix_planograms_organization_id ON planograms (organization_id);

CREATE TABLE slots (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    planogram_id uuid NOT NULL REFERENCES planograms (id) ON DELETE CASCADE,
    slot_index int NOT NULL CHECK (slot_index >= 0),
    product_id uuid REFERENCES products (id) ON DELETE SET NULL,
    max_quantity int NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ux_slots_planogram_index UNIQUE (planogram_id, slot_index)
);

CREATE INDEX ix_slots_product_id ON slots (product_id);

CREATE TABLE machine_slot_state (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    planogram_id uuid NOT NULL REFERENCES planograms (id) ON DELETE CASCADE,
    slot_index int NOT NULL CHECK (slot_index >= 0),
    current_quantity int NOT NULL DEFAULT 0,
    price_minor bigint NOT NULL DEFAULT 0,
    planogram_revision_applied int NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ux_machine_slot_state_machine_plan_slot UNIQUE (machine_id, planogram_id, slot_index)
);

CREATE INDEX ix_machine_slot_state_planogram_id ON machine_slot_state (planogram_id);

CREATE TABLE orders (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE RESTRICT,
    status text NOT NULL CHECK (status IN ('created', 'quoted', 'paid', 'vending', 'completed', 'failed', 'cancelled')),
    currency char(3) NOT NULL,
    subtotal_minor bigint NOT NULL DEFAULT 0 CHECK (subtotal_minor >= 0),
    tax_minor bigint NOT NULL DEFAULT 0 CHECK (tax_minor >= 0),
    total_minor bigint NOT NULL DEFAULT 0 CHECK (total_minor >= 0),
    idempotency_key text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ux_orders_org_idempotency ON orders (organization_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL AND btrim(idempotency_key) <> '';

CREATE INDEX ix_orders_machine_id ON orders (machine_id);

CREATE TABLE vend_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id uuid NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE RESTRICT,
    slot_index int NOT NULL,
    product_id uuid NOT NULL REFERENCES products (id) ON DELETE RESTRICT,
    state text NOT NULL CHECK (state IN ('pending', 'in_progress', 'success', 'failed')),
    failure_reason text,
    correlation_id uuid,
    started_at timestamptz,
    completed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_vend_sessions_order_id ON vend_sessions (order_id);
CREATE INDEX ix_vend_sessions_machine_id ON vend_sessions (machine_id);

CREATE TABLE payments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id uuid NOT NULL REFERENCES orders (id) ON DELETE RESTRICT,
    provider text NOT NULL,
    state text NOT NULL CHECK (state IN ('created', 'authorized', 'captured', 'failed', 'refunded')),
    amount_minor bigint NOT NULL CHECK (amount_minor >= 0),
    currency char(3) NOT NULL,
    idempotency_key text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ux_payments_order_idempotency ON payments (order_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL AND btrim(idempotency_key) <> '';

CREATE INDEX ix_payments_order_id ON payments (order_id);

CREATE TABLE payment_attempts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_id uuid NOT NULL REFERENCES payments (id) ON DELETE CASCADE,
    provider_reference text,
    state text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_payment_attempts_payment_id ON payment_attempts (payment_id);

CREATE TABLE refunds (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_id uuid NOT NULL REFERENCES payments (id) ON DELETE RESTRICT,
    order_id uuid NOT NULL REFERENCES orders (id) ON DELETE RESTRICT,
    amount_minor bigint NOT NULL CHECK (amount_minor >= 0),
    state text NOT NULL CHECK (state IN ('requested', 'processing', 'completed', 'failed')),
    reason text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_refunds_payment_id ON refunds (payment_id);
CREATE INDEX ix_refunds_order_id ON refunds (order_id);

CREATE TABLE command_ledger (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    sequence bigint NOT NULL,
    command_type text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    correlation_id uuid,
    idempotency_key text,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ux_command_ledger_machine_sequence UNIQUE (machine_id, sequence)
);

CREATE UNIQUE INDEX ux_command_ledger_machine_idempotency ON command_ledger (machine_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL AND btrim(idempotency_key) <> '';

CREATE INDEX ix_command_ledger_machine_id ON command_ledger (machine_id);

CREATE TABLE machine_shadow (
    machine_id uuid PRIMARY KEY REFERENCES machines (id) ON DELETE CASCADE,
    desired_state jsonb NOT NULL DEFAULT '{}'::jsonb,
    reported_state jsonb NOT NULL DEFAULT '{}'::jsonb,
    version bigint NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE outbox_events (
    id bigserial PRIMARY KEY,
    organization_id uuid REFERENCES organizations (id) ON DELETE SET NULL,
    topic text NOT NULL,
    event_type text NOT NULL,
    payload jsonb NOT NULL,
    aggregate_type text NOT NULL,
    aggregate_id uuid NOT NULL,
    idempotency_key text,
    created_at timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz
);

CREATE UNIQUE INDEX ux_outbox_topic_idempotency ON outbox_events (topic, idempotency_key)
    WHERE idempotency_key IS NOT NULL AND btrim(idempotency_key) <> '';

CREATE INDEX ix_outbox_unpublished ON outbox_events (created_at)
    WHERE published_at IS NULL;

CREATE TABLE audit_logs (
    id bigserial PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    actor_type text NOT NULL,
    actor_id text NOT NULL DEFAULT '',
    action text NOT NULL,
    resource_type text NOT NULL,
    resource_id uuid,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    ip text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_audit_logs_organization_id ON audit_logs (organization_id);

CREATE TABLE ota_artifacts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    storage_key text NOT NULL,
    sha256 text,
    size_bytes bigint CHECK (size_bytes IS NULL OR size_bytes >= 0),
    semver text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_ota_artifacts_organization_id ON ota_artifacts (organization_id);

CREATE TABLE ota_campaigns (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    name text NOT NULL,
    artifact_id uuid NOT NULL REFERENCES ota_artifacts (id) ON DELETE RESTRICT,
    strategy text NOT NULL DEFAULT 'rolling',
    status text NOT NULL CHECK (status IN ('draft', 'active', 'paused', 'completed')),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_ota_campaigns_organization_id ON ota_campaigns (organization_id);

CREATE TABLE ota_targets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id uuid NOT NULL REFERENCES ota_campaigns (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    state text NOT NULL DEFAULT 'pending',
    last_error text,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ux_ota_targets_campaign_machine UNIQUE (campaign_id, machine_id)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS ota_targets CASCADE;
DROP TABLE IF EXISTS ota_campaigns CASCADE;
DROP TABLE IF EXISTS ota_artifacts CASCADE;
DROP TABLE IF EXISTS audit_logs CASCADE;
DROP TABLE IF EXISTS outbox_events CASCADE;
DROP TABLE IF EXISTS machine_shadow CASCADE;
DROP TABLE IF EXISTS command_ledger CASCADE;
DROP TABLE IF EXISTS refunds CASCADE;
DROP TABLE IF EXISTS payment_attempts CASCADE;
DROP TABLE IF EXISTS payments CASCADE;
DROP TABLE IF EXISTS vend_sessions CASCADE;
DROP TABLE IF EXISTS orders CASCADE;
DROP TABLE IF EXISTS machine_slot_state CASCADE;
DROP TABLE IF EXISTS slots CASCADE;
DROP TABLE IF EXISTS planograms CASCADE;
DROP TABLE IF EXISTS price_book_items CASCADE;
DROP TABLE IF EXISTS price_books CASCADE;
DROP TABLE IF EXISTS products CASCADE;
DROP TABLE IF EXISTS technician_machine_assignments CASCADE;
DROP TABLE IF EXISTS technicians CASCADE;
DROP TABLE IF EXISTS machines CASCADE;
DROP TABLE IF EXISTS machine_hardware_profiles CASCADE;
DROP TABLE IF EXISTS sites CASCADE;
DROP TABLE IF EXISTS regions CASCADE;
DROP TABLE IF EXISTS organizations CASCADE;

-- +goose StatementEnd
