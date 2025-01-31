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
    gen_random_uuid(), -- Generate a UUID for the domain ID
    'default', -- Domain name
    '{"key1":"value1","key2":"value2"}', -- Example JSON data
    'json', -- Encoding type
    TRUE -- Set to TRUE for a global domain
) ON CONFLICT (shard_id, id) DO UPDATE
SET 
    name = EXCLUDED.name,
    data = EXCLUDED.data,
    data_encoding = EXCLUDED.data_encoding,
    is_global = EXCLUDED.is_global;

-- Insert into domain_metadata table
INSERT INTO domain_metadata (
    id,
    notification_version
) VALUES (
    1, -- Default ID
    1 -- Notification version
) ON CONFLICT (id) DO UPDATE
SET 
    notification_version = EXCLUDED.notification_version;