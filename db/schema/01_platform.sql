-- Canonical DDL mirror of goose migrations (00002 platform, 00004 device ingest, 00005 catalog/pricing/promotions, 00006 command protocol traceability, 00007 financial ledger reconciliation, 00008 machine operator sessions, 00009 operator action attribution correlation, 00010 operator session activity end reason, 00011 operator domain resources, 00014 platform auth API accounts, 00015 machine cabinets/assortments/inventory, 00016 machine slot layouts/configs, 00017 platform timezones and machine identity snapshot columns, 00018 machine cabinet index/slot_capacity/status, 00019 inventory ledger columns and refill_session_lines) for sqlc.
-- When changing schema, update migrations first, then sync this file.

CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE organizations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    slug text NOT NULL,
    status text NOT NULL CHECK (status IN ('active', 'suspended')),
    default_timezone text NOT NULL DEFAULT 'UTC',
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
    timezone text NOT NULL DEFAULT 'UTC',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_sites_organization_id ON sites (organization_id);
CREATE INDEX ix_sites_region_id ON sites (region_id);

CREATE UNIQUE INDEX ux_sites_org_id ON sites (organization_id, id);

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
    timezone_override text NULL,
    name text NOT NULL DEFAULT '',
    status text NOT NULL CHECK (status IN ('provisioning', 'online', 'offline', 'maintenance', 'retired')),
    command_sequence bigint NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ux_machines_org_serial UNIQUE (organization_id, serial_number)
);

CREATE INDEX ix_machines_site_id ON machines (site_id);
CREATE INDEX ix_machines_hardware_profile_id ON machines (hardware_profile_id);

CREATE UNIQUE INDEX ux_machines_org_id ON machines (organization_id, id);

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

-- Machine operator sessions (see migrations/00008_machine_operator_sessions.sql). Text CHECKs replace PG enums in this repo.
CREATE TABLE machine_operator_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    actor_type text NOT NULL CHECK (actor_type IN ('TECHNICIAN', 'USER')),
    technician_id uuid REFERENCES technicians (id) ON DELETE SET NULL,
    user_principal text,
    status text NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'ENDED', 'EXPIRED', 'REVOKED')),
    started_at timestamptz NOT NULL DEFAULT now(),
    ended_at timestamptz,
    expires_at timestamptz,
    client_metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    last_activity_at timestamptz NOT NULL DEFAULT now(),
    ended_reason text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_operator_session_actor_shape CHECK (
        (actor_type = 'TECHNICIAN' AND technician_id IS NOT NULL AND user_principal IS NULL)
        OR (
            actor_type = 'USER'
            AND technician_id IS NULL
            AND user_principal IS NOT NULL
            AND btrim(user_principal) <> ''
        )
    )
);

CREATE UNIQUE INDEX ux_machine_operator_sessions_one_active ON machine_operator_sessions (machine_id)
    WHERE status = 'ACTIVE';

CREATE INDEX ix_machine_operator_sessions_machine_started ON machine_operator_sessions (machine_id, started_at DESC);
CREATE INDEX ix_machine_operator_sessions_org_started ON machine_operator_sessions (organization_id, started_at DESC);
CREATE INDEX ix_machine_operator_sessions_technician ON machine_operator_sessions (technician_id, started_at DESC)
    WHERE technician_id IS NOT NULL;
CREATE INDEX ix_machine_operator_sessions_user_principal ON machine_operator_sessions (organization_id, user_principal, started_at DESC)
    WHERE actor_type = 'USER' AND user_principal IS NOT NULL;
CREATE INDEX ix_machine_operator_sessions_org_machine_started ON machine_operator_sessions (organization_id, machine_id, started_at DESC);
CREATE INDEX ix_machine_operator_sessions_org_active_started ON machine_operator_sessions (organization_id, started_at DESC)
    WHERE status = 'ACTIVE';

COMMENT ON TABLE machine_operator_sessions IS 'Machine-side operator login context; machine identity stays on machines, technician identity on technicians, USER uses opaque user_principal (IdP sub / admin id).';
COMMENT ON COLUMN machine_operator_sessions.user_principal IS 'Non-technician operator identity when actor_type=USER; never store technician PII here.';
COMMENT ON COLUMN machine_operator_sessions.client_metadata IS 'Device/session hints (app version, locale); avoid secrets.';
COMMENT ON COLUMN machine_operator_sessions.last_activity_at IS 'Last client heartbeat or successful session activity; updated independently of generic updated_at when desired.';
COMMENT ON COLUMN machine_operator_sessions.ended_reason IS 'Optional stable code or free text for why the session ended.';

CREATE TABLE machine_operator_auth_events (
    id bigserial PRIMARY KEY,
    operator_session_id uuid REFERENCES machine_operator_sessions (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    event_type text NOT NULL CHECK (
        event_type IN ('login_success', 'login_failure', 'logout', 'session_refresh', 'lockout', 'unknown')
    ),
    auth_method text NOT NULL CHECK (
        auth_method IN ('pin', 'password', 'badge', 'oidc', 'device_cert', 'unknown')
    ),
    occurred_at timestamptz NOT NULL DEFAULT now(),
    correlation_id uuid,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX ix_machine_operator_auth_events_machine_time ON machine_operator_auth_events (machine_id, occurred_at DESC);
CREATE INDEX ix_machine_operator_auth_events_session_time ON machine_operator_auth_events (operator_session_id, occurred_at DESC)
    WHERE operator_session_id IS NOT NULL;
CREATE INDEX ix_machine_operator_auth_events_correlation ON machine_operator_auth_events (correlation_id, occurred_at DESC)
    WHERE correlation_id IS NOT NULL;

COMMENT ON TABLE machine_operator_auth_events IS 'Append-only auth audit for operator sessions; do not UPDATE rows.';
COMMENT ON COLUMN machine_operator_auth_events.operator_session_id IS 'NULL allowed for machine-scoped login_failure before a session row exists.';

CREATE TABLE machine_action_attributions (
    id bigserial PRIMARY KEY,
    operator_session_id uuid REFERENCES machine_operator_sessions (id) ON DELETE SET NULL,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    action_origin_type text NOT NULL CHECK (
        action_origin_type IN ('operator_session', 'system', 'scheduled', 'api', 'remote_support')
    ),
    resource_type text NOT NULL,
    resource_id text NOT NULL,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    correlation_id uuid
);

CREATE INDEX ix_machine_action_attributions_resource_time ON machine_action_attributions (resource_type, resource_id, occurred_at DESC);
CREATE INDEX ix_machine_action_attributions_machine_resource_time ON machine_action_attributions (machine_id, resource_type, resource_id, occurred_at DESC);
CREATE INDEX ix_machine_action_attributions_session_time ON machine_action_attributions (operator_session_id, occurred_at DESC)
    WHERE operator_session_id IS NOT NULL;
CREATE INDEX ix_machine_action_attributions_machine_time ON machine_action_attributions (machine_id, occurred_at DESC);

CREATE INDEX ix_machine_action_attributions_correlation ON machine_action_attributions (correlation_id, occurred_at DESC)
WHERE
    correlation_id IS NOT NULL;

CREATE INDEX ix_machine_action_attributions_machine_correlation ON machine_action_attributions (machine_id, correlation_id, occurred_at DESC)
WHERE
    correlation_id IS NOT NULL;

COMMENT ON TABLE machine_action_attributions IS 'Links domain actions to operator_session_id when known; resource_type/resource_id are polymorphic (e.g. command_ledger uuid as text).';
COMMENT ON COLUMN machine_action_attributions.operator_session_id IS 'NULL allowed for unattended/system/scheduled actions.';
COMMENT ON COLUMN machine_action_attributions.correlation_id IS 'Optional request/correlation id aligned with device and auth event tracing.';

CREATE TABLE categories (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    slug text NOT NULL,
    name text NOT NULL,
    parent_id uuid REFERENCES categories (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ux_categories_org_slug_lower ON categories (organization_id, lower(slug));
CREATE UNIQUE INDEX ux_categories_org_id ON categories (organization_id, id);

CREATE INDEX ix_categories_organization_id ON categories (organization_id);
CREATE INDEX ix_categories_parent_id ON categories (parent_id);

CREATE TABLE brands (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    slug text NOT NULL,
    name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ux_brands_org_slug_lower ON brands (organization_id, lower(slug));
CREATE UNIQUE INDEX ux_brands_org_id ON brands (organization_id, id);

CREATE INDEX ix_brands_organization_id ON brands (organization_id);

CREATE TABLE products (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    sku text NOT NULL,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    attrs jsonb NOT NULL DEFAULT '{}'::jsonb,
    active boolean NOT NULL DEFAULT true,
    category_id uuid,
    brand_id uuid,
    primary_image_id uuid,
    country_of_origin text,
    age_restricted boolean NOT NULL DEFAULT false,
    allergen_codes text[],
    nutritional_note text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ux_products_org_sku UNIQUE (organization_id, sku),
    CONSTRAINT fk_products_org_category FOREIGN KEY (organization_id, category_id)
        REFERENCES categories (organization_id, id),
    CONSTRAINT fk_products_org_brand FOREIGN KEY (organization_id, brand_id)
        REFERENCES brands (organization_id, id)
);

CREATE UNIQUE INDEX ux_products_org_id ON products (organization_id, id);

CREATE INDEX ix_products_organization_id ON products (organization_id);

CREATE TABLE product_images (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id uuid NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    storage_key text NOT NULL,
    cdn_url text,
    alt_text text NOT NULL DEFAULT '',
    sort_order int NOT NULL DEFAULT 0,
    is_primary boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ux_product_images_one_primary_per_product ON product_images (product_id) WHERE is_primary;

CREATE UNIQUE INDEX ux_product_images_product_id_id ON product_images (product_id, id);

CREATE INDEX ix_product_images_product_id ON product_images (product_id);

ALTER TABLE products
    ADD CONSTRAINT fk_products_primary_image FOREIGN KEY (id, primary_image_id)
        REFERENCES product_images (product_id, id) DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE price_books (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    name text NOT NULL,
    currency char(3) NOT NULL,
    effective_from timestamptz NOT NULL,
    effective_to timestamptz,
    is_default boolean NOT NULL DEFAULT false,
    scope_type text NOT NULL DEFAULT 'organization' CHECK (scope_type IN ('organization', 'site', 'machine')),
    site_id uuid,
    machine_id uuid,
    priority int NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_price_books_scope_shape CHECK (
        (scope_type = 'organization' AND site_id IS NULL AND machine_id IS NULL)
        OR (scope_type = 'site' AND site_id IS NOT NULL AND machine_id IS NULL)
        OR (scope_type = 'machine' AND machine_id IS NOT NULL AND site_id IS NULL)
    ),
    CONSTRAINT fk_price_books_org_site FOREIGN KEY (organization_id, site_id)
        REFERENCES sites (organization_id, id),
    CONSTRAINT fk_price_books_org_machine FOREIGN KEY (organization_id, machine_id)
        REFERENCES machines (organization_id, id)
);

CREATE UNIQUE INDEX ux_price_books_org_id ON price_books (organization_id, id);

CREATE UNIQUE INDEX ux_price_books_org_scope_org_name_effective
    ON price_books (organization_id, lower(name), effective_from)
    WHERE scope_type = 'organization';

CREATE UNIQUE INDEX ux_price_books_org_scope_site_name_effective
    ON price_books (organization_id, site_id, lower(name), effective_from)
    WHERE scope_type = 'site';

CREATE UNIQUE INDEX ux_price_books_org_scope_machine_name_effective
    ON price_books (organization_id, machine_id, lower(name), effective_from)
    WHERE scope_type = 'machine';

CREATE INDEX ix_price_books_organization_id ON price_books (organization_id);

CREATE TABLE price_book_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    price_book_id uuid NOT NULL,
    product_id uuid NOT NULL,
    unit_price_minor bigint NOT NULL CHECK (unit_price_minor >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT fk_price_book_items_org_book FOREIGN KEY (organization_id, price_book_id)
        REFERENCES price_books (organization_id, id) ON DELETE CASCADE,
    CONSTRAINT fk_price_book_items_org_product FOREIGN KEY (organization_id, product_id)
        REFERENCES products (organization_id, id) ON DELETE RESTRICT,
    CONSTRAINT ux_price_book_items_org_book_product UNIQUE (organization_id, price_book_id, product_id)
);

CREATE INDEX ix_price_book_items_product_id ON price_book_items (product_id);
CREATE INDEX ix_price_book_items_organization_id ON price_book_items (organization_id);

CREATE TABLE machine_price_overrides (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    product_id uuid NOT NULL REFERENCES products (id) ON DELETE RESTRICT,
    unit_price_minor bigint NOT NULL CHECK (unit_price_minor >= 0),
    currency char(3) NOT NULL,
    valid_from timestamptz NOT NULL,
    valid_to timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT fk_machine_price_overrides_org_machine FOREIGN KEY (organization_id, machine_id)
        REFERENCES machines (organization_id, id),
    CONSTRAINT fk_machine_price_overrides_org_product FOREIGN KEY (organization_id, product_id)
        REFERENCES products (organization_id, id),
    CONSTRAINT ex_machine_price_overrides_no_overlap EXCLUDE USING gist (
        machine_id WITH =,
        product_id WITH =,
        tstzrange(valid_from, COALESCE(valid_to, 'infinity'::timestamptz), '[)') WITH &&
    )
);

CREATE INDEX ix_machine_price_overrides_machine_product_valid
    ON machine_price_overrides (machine_id, product_id, valid_from DESC);

CREATE INDEX ix_machine_price_overrides_organization_id ON machine_price_overrides (organization_id);

CREATE TABLE promotions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    name text NOT NULL,
    approval_status text NOT NULL DEFAULT 'draft' CHECK (
        approval_status IN ('draft', 'pending_approval', 'approved', 'rejected', 'archived')
    ),
    starts_at timestamptz NOT NULL,
    ends_at timestamptz NOT NULL,
    budget_limit_minor bigint,
    redemption_limit int CHECK (redemption_limit IS NULL OR redemption_limit >= 0),
    channel_scope text CHECK (channel_scope IS NULL OR channel_scope IN ('in_person', 'mobile', 'all')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ux_promotions_org_id ON promotions (organization_id, id);

CREATE INDEX ix_promotions_organization_id ON promotions (organization_id);
CREATE INDEX ix_promotions_org_approval ON promotions (organization_id, approval_status);

CREATE TABLE promotion_rules (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    promotion_id uuid NOT NULL REFERENCES promotions (id) ON DELETE CASCADE,
    rule_type text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    priority int NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ux_promotion_rules_promo_type_priority UNIQUE (promotion_id, rule_type, priority)
);

CREATE INDEX ix_promotion_rules_promotion_id ON promotion_rules (promotion_id);

CREATE TABLE promotion_targets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    promotion_id uuid NOT NULL REFERENCES promotions (id) ON DELETE CASCADE,
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    target_type text NOT NULL CHECK (target_type IN ('product', 'category', 'machine', 'site', 'organization')),
    product_id uuid,
    category_id uuid,
    machine_id uuid,
    site_id uuid,
    organization_target_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT fk_promotion_targets_org_promo FOREIGN KEY (organization_id, promotion_id)
        REFERENCES promotions (organization_id, id) ON DELETE CASCADE,
    CONSTRAINT fk_pt_org_product FOREIGN KEY (organization_id, product_id)
        REFERENCES products (organization_id, id) ON DELETE CASCADE,
    CONSTRAINT fk_pt_org_category FOREIGN KEY (organization_id, category_id)
        REFERENCES categories (organization_id, id) ON DELETE CASCADE,
    CONSTRAINT fk_pt_org_machine FOREIGN KEY (organization_id, machine_id)
        REFERENCES machines (organization_id, id) ON DELETE CASCADE,
    CONSTRAINT fk_pt_org_site FOREIGN KEY (organization_id, site_id)
        REFERENCES sites (organization_id, id) ON DELETE CASCADE,
    CONSTRAINT fk_pt_organization_target FOREIGN KEY (organization_target_id)
        REFERENCES organizations (id) ON DELETE CASCADE,
    CONSTRAINT ck_promotion_targets_one_target CHECK (
        (target_type = 'product' AND product_id IS NOT NULL AND category_id IS NULL AND machine_id IS NULL AND site_id IS NULL AND organization_target_id IS NULL)
        OR (target_type = 'category' AND category_id IS NOT NULL AND product_id IS NULL AND machine_id IS NULL AND site_id IS NULL AND organization_target_id IS NULL)
        OR (target_type = 'machine' AND machine_id IS NOT NULL AND product_id IS NULL AND category_id IS NULL AND site_id IS NULL AND organization_target_id IS NULL)
        OR (target_type = 'site' AND site_id IS NOT NULL AND product_id IS NULL AND category_id IS NULL AND machine_id IS NULL AND organization_target_id IS NULL)
        OR (target_type = 'organization' AND organization_target_id IS NOT NULL AND product_id IS NULL AND category_id IS NULL AND machine_id IS NULL AND site_id IS NULL)
    )
);

CREATE INDEX ix_promotion_targets_promotion_id ON promotion_targets (promotion_id);
CREATE INDEX ix_promotion_targets_organization_id ON promotion_targets (organization_id);

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
    final_command_attempt_id uuid,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_vend_sessions_order_id ON vend_sessions (order_id);
CREATE INDEX ix_vend_sessions_machine_id ON vend_sessions (machine_id);
CREATE INDEX ix_vend_sessions_final_command_attempt ON vend_sessions (final_command_attempt_id)
    WHERE final_command_attempt_id IS NOT NULL;

CREATE TABLE settlement_batches (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    provider text NOT NULL,
    period_start date NOT NULL,
    period_end date NOT NULL,
    status text NOT NULL CHECK (status IN ('draft', 'open', 'processing', 'posted', 'failed', 'cancelled')),
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_settlement_batches_provider_period ON settlement_batches (provider, period_start, period_end);

COMMENT ON TABLE settlement_batches IS 'PSP settlement window; link payments via settlement_batch_id when batched.';

CREATE TABLE machine_reconciliation_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    business_date date NOT NULL,
    opened_at timestamptz NOT NULL,
    closed_at timestamptz,
    expected_cash_amount_minor bigint NOT NULL DEFAULT 0,
    actual_cash_amount_minor bigint NOT NULL DEFAULT 0,
    expected_digital_amount_minor bigint NOT NULL DEFAULT 0,
    actual_digital_amount_minor bigint NOT NULL DEFAULT 0,
    variance_amount_minor bigint NOT NULL DEFAULT 0,
    status text NOT NULL CHECK (status IN ('open', 'closed', 'variance_review', 'cancelled'))
);

CREATE UNIQUE INDEX ux_machine_reconciliation_sessions_open_per_day ON machine_reconciliation_sessions (machine_id, business_date)
    WHERE status = 'open';

CREATE INDEX ix_machine_reconciliation_sessions_machine_date ON machine_reconciliation_sessions (machine_id, business_date DESC);

COMMENT ON COLUMN machine_reconciliation_sessions.business_date IS 'Operator calendar day in organization TZ; store date only—resolve TZ in application.';
COMMENT ON COLUMN machine_reconciliation_sessions.variance_amount_minor IS 'actual - expected under session convention when closed.';

CREATE TABLE cash_collections (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    collected_at timestamptz NOT NULL,
    amount_minor bigint NOT NULL CHECK (amount_minor >= 0),
    currency char(3) NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    reconciliation_status text NOT NULL DEFAULT 'pending' CHECK (
        reconciliation_status IN ('pending', 'matched', 'mismatch', 'waived')
    ),
    reconciled_by text,
    reconciled_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    operator_session_id uuid REFERENCES machine_operator_sessions (id) ON DELETE SET NULL
);

CREATE INDEX ix_cash_collections_machine_collected ON cash_collections (machine_id, collected_at DESC);
CREATE INDEX ix_cash_collections_org_collected ON cash_collections (organization_id, collected_at DESC);
CREATE INDEX ix_cash_collections_unreconciled ON cash_collections (machine_id, collected_at DESC)
    WHERE reconciliation_status <> 'matched';
CREATE INDEX ix_cash_collections_operator_session ON cash_collections (operator_session_id)
    WHERE operator_session_id IS NOT NULL;

COMMENT ON TABLE cash_collections IS 'Physical cash removed from machine; reconcile against expected vault from cash_events.';
COMMENT ON COLUMN cash_collections.operator_session_id IS 'Operator session active during physical collection when recorded.';

CREATE TABLE cash_events (
    id bigserial PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    event_type text NOT NULL CHECK (
        event_type IN ('insert', 'dispense_change', 'reject', 'audit_adjust', 'transfer', 'other')
    ),
    amount_minor bigint NOT NULL,
    currency char(3) NOT NULL,
    occurred_at timestamptz NOT NULL,
    correlation_id uuid,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    reconciliation_session_id uuid REFERENCES machine_reconciliation_sessions (id) ON DELETE SET NULL
);

CREATE INDEX ix_cash_events_org_occurred ON cash_events (organization_id, occurred_at DESC);
CREATE INDEX ix_cash_events_machine_occurred ON cash_events (machine_id, occurred_at DESC);
CREATE INDEX ix_cash_events_session ON cash_events (reconciliation_session_id)
    WHERE reconciliation_session_id IS NOT NULL;
CREATE INDEX ix_cash_events_correlation ON cash_events (correlation_id, occurred_at DESC)
    WHERE correlation_id IS NOT NULL;

COMMENT ON TABLE cash_events IS 'Append-only cash movement log; application INSERT-only. amount_minor semantics per event_type in metadata or ops runbook.';

CREATE TABLE payments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id uuid NOT NULL REFERENCES orders (id) ON DELETE RESTRICT,
    provider text NOT NULL,
    state text NOT NULL CHECK (state IN ('created', 'authorized', 'captured', 'failed', 'refunded')),
    amount_minor bigint NOT NULL CHECK (amount_minor >= 0),
    currency char(3) NOT NULL,
    idempotency_key text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    reconciliation_status text NOT NULL DEFAULT 'pending' CHECK (
        reconciliation_status IN ('pending', 'matched', 'mismatch', 'not_required')
    ),
    settlement_status text NOT NULL DEFAULT 'unsettled' CHECK (
        settlement_status IN ('unsettled', 'batched', 'settled', 'written_off')
    ),
    settlement_batch_id uuid REFERENCES settlement_batches (id) ON DELETE SET NULL
);

CREATE UNIQUE INDEX ux_payments_order_idempotency ON payments (order_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL AND btrim(idempotency_key) <> '';

CREATE INDEX ix_payments_order_id ON payments (order_id);
CREATE INDEX ix_payments_reconciliation_queue ON payments (provider, updated_at DESC)
    WHERE reconciliation_status <> 'matched';
CREATE INDEX ix_payments_settlement_batch ON payments (settlement_batch_id)
    WHERE settlement_batch_id IS NOT NULL;

COMMENT ON COLUMN payments.reconciliation_status IS 'Provider vs internal ledger alignment; use payment_reconciliations for detail.';
COMMENT ON COLUMN payments.settlement_status IS 'PSP settlement lifecycle; settlement_batch_id when batched.';

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
    created_at timestamptz NOT NULL DEFAULT now(),
    reconciliation_status text NOT NULL DEFAULT 'pending' CHECK (
        reconciliation_status IN ('pending', 'matched', 'mismatch', 'not_required')
    ),
    settlement_status text NOT NULL DEFAULT 'unsettled' CHECK (
        settlement_status IN ('unsettled', 'batched', 'settled', 'written_off')
    ),
    settlement_batch_id uuid REFERENCES settlement_batches (id) ON DELETE SET NULL
);

CREATE INDEX ix_refunds_payment_id ON refunds (payment_id);
CREATE INDEX ix_refunds_order_id ON refunds (order_id);
CREATE INDEX ix_refunds_reconciliation_queue ON refunds (payment_id, created_at DESC)
    WHERE reconciliation_status <> 'matched';
CREATE INDEX ix_refunds_settlement_batch ON refunds (settlement_batch_id)
    WHERE settlement_batch_id IS NOT NULL;

CREATE TABLE payment_provider_events (
    id bigserial PRIMARY KEY,
    payment_id uuid REFERENCES payments (id) ON DELETE SET NULL,
    provider text NOT NULL,
    provider_ref text,
    provider_amount_minor bigint,
    currency char(3),
    event_type text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    received_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ux_payment_provider_events_provider_ref ON payment_provider_events (provider, provider_ref)
    WHERE provider_ref IS NOT NULL AND btrim(provider_ref) <> '';

CREATE INDEX ix_payment_provider_events_payment ON payment_provider_events (payment_id, received_at DESC)
    WHERE payment_id IS NOT NULL;
CREATE INDEX ix_payment_provider_events_received ON payment_provider_events (provider, received_at DESC);

COMMENT ON TABLE payment_provider_events IS 'Raw PSP notifications; payment_id nullable for orphan webhooks until correlated.';

CREATE TABLE payment_reconciliations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_id uuid NOT NULL REFERENCES payments (id) ON DELETE CASCADE,
    provider text NOT NULL,
    provider_ref text NOT NULL,
    provider_amount_minor bigint NOT NULL,
    internal_amount_minor bigint NOT NULL,
    currency char(3) NOT NULL,
    reconciled_at timestamptz NOT NULL,
    status text NOT NULL CHECK (status IN ('matched', 'mismatch', 'pending')),
    mismatch_reason text,
    CONSTRAINT ux_payment_reconciliations_provider_ref_payment UNIQUE (provider, provider_ref, payment_id)
);

CREATE INDEX ix_payment_reconciliations_payment_time ON payment_reconciliations (payment_id, reconciled_at DESC);
CREATE INDEX ix_payment_reconciliations_unmatched ON payment_reconciliations (provider, reconciled_at DESC)
    WHERE status IN ('pending', 'mismatch');

CREATE TABLE cash_reconciliations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    cash_session_id uuid,
    cash_collection_id uuid REFERENCES cash_collections (id) ON DELETE SET NULL,
    expected_amount_minor bigint NOT NULL,
    counted_amount_minor bigint NOT NULL,
    variance_amount_minor bigint NOT NULL,
    reconciled_at timestamptz NOT NULL,
    status text NOT NULL CHECK (status IN ('matched', 'mismatch', 'pending', 'review')),
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX ix_cash_reconciliations_machine_time ON cash_reconciliations (machine_id, reconciled_at DESC);
CREATE INDEX ix_cash_reconciliations_unmatched ON cash_reconciliations (machine_id, reconciled_at DESC)
    WHERE status IN ('pending', 'mismatch', 'review');

COMMENT ON COLUMN cash_reconciliations.cash_session_id IS 'Reserved for future cash_sessions table; no FK until introduced.';

CREATE TABLE financial_ledger_entries (
    id bigserial PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid REFERENCES machines (id) ON DELETE SET NULL,
    site_id uuid REFERENCES sites (id) ON DELETE SET NULL,
    order_id uuid REFERENCES orders (id) ON DELETE SET NULL,
    payment_id uuid REFERENCES payments (id) ON DELETE SET NULL,
    refund_id uuid REFERENCES refunds (id) ON DELETE SET NULL,
    cash_event_id bigint REFERENCES cash_events (id) ON DELETE SET NULL,
    cash_collection_id uuid REFERENCES cash_collections (id) ON DELETE SET NULL,
    entry_type text NOT NULL CHECK (
        entry_type IN (
            'order_created',
            'payment_authorized',
            'payment_captured',
            'payment_failed',
            'refund_issued',
            'cash_inserted',
            'change_dispensed',
            'cash_collected',
            'variance_recorded',
            'adjustment',
            'other'
        )
    ),
    signed_amount_minor bigint NOT NULL,
    currency char(3) NOT NULL,
    occurred_at timestamptz NOT NULL,
    reference_type text,
    reference_id uuid,
    correlation_id uuid,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX ix_financial_ledger_entries_org_time ON financial_ledger_entries (organization_id, occurred_at DESC);
CREATE INDEX ix_financial_ledger_entries_machine_time ON financial_ledger_entries (machine_id, occurred_at DESC)
    WHERE machine_id IS NOT NULL;
CREATE INDEX ix_financial_ledger_entries_payment ON financial_ledger_entries (payment_id)
    WHERE payment_id IS NOT NULL;
CREATE INDEX ix_financial_ledger_entries_order ON financial_ledger_entries (order_id)
    WHERE order_id IS NOT NULL;
CREATE INDEX ix_financial_ledger_entries_cash_event ON financial_ledger_entries (cash_event_id)
    WHERE cash_event_id IS NOT NULL;
CREATE INDEX ix_financial_ledger_entries_cash_collection ON financial_ledger_entries (cash_collection_id)
    WHERE cash_collection_id IS NOT NULL;
CREATE INDEX ix_financial_ledger_entries_correlation ON financial_ledger_entries (correlation_id, occurred_at DESC)
    WHERE correlation_id IS NOT NULL;

COMMENT ON TABLE financial_ledger_entries IS 'Append-only monetary fact stream; no updated_at. Application: INSERT only (revoke UPDATE for app role or enforce in repo).';
COMMENT ON COLUMN financial_ledger_entries.signed_amount_minor IS 'Signed minor units: positive = economic benefit to org (e.g. captured payment), negative = outflow (refund, change); document per entry_type in app.';
COMMENT ON COLUMN financial_ledger_entries.reference_type IS 'Polymorphic pointer when no dedicated FK column; prefer order_id/payment_id/cash_event_id when possible.';

CREATE TABLE command_ledger (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    sequence bigint NOT NULL,
    command_type text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    correlation_id uuid,
    idempotency_key text,
    created_at timestamptz NOT NULL DEFAULT now(),
    protocol_type text,
    deadline_at timestamptz,
    timeout_at timestamptz,
    attempt_count int NOT NULL DEFAULT 0,
    last_attempt_at timestamptz,
    route_key text,
    source_system text,
    source_event_id text,
    operator_session_id uuid REFERENCES machine_operator_sessions (id) ON DELETE SET NULL,
    CONSTRAINT ux_command_ledger_machine_sequence UNIQUE (machine_id, sequence)
);

CREATE UNIQUE INDEX ux_command_ledger_machine_idempotency ON command_ledger (machine_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL AND btrim(idempotency_key) <> '';

CREATE INDEX ix_command_ledger_machine_id ON command_ledger (machine_id);
CREATE INDEX ix_command_ledger_machine_created ON command_ledger (machine_id, created_at DESC);
CREATE INDEX ix_command_ledger_correlation_id ON command_ledger (correlation_id)
    WHERE correlation_id IS NOT NULL;
CREATE INDEX ix_command_ledger_operator_session ON command_ledger (operator_session_id)
    WHERE operator_session_id IS NOT NULL;

COMMENT ON COLUMN command_ledger.protocol_type IS 'Transport/protocol family, e.g. mqtt, dex, mcb, vendor_specific.';
COMMENT ON COLUMN command_ledger.deadline_at IS 'Business SLA deadline for command outcome.';
COMMENT ON COLUMN command_ledger.timeout_at IS 'Transport-layer timeout for acknowledgement.';
COMMENT ON COLUMN command_ledger.attempt_count IS 'Number of send attempts tracked in machine_command_attempts.';
COMMENT ON COLUMN command_ledger.last_attempt_at IS 'Timestamp of the latest machine_command_attempts row.';
COMMENT ON COLUMN command_ledger.route_key IS 'Broker shard / topic suffix for routing.';
COMMENT ON COLUMN command_ledger.source_system IS 'Upstream producer (outbox, webhook, admin UI, etc.).';
COMMENT ON COLUMN command_ledger.source_event_id IS 'Opaque id from source_system for cross-system trace.';
COMMENT ON TABLE command_ledger IS 'Authoritative machine command rows (sequence = device monotonic id). Trace: ledger -> machine_command_attempts -> transport/raw/ack -> device_command_receipts; correlate with vend_sessions / orders via correlation_id and time.';
COMMENT ON COLUMN command_ledger.operator_session_id IS 'This repo uses command_ledger as machine command rows (no separate machine_commands table).';

CREATE TABLE machine_modules (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    module_kind text NOT NULL CHECK (
        module_kind IN (
            'vend_motor',
            'bill_validator',
            'coin',
            'board',
            'remote',
            'display',
            'sensor',
            'other'
        )
    ),
    module_code text,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_machine_modules_module_code_nonempty CHECK (module_code IS NULL OR btrim(module_code) <> '')
);

CREATE UNIQUE INDEX ux_machine_modules_machine_kind_code ON machine_modules (machine_id, module_kind, module_code)
    WHERE module_code IS NOT NULL;

CREATE UNIQUE INDEX ux_machine_modules_machine_kind_default ON machine_modules (machine_id, module_kind)
    WHERE module_code IS NULL;

CREATE INDEX ix_machine_modules_machine_id ON machine_modules (machine_id);

COMMENT ON TABLE machine_modules IS 'Physical or logical sub-units on a machine (coin, motor bank, etc.).';

CREATE TABLE machine_transport_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    protocol_type text NOT NULL,
    transport_type text NOT NULL,
    client_id text,
    bridge_id text,
    connected_at timestamptz NOT NULL,
    disconnected_at timestamptz,
    disconnect_reason text,
    session_metadata jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX ix_machine_transport_sessions_machine_connected ON machine_transport_sessions (machine_id, connected_at DESC);
CREATE INDEX ix_machine_transport_sessions_active ON machine_transport_sessions (machine_id)
    WHERE disconnected_at IS NULL;

COMMENT ON COLUMN machine_transport_sessions.transport_type IS 'e.g. mqtt, websocket, serial_bridge.';
COMMENT ON TABLE machine_transport_sessions IS 'One logical connection from edge to cloud for correlation of attempts and raw frames.';

CREATE TABLE machine_command_attempts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    command_id uuid NOT NULL REFERENCES command_ledger (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    transport_session_id uuid REFERENCES machine_transport_sessions (id) ON DELETE SET NULL,
    attempt_no int NOT NULL CHECK (attempt_no >= 1),
    sent_at timestamptz NOT NULL,
    ack_deadline_at timestamptz,
    acked_at timestamptz,
    result_received_at timestamptz,
    status text NOT NULL CHECK (
        status IN (
            'pending',
            'sent',
            'ack_timeout',
            'nack',
            'completed',
            'failed',
            'duplicate',
            'late'
        )
    ),
    timeout_reason text,
    protocol_pack_no bigint,
    sequence_no bigint,
    correlation_id uuid,
    request_payload_json jsonb,
    raw_request bytea,
    raw_response bytea,
    latency_ms int,
    CONSTRAINT ux_machine_command_attempts_command_attempt UNIQUE (command_id, attempt_no)
);

CREATE INDEX ix_machine_command_attempts_command_attempt ON machine_command_attempts (command_id, attempt_no);
CREATE INDEX ix_machine_command_attempts_machine_sent ON machine_command_attempts (machine_id, sent_at DESC);
CREATE INDEX ix_machine_command_attempts_transport_sent ON machine_command_attempts (transport_session_id, sent_at DESC);
CREATE INDEX ix_machine_command_attempts_correlation ON machine_command_attempts (correlation_id)
    WHERE correlation_id IS NOT NULL;

COMMENT ON TABLE machine_command_attempts IS 'Per-send attempt for a command_ledger row; machine_id denormalized for index locality—must match parent command row (enforced in application).';
COMMENT ON COLUMN machine_command_attempts.raw_request IS 'Prefer bytea for binary protocols; use request_payload_json when parsed.';
COMMENT ON COLUMN machine_command_attempts.raw_response IS 'Raw wire-level response bytes when applicable.';

ALTER TABLE vend_sessions
    ADD CONSTRAINT fk_vend_sessions_final_command_attempt FOREIGN KEY (final_command_attempt_id)
        REFERENCES machine_command_attempts (id) ON DELETE SET NULL;

COMMENT ON COLUMN vend_sessions.correlation_id IS 'Cross-link to command_ledger.correlation_id and orders for payment→vend traces.';
COMMENT ON COLUMN vend_sessions.final_command_attempt_id IS 'Set when vend outcome is tied to a specific command attempt; NULL when inferred without command trace.';
COMMENT ON TABLE vend_sessions IS 'Field debug: payment ok but vend unclear—join orders/payments to machine_command_attempts and device_messages_raw by correlation_id and time window.';

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
    published_at timestamptz,
    publish_attempt_count integer NOT NULL DEFAULT 0,
    last_publish_error text,
    last_publish_attempt_at timestamptz,
    next_publish_after timestamptz,
    dead_lettered_at timestamptz
);

CREATE UNIQUE INDEX ux_outbox_topic_idempotency ON outbox_events (topic, idempotency_key)
    WHERE idempotency_key IS NOT NULL AND btrim(idempotency_key) <> '';

CREATE INDEX ix_outbox_unpublished ON outbox_events (created_at)
    WHERE published_at IS NULL;

CREATE INDEX ix_outbox_pending_due ON outbox_events (created_at, id)
WHERE
    published_at IS NULL
    AND dead_lettered_at IS NULL;

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

-- MQTT / edge ingest (migrations/00004_device_mqtt_ingest.sql, 00006_command_protocol_traceability.sql).
CREATE TABLE device_telemetry_events (
    id bigserial PRIMARY KEY,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    event_type text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    dedupe_key text,
    received_at timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE device_telemetry_events IS 'Legacy row-per-event table. At scale, route high-frequency telemetry through NATS + telemetry_rollups (see machine_current_snapshot / ops TELEMETRY_PIPELINE.md).';

CREATE UNIQUE INDEX ux_device_telemetry_dedupe ON device_telemetry_events (dedupe_key)
    WHERE dedupe_key IS NOT NULL AND btrim(dedupe_key) <> '';

CREATE INDEX ix_device_telemetry_machine_received ON device_telemetry_events (machine_id, received_at DESC);

CREATE TABLE device_command_receipts (
    id bigserial PRIMARY KEY,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    sequence bigint NOT NULL CHECK (sequence >= 0),
    status text NOT NULL,
    correlation_id uuid,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    dedupe_key text NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now(),
    command_attempt_id uuid,
    CONSTRAINT ux_device_command_receipts_dedupe UNIQUE (dedupe_key),
    CONSTRAINT fk_device_command_receipts_command_attempt FOREIGN KEY (command_attempt_id)
        REFERENCES machine_command_attempts (id) ON DELETE SET NULL
);

CREATE INDEX ix_device_command_receipts_machine_seq ON device_command_receipts (machine_id, sequence DESC);
CREATE INDEX ix_device_command_receipts_machine_received ON device_command_receipts (machine_id, received_at DESC);
CREATE INDEX ix_device_command_receipts_correlation ON device_command_receipts (correlation_id)
    WHERE correlation_id IS NOT NULL;
CREATE INDEX ix_device_command_receipts_command_attempt ON device_command_receipts (command_attempt_id)
    WHERE command_attempt_id IS NOT NULL;

COMMENT ON COLUMN device_command_receipts.command_attempt_id IS 'Optional link to the machine_command_attempts row this receipt answers.';
COMMENT ON TABLE device_command_receipts IS 'Device-reported outcome for a command sequence; optional command_attempt_id links to the send being acknowledged.';

-- Telemetry pipeline (migrations/00013_telemetry_pipeline.sql): rollups + snapshots, not raw high-frequency MQTT.
CREATE TABLE machine_current_snapshot (
    machine_id uuid PRIMARY KEY REFERENCES machines (id) ON DELETE CASCADE,
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    site_id uuid NOT NULL REFERENCES sites (id) ON DELETE CASCADE,
    reported_fingerprint text,
    metrics_fingerprint text,
    reported_state jsonb NOT NULL DEFAULT '{}'::jsonb,
    metrics_state jsonb NOT NULL DEFAULT '{}'::jsonb,
    last_heartbeat_at timestamptz,
    app_version text,
    firmware_version text,
    android_id text NULL,
    sim_serial text NULL,
    sim_iccid text NULL,
    device_model text NULL,
    os_version text NULL,
    last_identity_at timestamptz NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_machine_current_snapshot_org ON machine_current_snapshot (organization_id);

COMMENT ON TABLE machine_current_snapshot IS 'Single current row per machine; updated by telemetry state/metrics workers — not a raw ingest log.';

CREATE TABLE machine_state_transitions (
    id bigserial PRIMARY KEY,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    transition_key text NOT NULL,
    from_value jsonb,
    to_value jsonb NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    occurred_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_machine_state_transitions_machine_occurred ON machine_state_transitions (machine_id, occurred_at DESC);

COMMENT ON TABLE machine_state_transitions IS 'Append-only semantic transitions derived from shadow/state stream; pruned by retention job.';

CREATE TABLE machine_incidents (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    severity text NOT NULL,
    code text NOT NULL,
    title text,
    detail jsonb NOT NULL DEFAULT '{}'::jsonb,
    dedupe_key text,
    opened_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ux_machine_incidents_machine_dedupe ON machine_incidents (machine_id, dedupe_key)
WHERE
    dedupe_key IS NOT NULL
    AND btrim(dedupe_key) <> '';

CREATE INDEX ix_machine_incidents_machine_opened ON machine_incidents (machine_id, opened_at DESC);

COMMENT ON TABLE machine_incidents IS 'Operational/security incidents promoted from telemetry; not raw high-frequency logs.';

CREATE TABLE telemetry_rollups (
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    bucket_start timestamptz NOT NULL,
    granularity text NOT NULL CHECK (granularity IN ('1m', '1h')),
    metric_key text NOT NULL,
    sample_count bigint NOT NULL DEFAULT 0,
    sum_val double precision,
    min_val double precision,
    max_val double precision,
    last_val double precision,
    extra jsonb NOT NULL DEFAULT '{}'::jsonb,
    PRIMARY KEY (machine_id, bucket_start, granularity, metric_key)
);

CREATE INDEX ix_telemetry_rollups_machine_bucket ON telemetry_rollups (machine_id, bucket_start DESC);

COMMENT ON TABLE telemetry_rollups IS 'Aggregated telemetry; workers upsert buckets — raw MQTT metrics are not stored in Postgres.';

CREATE TABLE diagnostic_bundle_manifests (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    storage_key text NOT NULL,
    storage_provider text NOT NULL DEFAULT 's3',
    content_type text,
    size_bytes bigint,
    sha256_hex text,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz
);

CREATE INDEX ix_diagnostic_bundle_manifests_machine_created ON diagnostic_bundle_manifests (machine_id, created_at DESC);

COMMENT ON TABLE diagnostic_bundle_manifests IS 'Metadata for cold diagnostic bundles; blobs referenced by storage_key only.';

CREATE TABLE device_messages_raw (
    id bigserial PRIMARY KEY,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    module_id uuid REFERENCES machine_modules (id) ON DELETE SET NULL,
    transport_session_id uuid REFERENCES machine_transport_sessions (id) ON DELETE SET NULL,
    direction text NOT NULL CHECK (direction IN ('inbound', 'outbound')),
    protocol_type text NOT NULL,
    message_type text NOT NULL,
    correlation_id uuid,
    pack_no bigint,
    sequence_no bigint,
    payload_json jsonb,
    raw_payload bytea,
    message_hash bytea NOT NULL,
    occurred_at timestamptz NOT NULL
);

CREATE INDEX ix_device_messages_raw_machine_occurred ON device_messages_raw (machine_id, occurred_at DESC);
CREATE INDEX ix_device_messages_raw_correlation_occurred ON device_messages_raw (correlation_id, occurred_at DESC)
    WHERE correlation_id IS NOT NULL;
CREATE INDEX ix_device_messages_raw_transport_occurred ON device_messages_raw (transport_session_id, occurred_at DESC)
    WHERE transport_session_id IS NOT NULL;
CREATE INDEX ix_device_messages_raw_machine_proto_seq ON device_messages_raw (machine_id, protocol_type, pack_no, sequence_no)
    WHERE pack_no IS NOT NULL;
CREATE INDEX ix_device_messages_raw_message_hash ON device_messages_raw (machine_id, message_hash, occurred_at);

COMMENT ON TABLE device_messages_raw IS 'Immutable raw protocol log; prefer raw_payload bytea when JSON is not representative. Application: INSERT + SELECT only (no UPDATE). Dedup analysis via message_hash (non-unique).';
COMMENT ON COLUMN device_messages_raw.message_hash IS 'SHA-256 digest (32 bytes) of canonical wire bytes for forensics.';

CREATE TABLE protocol_ack_events (
    id bigserial PRIMARY KEY,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    command_attempt_id uuid REFERENCES machine_command_attempts (id) ON DELETE SET NULL,
    raw_message_id bigint REFERENCES device_messages_raw (id) ON DELETE SET NULL,
    device_receipt_id bigint REFERENCES device_command_receipts (id) ON DELETE SET NULL,
    event_type text NOT NULL CHECK (event_type IN ('ack', 'nack', 'timeout', 'retry_scheduled', 'inferred')),
    occurred_at timestamptz NOT NULL,
    latency_ms int,
    details jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX ix_protocol_ack_events_attempt_occurred ON protocol_ack_events (command_attempt_id, occurred_at);
CREATE INDEX ix_protocol_ack_events_machine_occurred ON protocol_ack_events (machine_id, occurred_at DESC);
CREATE INDEX ix_protocol_ack_events_raw_message ON protocol_ack_events (raw_message_id)
    WHERE raw_message_id IS NOT NULL;

COMMENT ON TABLE protocol_ack_events IS 'Low-level ack/nack/timeout for retry analysis; join to attempts, raw rows, or device_command_receipts.';

CREATE TABLE refill_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    started_at timestamptz NOT NULL DEFAULT now(),
    ended_at timestamptz,
    operator_session_id uuid REFERENCES machine_operator_sessions (id) ON DELETE SET NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_refill_sessions_machine_started ON refill_sessions (machine_id, started_at DESC);

CREATE INDEX ix_refill_sessions_org_started ON refill_sessions (organization_id, started_at DESC);

CREATE INDEX ix_refill_sessions_operator_session ON refill_sessions (operator_session_id)
WHERE
    operator_session_id IS NOT NULL;

COMMENT ON TABLE refill_sessions IS 'Field refill visit context; link operator_session_id for attribution.';

CREATE TABLE refill_session_lines (
    id bigserial PRIMARY KEY,
    refill_session_id uuid NOT NULL REFERENCES refill_sessions (id) ON DELETE CASCADE,
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    cabinet_code text NOT NULL,
    slot_code text NOT NULL,
    product_id uuid,
    before_quantity int NOT NULL,
    added_quantity int NOT NULL,
    after_quantity int NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_refill_session_lines_codes_nonempty CHECK (
        btrim(cabinet_code) <> ''
        AND btrim(slot_code) <> ''
    ),
    CONSTRAINT ck_refill_session_lines_nonneg CHECK (
        before_quantity >= 0
        AND after_quantity >= 0
    ),
    CONSTRAINT fk_refill_session_lines_org_product FOREIGN KEY (organization_id, product_id) REFERENCES products (organization_id, id) ON DELETE SET NULL
);

CREATE INDEX ix_refill_session_lines_session ON refill_session_lines (refill_session_id, created_at DESC);

COMMENT ON TABLE refill_session_lines IS 'Per-slot deltas recorded during a refill session; append-only.';

CREATE TABLE machine_configs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    applied_at timestamptz NOT NULL DEFAULT now(),
    config_revision int NOT NULL DEFAULT 1,
    config_payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    operator_session_id uuid REFERENCES machine_operator_sessions (id) ON DELETE SET NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_machine_configs_machine_applied ON machine_configs (machine_id, applied_at DESC);

CREATE INDEX ix_machine_configs_org_applied ON machine_configs (organization_id, applied_at DESC);

CREATE INDEX ix_machine_configs_operator_session ON machine_configs (operator_session_id)
WHERE
    operator_session_id IS NOT NULL;

COMMENT ON TABLE machine_configs IS 'Machine-local config application snapshots; operator_session_id when applied by a logged-in operator.';

CREATE TABLE incidents (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    status text NOT NULL DEFAULT 'open' CHECK (
        status IN ('open', 'acknowledged', 'in_progress', 'resolved', 'closed', 'cancelled')
    ),
    title text NOT NULL DEFAULT '',
    opened_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    operator_session_id uuid REFERENCES machine_operator_sessions (id) ON DELETE SET NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX ix_incidents_machine_updated ON incidents (machine_id, updated_at DESC);

CREATE INDEX ix_incidents_org_opened ON incidents (organization_id, opened_at DESC);

CREATE INDEX ix_incidents_operator_session ON incidents (operator_session_id)
WHERE
    operator_session_id IS NOT NULL;

COMMENT ON TABLE incidents IS 'Machine-side incidents; operator_session_id for operator-opened or last operator update when recorded.';

CREATE VIEW v_machine_current_operator AS
SELECT
    m.id AS machine_id,
    m.organization_id,
    s.id AS operator_session_id,
    s.actor_type,
    s.technician_id,
    t.display_name AS technician_display_name,
    s.user_principal,
    s.started_at AS session_started_at,
    s.status AS session_status,
    s.expires_at AS session_expires_at
FROM machines m
LEFT JOIN machine_operator_sessions s ON s.machine_id = m.id
    AND s.status = 'ACTIVE'
LEFT JOIN technicians t ON t.id = s.technician_id;

COMMENT ON VIEW v_machine_current_operator IS 'Convenience join for UI: one row per machine; operator_session_id NULL when nobody logged in. At most one ACTIVE session per machine is enforced by index ux_machine_operator_sessions_one_active.';

CREATE TABLE platform_auth_accounts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    email text NOT NULL,
    password_hash text NOT NULL,
    roles text[] NOT NULL DEFAULT '{}'::text[],
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ux_platform_auth_accounts_org_email ON platform_auth_accounts (organization_id, lower(email));

CREATE INDEX ix_platform_auth_accounts_organization_id ON platform_auth_accounts (organization_id);

CREATE TABLE auth_refresh_tokens (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id uuid NOT NULL REFERENCES platform_auth_accounts (id) ON DELETE CASCADE,
    token_hash bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz
);

CREATE INDEX ix_auth_refresh_tokens_account_created ON auth_refresh_tokens (account_id, created_at DESC);
CREATE UNIQUE INDEX ux_auth_refresh_tokens_active_hash ON auth_refresh_tokens (token_hash)
WHERE revoked_at IS NULL;

-- Multi-cabinet, assortments, inventory ledger (migrations/00015_machine_cabinets_assortments_inventory.sql).
CREATE TABLE machine_cabinets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    cabinet_code text NOT NULL,
    title text NOT NULL DEFAULT '',
    sort_order int NOT NULL DEFAULT 0,
    cabinet_index int NOT NULL DEFAULT 0,
    slot_capacity int CHECK (slot_capacity IS NULL OR slot_capacity >= 0),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'maintenance')),
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_machine_cabinets_cabinet_code_nonempty CHECK (btrim(cabinet_code) <> '')
);

CREATE UNIQUE INDEX ux_machine_cabinets_machine_cabinet_code ON machine_cabinets (machine_id, cabinet_code);

CREATE INDEX ix_machine_cabinets_machine_sort ON machine_cabinets (machine_id, sort_order ASC, id ASC);

CREATE TABLE assortments (
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

CREATE UNIQUE INDEX ux_assortments_org_id ON assortments (organization_id, id);

CREATE UNIQUE INDEX ux_assortments_org_name_lower ON assortments (organization_id, lower(name));

CREATE INDEX ix_assortments_organization_id ON assortments (organization_id);

CREATE TABLE assortment_items (
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

CREATE INDEX ix_assortment_items_assortment_sort ON assortment_items (assortment_id, sort_order ASC, id ASC);

CREATE INDEX ix_assortment_items_product_id ON assortment_items (product_id);

CREATE TABLE machine_assortment_bindings (
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

CREATE UNIQUE INDEX ux_machine_assortment_bindings_one_active_primary ON machine_assortment_bindings (machine_id)
WHERE
    is_primary
    AND valid_to IS NULL;

CREATE INDEX ix_machine_assortment_bindings_machine_valid_from ON machine_assortment_bindings (machine_id, valid_from DESC);

CREATE INDEX ix_machine_assortment_bindings_assortment ON machine_assortment_bindings (assortment_id);

CREATE TABLE inventory_count_sessions (
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

CREATE INDEX ix_inventory_count_sessions_machine_started ON inventory_count_sessions (machine_id, started_at DESC);

CREATE INDEX ix_inventory_count_sessions_org_started ON inventory_count_sessions (organization_id, started_at DESC);

CREATE TABLE inventory_events (
    id bigserial PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    machine_cabinet_id uuid REFERENCES machine_cabinets (id) ON DELETE SET NULL,
    cabinet_code text,
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
    reason_code text,
    quantity_before int,
    quantity_delta int NOT NULL,
    quantity_after int,
    unit_price_minor bigint NOT NULL DEFAULT 0,
    currency text NOT NULL DEFAULT 'USD',
    correlation_id uuid,
    operator_session_id uuid REFERENCES machine_operator_sessions (id) ON DELETE SET NULL,
    technician_id uuid REFERENCES technicians (id) ON DELETE SET NULL,
    refill_session_id uuid REFERENCES refill_sessions (id) ON DELETE SET NULL,
    inventory_count_session_id uuid REFERENCES inventory_count_sessions (id) ON DELETE SET NULL,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    recorded_at timestamptz NOT NULL DEFAULT now(),
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT fk_inventory_events_org_machine FOREIGN KEY (organization_id, machine_id) REFERENCES machines (organization_id, id) ON DELETE CASCADE,
    CONSTRAINT fk_inventory_events_org_product FOREIGN KEY (organization_id, product_id) REFERENCES products (organization_id, id) ON DELETE SET NULL,
    CONSTRAINT ck_inventory_events_slot_code_nonempty CHECK (slot_code IS NULL OR btrim(slot_code) <> ''),
    CONSTRAINT ck_inventory_events_cabinet_code_nonempty CHECK (cabinet_code IS NULL OR btrim(cabinet_code) <> '')
);

CREATE INDEX ix_inventory_events_machine_occurred ON inventory_events (machine_id, occurred_at DESC);

CREATE INDEX ix_inventory_events_org_occurred ON inventory_events (organization_id, occurred_at DESC);

CREATE INDEX ix_inventory_events_machine_slot_occurred ON inventory_events (machine_id, slot_code, occurred_at DESC)
WHERE
    slot_code IS NOT NULL;

CREATE INDEX ix_inventory_events_machine_product_occurred ON inventory_events (machine_id, product_id, occurred_at DESC)
WHERE
    product_id IS NOT NULL;

CREATE INDEX ix_inventory_events_correlation ON inventory_events (correlation_id, occurred_at DESC)
WHERE
    correlation_id IS NOT NULL;

CREATE INDEX ix_inventory_events_count_session ON inventory_events (inventory_count_session_id, occurred_at DESC)
WHERE
    inventory_count_session_id IS NOT NULL;

COMMENT ON TABLE machine_cabinets IS 'Logical cabinets on a machine; cabinet_code is stable within machine_id.';

COMMENT ON TABLE assortments IS 'Named product bundles for machine-specific merchandising.';

COMMENT ON TABLE assortment_items IS 'Products belonging to an assortment; sort_order drives UI and default sequencing.';

COMMENT ON TABLE machine_assortment_bindings IS 'Links machines to assortments; at most one active primary binding per machine (valid_to IS NULL, is_primary).';

COMMENT ON TABLE inventory_events IS 'Append-only inventory ledger; application INSERT-only.';

COMMENT ON TABLE inventory_count_sessions IS 'Optional physical count visit context; link operator_session_id when known.';

-- Slot layouts and configs (migrations/00016_machine_slot_layouts_configs.sql).
CREATE TABLE machine_slot_layouts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    machine_cabinet_id uuid NOT NULL REFERENCES machine_cabinets (id) ON DELETE CASCADE,
    layout_key text NOT NULL,
    revision int NOT NULL DEFAULT 1,
    layout_spec jsonb NOT NULL DEFAULT '{}'::jsonb,
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_machine_slot_layouts_layout_key_nonempty CHECK (btrim(layout_key) <> ''),
    CONSTRAINT ck_machine_slot_layouts_revision_positive CHECK (revision >= 1),
    CONSTRAINT fk_machine_slot_layouts_org_machine FOREIGN KEY (organization_id, machine_id) REFERENCES machines (organization_id, id) ON DELETE CASCADE,
    CONSTRAINT fk_machine_slot_layouts_machine_cabinet FOREIGN KEY (machine_cabinet_id) REFERENCES machine_cabinets (id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX ux_machine_slot_layouts_machine_cabinet_key_revision ON machine_slot_layouts (machine_id, machine_cabinet_id, layout_key, revision);

CREATE INDEX ix_machine_slot_layouts_machine_cabinet ON machine_slot_layouts (machine_id, machine_cabinet_id, created_at DESC);

CREATE INDEX ix_machine_slot_layouts_org ON machine_slot_layouts (organization_id, created_at DESC);

CREATE TABLE machine_slot_configs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    machine_cabinet_id uuid NOT NULL REFERENCES machine_cabinets (id) ON DELETE CASCADE,
    machine_slot_layout_id uuid NOT NULL REFERENCES machine_slot_layouts (id) ON DELETE RESTRICT,
    slot_code text NOT NULL,
    slot_index int CHECK (
        slot_index IS NULL
        OR slot_index >= 0
    ),
    product_id uuid,
    max_quantity int NOT NULL DEFAULT 0 CHECK (max_quantity >= 0),
    price_minor bigint NOT NULL DEFAULT 0 CHECK (price_minor >= 0),
    effective_from timestamptz NOT NULL DEFAULT now(),
    effective_to timestamptz,
    is_current boolean NOT NULL DEFAULT false,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_machine_slot_configs_slot_code_nonempty CHECK (btrim(slot_code) <> ''),
    CONSTRAINT fk_machine_slot_configs_org_machine FOREIGN KEY (organization_id, machine_id) REFERENCES machines (organization_id, id) ON DELETE CASCADE,
    CONSTRAINT fk_machine_slot_configs_machine_cabinet FOREIGN KEY (machine_cabinet_id) REFERENCES machine_cabinets (id) ON DELETE CASCADE,
    CONSTRAINT fk_machine_slot_configs_org_product FOREIGN KEY (organization_id, product_id) REFERENCES products (organization_id, id) ON DELETE SET NULL
);

CREATE UNIQUE INDEX ux_machine_slot_configs_current_machine_slot ON machine_slot_configs (machine_id, slot_code)
WHERE
    is_current;

CREATE INDEX ix_machine_slot_configs_machine_current ON machine_slot_configs (machine_id)
WHERE
    is_current;

CREATE INDEX ix_machine_slot_configs_layout ON machine_slot_configs (machine_slot_layout_id);

CREATE INDEX ix_machine_slot_configs_machine_cabinet_current ON machine_slot_configs (machine_cabinet_id)
WHERE
    is_current;

COMMENT ON TABLE machine_slot_layouts IS 'Cabinet-scoped slot grid / wiring metadata; layout_spec holds structured slot definitions.';

COMMENT ON TABLE machine_slot_configs IS 'Per-slot merchandising config; history via is_current / effective_*; at most one is_current row per (machine_id, slot_code).';

COMMENT ON INDEX ux_machine_slot_configs_current_machine_slot IS 'Partial unique: one current config row per physical slot_code on a machine.';
