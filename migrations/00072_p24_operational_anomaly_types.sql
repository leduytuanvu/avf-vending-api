-- +goose Up
-- P2.4: extend inventory_anomalies with operational / predictive restock detector types (same dedupe model: open fingerprint per machine).
DO $$
DECLARE
    con_name text;
BEGIN
    SELECT
        c.conname INTO con_name
    FROM
        pg_constraint c
        INNER JOIN pg_class rel ON rel.oid = c.conrelid
            AND rel.relname = 'inventory_anomalies'
    WHERE
        c.contype = 'c'
        AND pg_get_constraintdef(c.oid) LIKE '%anomaly_type%'
    LIMIT 1;
    IF con_name IS NOT NULL THEN
        EXECUTE format('ALTER TABLE inventory_anomalies DROP CONSTRAINT %I', con_name);
    END IF;
END
$$;

ALTER TABLE inventory_anomalies
ADD CONSTRAINT inventory_anomalies_anomaly_type_check CHECK (
    anomaly_type IN (
        'negative_stock',
        'stock_mismatch_after_fill',
        'vend_without_stock_decrement',
        'manual_adjustment_above_threshold',
        'stale_inventory_sync',
        'slot_missing_product_but_stock',
        'machine_offline_too_long',
        'repeated_vend_failure',
        'repeated_payment_failure',
        'stock_mismatch',
        'negative_stock_attempt',
        'high_cash_variance',
        'command_failure_spike',
        'telemetry_missing',
        'low_stock_threshold',
        'product_sold_out_soon_estimate'
    )
);

COMMENT ON TABLE inventory_anomalies IS 'Operator-visible machine anomalies (inventory + operational detectors); open rows deduped by (machine_id, fingerprint); resolve/ignore closes rows for audit trails.';

-- +goose Down
DO $$
DECLARE
    con_name text;
BEGIN
    SELECT
        c.conname INTO con_name
    FROM
        pg_constraint c
        INNER JOIN pg_class rel ON rel.oid = c.conrelid
            AND rel.relname = 'inventory_anomalies'
    WHERE
        c.contype = 'c'
        AND c.conname = 'inventory_anomalies_anomaly_type_check'
    LIMIT 1;
    IF con_name IS NOT NULL THEN
        EXECUTE format('ALTER TABLE inventory_anomalies DROP CONSTRAINT %I', con_name);
    END IF;
END
$$;

ALTER TABLE inventory_anomalies
ADD CONSTRAINT inventory_anomalies_anomaly_type_check CHECK (
    anomaly_type IN (
        'negative_stock',
        'stock_mismatch_after_fill',
        'vend_without_stock_decrement',
        'manual_adjustment_above_threshold',
        'stale_inventory_sync',
        'slot_missing_product_but_stock'
    )
);

COMMENT ON TABLE inventory_anomalies IS 'Operator-visible inventory anomaly rows; detectors upsert open fingerprints; resolve closes rows for audit trails.';
