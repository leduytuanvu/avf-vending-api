-- +goose Up
-- +goose StatementBegin

-- Deterministic UUIDs for local dev and integration tests (see internal/modules/postgres/integration_test.go).
INSERT INTO organizations (id, name, slug, status)
VALUES (
    '11111111-1111-1111-1111-111111111111',
    'Local Dev Org',
    'local-dev',
    'active'
);

INSERT INTO regions (id, organization_id, name, code)
VALUES (
    '22222222-2222-2222-2222-222222222222',
    '11111111-1111-1111-1111-111111111111',
    'HQ',
    'hq'
);

INSERT INTO sites (id, organization_id, region_id, name, address)
VALUES (
    '33333333-3333-3333-3333-333333333333',
    '11111111-1111-1111-1111-111111111111',
    '22222222-2222-2222-2222-222222222222',
    'Main DC',
    '{"city": "DevCity"}'::jsonb
);

INSERT INTO machine_hardware_profiles (id, organization_id, name, spec)
VALUES (
    '44444444-4444-4444-4444-444444444444',
    '11111111-1111-1111-1111-111111111111',
    'Generic VMC',
    '{"slots": 60}'::jsonb
);

INSERT INTO machines (id, organization_id, site_id, hardware_profile_id, serial_number, name, status, command_sequence)
VALUES (
    '55555555-5555-5555-5555-555555555555',
    '11111111-1111-1111-1111-111111111111',
    '33333333-3333-3333-3333-333333333333',
    '44444444-4444-4444-4444-444444444444',
    'SN-DEV-001',
    'Dev Machine 1',
    'online',
    0
);

INSERT INTO technicians (id, organization_id, display_name, email)
VALUES (
    '66666666-6666-6666-6666-666666666666',
    '11111111-1111-1111-1111-111111111111',
    'Pat Technician',
    'pat@example.com'
);

INSERT INTO technician_machine_assignments (technician_id, machine_id, role)
VALUES (
    '66666666-6666-6666-6666-666666666666',
    '55555555-5555-5555-5555-555555555555',
    'maintainer'
);

INSERT INTO products (id, organization_id, sku, name, description, active)
VALUES
    (
        'aaaaaaaa-aaaa-aaaa-aaaa-000000000001',
        '11111111-1111-1111-1111-111111111111',
        'SKU-COLA',
        'Cola 330ml',
        'Carbonated beverage',
        true
    ),
    (
        'aaaaaaaa-aaaa-aaaa-aaaa-000000000002',
        '11111111-1111-1111-1111-111111111111',
        'SKU-WATER',
        'Still water 500ml',
        'Water',
        true
    );

INSERT INTO price_books (id, organization_id, name, currency, effective_from, is_default)
VALUES (
    'bbbbbbbb-bbbb-bbbb-bbbb-000000000001',
    '11111111-1111-1111-1111-111111111111',
    'Default USD',
    'USD',
    now(),
    true
);

INSERT INTO price_book_items (price_book_id, product_id, unit_price_minor)
VALUES
    ('bbbbbbbb-bbbb-bbbb-bbbb-000000000001', 'aaaaaaaa-aaaa-aaaa-aaaa-000000000001', 150),
    ('bbbbbbbb-bbbb-bbbb-bbbb-000000000001', 'aaaaaaaa-aaaa-aaaa-aaaa-000000000002', 120);

INSERT INTO planograms (id, organization_id, name, revision, status, meta)
VALUES (
    'cccccccc-cccc-cccc-cccc-000000000001',
    '11111111-1111-1111-1111-111111111111',
    'Default Planogram',
    1,
    'published',
    '{}'::jsonb
);

INSERT INTO slots (planogram_id, slot_index, product_id, max_quantity)
VALUES
    ('cccccccc-cccc-cccc-cccc-000000000001', 0, 'aaaaaaaa-aaaa-aaaa-aaaa-000000000001', 10),
    ('cccccccc-cccc-cccc-cccc-000000000001', 1, 'aaaaaaaa-aaaa-aaaa-aaaa-000000000002', 10);

INSERT INTO machine_slot_state (machine_id, planogram_id, slot_index, current_quantity, price_minor, planogram_revision_applied)
VALUES
    ('55555555-5555-5555-5555-555555555555', 'cccccccc-cccc-cccc-cccc-000000000001', 0, 5, 150, 1),
    ('55555555-5555-5555-5555-555555555555', 'cccccccc-cccc-cccc-cccc-000000000001', 1, 8, 120, 1);

INSERT INTO machine_shadow (machine_id, desired_state, reported_state, version)
VALUES (
    '55555555-5555-5555-5555-555555555555',
    '{"planogram_id": "cccccccc-cccc-cccc-cccc-000000000001"}'::jsonb,
    '{"temperature_c": 4}'::jsonb,
    1
);

INSERT INTO ota_artifacts (id, organization_id, storage_key, sha256, size_bytes, semver)
VALUES (
    'dddddddd-dddd-dddd-dddd-000000000001',
    '11111111-1111-1111-1111-111111111111',
    'dev/firmware/1.0.0.bin',
    repeat('0', 64),
    1024,
    '1.0.0'
);

INSERT INTO ota_campaigns (id, organization_id, name, artifact_id, strategy, status)
VALUES (
    'eeeeeeee-eeee-eeee-eeee-000000000001',
    '11111111-1111-1111-1111-111111111111',
    'Pilot rollout',
    'dddddddd-dddd-dddd-dddd-000000000001',
    'rolling',
    'draft'
);

INSERT INTO ota_targets (campaign_id, machine_id, state)
VALUES (
    'eeeeeeee-eeee-eeee-eeee-000000000001',
    '55555555-5555-5555-5555-555555555555',
    'pending'
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DELETE FROM ota_targets WHERE campaign_id = 'eeeeeeee-eeee-eeee-eeee-000000000001';
DELETE FROM ota_campaigns WHERE id = 'eeeeeeee-eeee-eeee-eeee-000000000001';
DELETE FROM ota_artifacts WHERE id = 'dddddddd-dddd-dddd-dddd-000000000001';
DELETE FROM machine_shadow WHERE machine_id = '55555555-5555-5555-5555-555555555555';
DELETE FROM machine_slot_state WHERE machine_id = '55555555-5555-5555-5555-555555555555';
DELETE FROM slots WHERE planogram_id = 'cccccccc-cccc-cccc-cccc-000000000001';
DELETE FROM planograms WHERE id = 'cccccccc-cccc-cccc-cccc-000000000001';
DELETE FROM price_book_items WHERE price_book_id = 'bbbbbbbb-bbbb-bbbb-bbbb-000000000001';
DELETE FROM price_books WHERE id = 'bbbbbbbb-bbbb-bbbb-bbbb-000000000001';
DELETE FROM products WHERE organization_id = '11111111-1111-1111-1111-111111111111';
DELETE FROM technician_machine_assignments WHERE technician_id = '66666666-6666-6666-6666-666666666666';
DELETE FROM technicians WHERE id = '66666666-6666-6666-6666-666666666666';
DELETE FROM machines WHERE id = '55555555-5555-5555-5555-555555555555';
DELETE FROM machine_hardware_profiles WHERE id = '44444444-4444-4444-4444-444444444444';
DELETE FROM sites WHERE id = '33333333-3333-3333-3333-333333333333';
DELETE FROM regions WHERE id = '22222222-2222-2222-2222-222222222222';
DELETE FROM organizations WHERE id = '11111111-1111-1111-1111-111111111111';

-- +goose StatementEnd
