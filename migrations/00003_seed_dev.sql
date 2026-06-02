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
    '2020-01-01T00:00:00Z'::timestamptz,
    true,
    true,
    'global'
);

INSERT INTO price_book_items (price_book_id, product_id, unit_price_minor)
VALUES
    ('bbbbbbbb-bbbb-bbbb-bbbb-000000000001', 'aaaaaaaa-aaaa-aaaa-aaaa-000000000001', 150),
    ('bbbbbbbb-bbbb-bbbb-bbbb-000000000001', 'aaaaaaaa-aaaa-aaaa-aaaa-000000000002', 120);

-- Ready primary media for dev catalog products (sell-readiness / slot publish gates).
INSERT INTO media_assets (
    id, kind, original_object_key, thumb_object_key, display_object_key,
    source_type, original_url, mime_type, size_bytes, status
)
VALUES
    (
        'dddddddd-dddd-dddd-dddd-000000000101',
        'product_image',
        'dev/media/cola/original.png',
        'dev/media/cola/thumb.png',
        'dev/media/cola/display.png',
        'external',
        'https://example.test/dev/cola.png',
        'image/png',
        1024,
        'ready'
    ),
    (
        'dddddddd-dddd-dddd-dddd-000000000102',
        'product_image',
        'dev/media/water/original.png',
        'dev/media/water/thumb.png',
        'dev/media/water/display.png',
        'external',
        'https://example.test/dev/water.png',
        'image/png',
        1024,
        'ready'
    );

INSERT INTO product_images (
    id, product_id, storage_key, cdn_url, thumb_cdn_url, is_primary, media_asset_id, status
)
VALUES
    (
        'dddddddd-dddd-dddd-dddd-000000000201',
        'aaaaaaaa-aaaa-aaaa-aaaa-000000000001',
        'dev/media/cola/original.png',
        'https://example.test/dev/cola.png',
        'https://example.test/dev/cola-thumb.png',
        true,
        'dddddddd-dddd-dddd-dddd-000000000101',
        'active'
    ),
    (
        'dddddddd-dddd-dddd-dddd-000000000202',
        'aaaaaaaa-aaaa-aaaa-aaaa-000000000002',
        'dev/media/water/original.png',
        'https://example.test/dev/water.png',
        'https://example.test/dev/water-thumb.png',
        true,
        'dddddddd-dddd-dddd-dddd-000000000102',
        'active'
    );

INSERT INTO product_media (
    id, product_id, media_type, source_type, original_url, display_url, thumb_url,
    mime_type, size_bytes, status
)
VALUES
    (
        'dddddddd-dddd-dddd-dddd-000000000201',
        'aaaaaaaa-aaaa-aaaa-aaaa-000000000001',
        'image',
        'external',
        'https://example.test/dev/cola.png',
        'https://example.test/dev/cola.png',
        'https://example.test/dev/cola-thumb.png',
        'image/png',
        1024,
        'active'
    ),
    (
        'dddddddd-dddd-dddd-dddd-000000000202',
        'aaaaaaaa-aaaa-aaaa-aaaa-000000000002',
        'image',
        'external',
        'https://example.test/dev/water.png',
        'https://example.test/dev/water.png',
        'https://example.test/dev/water-thumb.png',
        'image/png',
        1024,
        'active'
    );

UPDATE products
SET primary_image_id = 'dddddddd-dddd-dddd-dddd-000000000201'
WHERE id = 'aaaaaaaa-aaaa-aaaa-aaaa-000000000001';

UPDATE products
SET primary_image_id = 'dddddddd-dddd-dddd-dddd-000000000202'
WHERE id = 'aaaaaaaa-aaaa-aaaa-aaaa-000000000002';

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
UPDATE products SET primary_image_id = NULL WHERE id IN (
    'aaaaaaaa-aaaa-aaaa-aaaa-000000000001',
    'aaaaaaaa-aaaa-aaaa-aaaa-000000000002'
);
DELETE FROM product_media WHERE id IN (
    'dddddddd-dddd-dddd-dddd-000000000201',
    'dddddddd-dddd-dddd-dddd-000000000202'
);
DELETE FROM product_images WHERE id IN (
    'dddddddd-dddd-dddd-dddd-000000000201',
    'dddddddd-dddd-dddd-dddd-000000000202'
);
DELETE FROM media_assets WHERE id IN (
    'dddddddd-dddd-dddd-dddd-000000000101',
    'dddddddd-dddd-dddd-dddd-000000000102'
);
DELETE FROM price_book_items WHERE price_book_id = 'bbbbbbbb-bbbb-bbbb-bbbb-000000000001';
DELETE FROM price_books WHERE id = 'bbbbbbbb-bbbb-bbbb-bbbb-000000000001';
DELETE FROM products WHERE sku IN ('SKU-COLA', 'SKU-WATER');
DELETE FROM technician_machine_assignments WHERE id = '77777777-7777-7777-7777-777777777777';
DELETE FROM technicians WHERE id = '66666666-6666-6666-6666-666666666666';
DELETE FROM machines WHERE id = '55555555-5555-5555-5555-555555555555';
DELETE FROM machine_hardware_profiles WHERE id = '44444444-4444-4444-4444-444444444444';
DELETE FROM sites WHERE id = '33333333-3333-3333-3333-333333333333';
DELETE FROM regions WHERE id = '22222222-2222-2222-2222-222222222222';
