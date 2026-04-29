-- name: PricingRuntimeListMachineOverridesAt :many
-- Latest active per-machine price override per product at evaluation time.
SELECT DISTINCT ON (product_id)
    product_id,
    unit_price_minor,
    currency
FROM machine_price_overrides
WHERE organization_id = $1
  AND machine_id = $2
  AND valid_from <= sqlc.arg('eval_at')::timestamptz
  AND (valid_to IS NULL OR valid_to > sqlc.arg('eval_at')::timestamptz)
ORDER BY product_id, valid_from DESC;
