-- Distributed semaphore metadata: capacity and configuration for a regional counting semaphore.
-- Partitioned by domain_id so all semaphores in a domain list together; clustered by semaphore_name.
-- Source of truth for size and bucket_size (which derive N = ceil(size / bucket_size)).
CREATE TABLE semaphore_metadata (
    domain_id         uuid,
    semaphore_name    text,
    size              int,       -- total capacity (max concurrent tokens)
    bucket_size       int,       -- tokens per bucket (per-host budget); sets N = ceil(size / bucket_size)
    created_time      timestamp,
    PRIMARY KEY ((domain_id), semaphore_name)
) WITH compaction = {
    'class': 'org.apache.cassandra.db.compaction.LeveledCompactionStrategy'
};
