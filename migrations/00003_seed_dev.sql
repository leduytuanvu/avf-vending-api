-- +goose Up

-- Deterministic UUIDs for local dev and integration tests (see internal/modules/postgres/integration_test.go).

INSERT INTO regions (id, name, code)
VALUES (
    '22222222-2222-2222-2222-222222222222',
    'HQ',
    'hq'
);

INSERT INTO sites (id, region_id, name, address, code)
VALUES (
    '33333333-3333-3333-3333-333333333333',
    '22222222-2222-2222-2222-222222222222',
    'Main DC',
    '{"city": "DevCity"}'::jsonb,
    'main-dc'
);

INSERT INTO machine_hardware_profiles (id, name, spec)
VALUES (
    '44444444-4444-4444-4444-444444444444',
    'Generic VMC',
    '{"slots": 60}'::jsonb
);

INSERT INTO machines (id, site_id, hardware_profile_id, serial_number, name, status, command_sequence)
VALUES (
    '55555555-5555-5555-5555-555555555555',
    '33333333-3333-3333-3333-333333333333',
    '44444444-4444-4444-4444-444444444444',
    'SN-DEV-001',
    'Dev Machine 1',
    'active',
    0
);

INSERT INTO technicians (id, display_name, email)
VALUES (
    '66666666-6666-6666-6666-666666666666',
    'Pat Technician',
    'pat@example.com'
);

INSERT INTO technician_machine_assignments (id, technician_id, machine_id, role)
VALUES (
    '77777777-7777-7777-7777-777777777777',
    '66666666-6666-6666-6666-666666666666',
    '55555555-5555-5555-5555-555555555555',
    'maintainer'
);

INSERT INTO products (id, sku, name, description, active)
VALUES
    (
        'aaaaaaaa-aaaa-aaaa-aaaa-000000000001',
        'SKU-COLA',
        'Cola 330ml',
        'Carbonated beverage',
        true
    ),
    (
        'aaaaaaaa-aaaa-aaaa-aaaa-000000000002',
        'SKU-WATER',
        'Still water 500ml',
        'Water',
        true
    );

INSERT INTO price_books (id, name, currency, effective_from, is_default, active, price_book_level)
VALUES (
    'bbbbbbbb-bbbb-bbbb-bbbb-000000000001',
    'Default USD',
    'USD',
    now(),
    true,
    true,
    'global'
);

INSERT INTO price_book_items (price_book_id, product_id, unit_price_minor)
VALUES
    ('bbbbbbbb-bbbb-bbbb-bbbb-000000000001', 'aaaaaaaa-aaaa-aaaa-aaaa-000000000001', 150),
    ('bbbbbbbb-bbbb-bbbb-bbbb-000000000001', 'aaaaaaaa-aaaa-aaaa-aaaa-000000000002', 120);

INSERT INTO planograms (id, name, revision, status, meta)
VALUES (
    'cccccccc-cccc-cccc-cccc-000000000001',
    'Default Planogram',
    1,
    'published',
    '{}'::jsonb
);

INSERT INTO slots (id, planogram_id, slot_index, product_id, max_quantity)
VALUES
    ('ffffffff-ffff-ffff-ffff-000000000001', 'cccccccc-cccc-cccc-cccc-000000000001', 0, 'aaaaaaaa-aaaa-aaaa-aaaa-000000000001', 10),
    ('ffffffff-ffff-ffff-ffff-000000000002', 'cccccccc-cccc-cccc-cccc-000000000001', 1, 'aaaaaaaa-aaaa-aaaa-aaaa-000000000002', 10);

INSERT INTO machine_slot_state (id, machine_id, planogram_id, slot_index, current_quantity, price_minor, planogram_revision_applied)
VALUES
    ('99999999-9999-9999-9999-999999999901', '55555555-5555-5555-5555-555555555555', 'cccccccc-cccc-cccc-cccc-000000000001', 0, 5, 150, 1),
    ('99999999-9999-9999-9999-999999999902', '55555555-5555-5555-5555-555555555555', 'cccccccc-cccc-cccc-cccc-000000000001', 1, 8, 120, 1);

INSERT INTO machine_shadow (machine_id, desired_state, reported_state, version)
VALUES (
    '55555555-5555-5555-5555-555555555555',
    '{"planogram_id": "cccccccc-cccc-cccc-cccc-000000000001"}'::jsonb,
    '{"temperature_c": 4}'::jsonb,
    1
);

INSERT INTO ota_artifacts (id, storage_key, sha256, size_bytes, semver)
VALUES (
    'dddddddd-dddd-dddd-dddd-000000000001',
    'dev/firmware/1.0.0.bin',
    repeat('0', 64),
    1024,
    '1.0.0'
);

INSERT INTO ota_campaigns (id, name, artifact_id, rollout_strategy, status)
VALUES (
    'eeeeeeee-eeee-eeee-eeee-000000000001',
    'Pilot rollout',
    'dddddddd-dddd-dddd-dddd-000000000001',
    'rolling',
    'draft'
);

INSERT INTO ota_campaign_targets (campaign_id, machine_id, state)
VALUES (
    'eeeeeeee-eeee-eeee-eeee-000000000001',
    '55555555-5555-5555-5555-555555555555',
    'pending'
);

-- +goose Down

DELETE FROM ota_campaign_targets WHERE campaign_id = 'eeeeeeee-eeee-eeee-eeee-000000000001';
DELETE FROM ota_campaigns WHERE id = 'eeeeeeee-eeee-eeee-eeee-000000000001';
DELETE FROM ota_artifacts WHERE id = 'dddddddd-dddd-dddd-dddd-000000000001';
DELETE FROM machine_shadow WHERE machine_id = '55555555-5555-5555-5555-555555555555';
DELETE FROM machine_slot_state WHERE machine_id = '55555555-5555-5555-5555-555555555555';
DELETE FROM slots WHERE planogram_id = 'cccccccc-cccc-cccc-cccc-000000000001';
DELETE FROM planograms WHERE id = 'cccccccc-cccc-cccc-cccc-000000000001';
DELETE FROM price_book_items WHERE price_book_id = 'bbbbbbbb-bbbb-bbbb-bbbb-000000000001';
DELETE FROM price_books WHERE id = 'bbbbbbbb-bbbb-bbbb-bbbb-000000000001';
DELETE FROM products WHERE sku IN ('SKU-COLA', 'SKU-WATER');
DELETE FROM technician_machine_assignments WHERE id = '77777777-7777-7777-7777-777777777777';
DELETE FROM technicians WHERE id = '66666666-6666-6666-6666-666666666666';
DELETE FROM machines WHERE id = '55555555-5555-5555-5555-555555555555';
DELETE FROM machine_hardware_profiles WHERE id = '44444444-4444-4444-4444-444444444444';
DELETE FROM sites WHERE id = '33333333-3333-3333-3333-333333333333';
DELETE FROM regions WHERE id = '22222222-2222-2222-2222-222222222222';
