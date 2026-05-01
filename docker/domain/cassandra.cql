-- Insert into domains table
INSERT INTO domains (
    id,
    domain
) VALUES (
    123e4567-e89b-12d3-a456-426614174000, -- Replace with your UUID
    {
        id: 123e4567-e89b-12d3-a456-426614174000, -- Replace with your UUID
        name: 'default',
        status: 0, -- Registered
        description: 'This is an example domain.',
        data: {'key1': 'value1', 'key2': 'value2'},
        owner_email: 'owner@example.com'
    }
) IF NOT EXISTS;

-- Insert into domains_by_name_v2 table
INSERT INTO domains_by_name_v2 (
    domains_partition,
    name,
    domain,
    config,
    replication_config,
    is_global_domain,
    config_version,
    failover_version,
    failover_notification_version,
    notification_version
) VALUES (
    0,
    'default',
    {
        id: 123e4567-e89b-12d3-a456-426614174000, -- Replace with your UUID
        name: 'default',
        status: 0, -- Registered
        description: 'This is an example domain.',
        data: {'key1': 'value1', 'key2': 'value2'},
        owner_email: 'owner@example.com'
    },
    {
        retention: 7,
        emit_metric: True,
        history_archival_status: 0, -- Default to disabled
        visibility_archival_status: 0 -- Default to disabled
    },
    {
        active_cluster_name: 'cluster0',
        clusters: [{cluster_name: 'cluster0'}]
    },
    True, -- is_global_domain
    1,    -- config_version
    0,    -- failover_version
    0,    -- failover_notification_version
    0     -- notification_version
) IF NOT EXISTS;
