-- Layout dimension migration verification (run after ClassifyLegacyLayoutDimensions).
-- Each query should return zero rows unless noted.

-- 1. Classification coverage: every layout must land in exactly one class.
SELECT class, count(*) AS layout_count
FROM layout_dimension_migration_audit
GROUP BY class
ORDER BY class;

-- 2. No layout silently defaulted while still marked REQUIRES_REVIEW.
SELECT count(*) AS wrongly_defaulted
FROM machine_slot_layouts l
JOIN layout_dimension_migration_audit a ON a.machine_slot_layout_id = l.id
WHERE a.class = 'REQUIRES_REVIEW'
  AND (l.grid_rows IS NOT NULL OR l.grid_cols IS NOT NULL);

-- 3. Backfilled dimensions must contain every existing slot ordinal.
SELECT l.id, l.grid_rows, l.grid_cols, max(c.slot_index) AS max_ordinal
FROM machine_slot_layouts l
JOIN machine_slot_configs c ON c.machine_slot_layout_id = l.id
WHERE l.grid_rows IS NOT NULL
GROUP BY l.id, l.grid_rows, l.grid_cols
HAVING max(c.slot_index) > l.grid_rows * l.grid_cols;

-- 4. Core invariant: never more than one current assignment per (machine, source).
SELECT machine_id, source, count(*) AS current_count
FROM machine_layout_assignments
WHERE is_current
GROUP BY machine_id, source
HAVING count(*) > 1;

-- 5. Reported revision monotonicity sanity.
SELECT machine_id
FROM machine_layout_state
WHERE reported_revision IS NOT NULL
  AND reported_revision < 1;

-- 6. Layouts still missing dimensions after classification.
SELECT count(*) AS layouts_missing_dimensions
FROM machine_slot_layouts
WHERE grid_rows IS NULL OR grid_cols IS NULL;
