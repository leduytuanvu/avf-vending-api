-- Expand machines.status CHECK for enterprise lifecycle vocabulary (additive; keeps existing values).

ALTER TABLE machines
    DROP CONSTRAINT IF EXISTS machines_status_check;

ALTER TABLE machines
    ADD CONSTRAINT machines_status_check CHECK (status IN (
        'draft',
        'provisioned',
        'active',
        'maintenance',
        'suspended',
        'retired',
        'decommissioned',
        'compromised',
        'provisioning',
        'online',
        'offline'
    ));
