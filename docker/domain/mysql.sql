-- Insert into domains table
INSERT INTO domains (
    shard_id,
    id,
    name,
    data,
    data_encoding,
    is_global
) VALUES (
    54321, -- Default shard_id
    UNHEX(REPLACE(UUID(), '-', '')), -- Generate a 16-byte UUID
    'default', -- Domain name
    '{"key1":"value1","key2":"value2"}', -- Example data as JSON
    'json', -- Encoding type
    1 -- Set to 1 for a global domain
) ON DUPLICATE KEY UPDATE
    name = VALUES(name);

-- Insert into domain_metadata table
INSERT INTO domain_metadata (
    notification_version
) VALUES (
    1
) ON DUPLICATE KEY UPDATE
    notification_version = VALUES(notification_version);