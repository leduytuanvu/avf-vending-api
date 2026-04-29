-- +goose Up
CREATE TABLE inventory_anomalies (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    anomaly_type text NOT NULL CHECK (
        anomaly_type IN (
            'negative_stock',
            'stock_mismatch_after_fill',
            'vend_without_stock_decrement',
            'manual_adjustment_above_threshold',
            'stale_inventory_sync',
            'slot_missing_product_but_stock'
        )
    ),
    status text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'resolved', 'ignored')),
    fingerprint text NOT NULL,
    slot_code text,
    product_id uuid,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    detected_at timestamptz NOT NULL DEFAULT now (),
    resolved_at timestamptz,
    resolved_by uuid REFERENCES platform_auth_accounts (id) ON DELETE SET NULL,
    resolution_note text,
    created_at timestamptz NOT NULL DEFAULT now (),
    updated_at timestamptz NOT NULL DEFAULT now (),
    CONSTRAINT fk_inventory_anomalies_org_machine FOREIGN KEY (organization_id, machine_id) REFERENCES machines (organization_id, id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX ux_inventory_anomalies_machine_fp_open ON inventory_anomalies (machine_id, fingerprint)
WHERE
    status = 'open';

CREATE INDEX ix_inventory_anomalies_org_status ON inventory_anomalies (organization_id, status);

CREATE INDEX ix_inventory_anomalies_machine_detected ON inventory_anomalies (machine_id, detected_at DESC);

COMMENT ON TABLE inventory_anomalies IS 'Operator-visible inventory anomaly rows; detectors upsert open fingerprints; resolve closes rows for audit trails.';

-- +goose Down
DROP TABLE IF EXISTS inventory_anomalies;
