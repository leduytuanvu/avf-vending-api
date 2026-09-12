-- Idempotent backfill: create Layout 1 for machines missing named layouts.
-- Precedence: device mirror JSON > current slot configs > default 6x10 grid.
-- +goose Up
-- +goose StatementBegin

INSERT INTO machine_layouts (
    machine_id,
    name,
    status,
    grid_rows,
    grid_cols,
    layout_revision,
    fingerprint
)
SELECT
    m.id,
    'Layout 1',
    'READY',
    COALESCE(NULLIF(mirror.grid_rows, 0), 6),
    COALESCE(NULLIF(mirror.grid_cols, 0), 10),
    GREATEST(COALESCE(mirror.revision, 1), 1),
    COALESCE(NULLIF(btrim(mirror.fingerprint), ''), 'backfill:v1')
FROM machines m
LEFT JOIN LATERAL (
    SELECT ml.grid_rows, ml.grid_cols, ml.revision, ml.fingerprint
    FROM machine_local_layout_mirror ml
    WHERE ml.machine_id = m.id
    ORDER BY ml.reported_at DESC
    LIMIT 1
) mirror ON true
WHERE NOT EXISTS (
    SELECT 1
    FROM machine_layouts existing
    WHERE existing.machine_id = m.id
        AND NOT existing.is_archived
);

INSERT INTO machine_layout_slots (
    layout_id,
    slot_code,
    slot_ordinal,
    slot_row,
    slot_column,
    physical_lane,
    product_id,
    max_quantity,
    price_minor,
    current_inventory,
    enabled,
    operational_state
)
SELECT
    ml.id,
    slot.slot_code,
    slot.slot_ordinal,
    slot.slot_row,
    slot.slot_column,
    slot.physical_lane,
    slot.product_id,
    COALESCE(slot.max_quantity, 6),
    slot.price_minor,
    COALESCE(slot.current_inventory, 0),
    COALESCE(slot.enabled, true),
    COALESCE(NULLIF(btrim(slot.operational_state), ''), 'unassigned')
FROM machine_layouts ml
JOIN machines m ON m.id = ml.machine_id
JOIN LATERAL (
    SELECT
        COALESCE(NULLIF(btrim(elem->>'slotCode'), ''), NULLIF(btrim(elem->>'slot_code'), '')) AS slot_code,
        COALESCE((elem->>'slotOrdinal')::int, (elem->>'slot_ordinal')::int, row_number() OVER ())::int AS slot_ordinal,
        NULL::int AS slot_row,
        NULL::int AS slot_column,
        COALESCE((elem->>'physicalLane')::int, (elem->>'physical_lane')::int) AS physical_lane,
        NULLIF(btrim(elem->>'productId'), '')::uuid AS product_id,
        COALESCE((elem->>'maxQuantity')::int, (elem->>'max_quantity')::int, 6) AS max_quantity,
        COALESCE((elem->>'priceMinor')::bigint, (elem->>'price_minor')::bigint) AS price_minor,
        COALESCE((elem->>'currentInventory')::int, (elem->>'current_inventory')::int, 0) AS current_inventory,
        COALESCE((elem->>'enabled')::boolean, true) AS enabled,
        COALESCE(NULLIF(btrim(elem->>'operationalState'), ''), NULLIF(btrim(elem->>'operational_state'), ''), 'unassigned') AS operational_state
    FROM machine_local_layout_mirror mirror
    CROSS JOIN LATERAL jsonb_array_elements(mirror.slots::jsonb) AS elem
    WHERE mirror.machine_id = m.id
) slot ON true
WHERE ml.name = 'Layout 1'
    AND NOT ml.is_archived
    AND NOT EXISTS (
        SELECT 1 FROM machine_layout_slots existing WHERE existing.layout_id = ml.id
    )
    AND slot.slot_code IS NOT NULL;

UPDATE machines m
SET
    active_layout_id = ml.id,
    desired_active_layout_id = ml.id,
    reported_active_layout_id = ml.id,
    updated_at = now()
FROM machine_layouts ml
WHERE ml.machine_id = m.id
    AND ml.name = 'Layout 1'
    AND NOT ml.is_archived
    AND m.active_layout_id IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Backfill is additive; down migration intentionally no-op to preserve named layouts.
-- +goose StatementEnd
