-- DLQ Schema Migration v0.48
-- Creates two tables for the history task dead letter queue.

-- Main DLQ table.
--
-- Partition key (shard_id, domain_id, cluster_attribute_scope, cluster_attribute_name)
-- partitions the DLQ per domain and cluster attribute pair, to allow processing per-failover.
--
-- Clustering key (task_type, visibility_ts, task_id) mirrors the HistoryTaskKey
-- structure used throughout the history service:
--   - Immediate tasks (transfer, replication): visibility_ts = epoch (time.Unix(0,0)),
--     ordered primarily by task_id.
--   - Timer tasks: ordered by (visibility_ts, task_id), which matches how timer ack
--     levels are tracked in the shard (cluster_timer_ack_level stores a timestamp).
--
-- For active-passive domains (no cluster attributes), use empty string ('') for both
-- cluster_attribute_scope and cluster_attribute_name.
CREATE TABLE history_task_dlq (
    shard_id                  int,
    domain_id                 text,
    cluster_attribute_scope   text,
    cluster_attribute_name    text,
    task_type                 int,
    visibility_ts             timestamp,
    task_id                   bigint,
    workflow_id               text,
    run_id                    text,
    task_payload              blob,
    encoding_type             text,
    version                   bigint,
    created_at                timestamp,
    PRIMARY KEY ((shard_id, domain_id, cluster_attribute_scope, cluster_attribute_name), task_type, visibility_ts, task_id)
) WITH CLUSTERING ORDER BY (task_type ASC, visibility_ts ASC, task_id ASC)
    AND compaction = {
        'class': 'org.apache.cassandra.db.compaction.LeveledCompactionStrategy'
    };

-- Ack level table.
--
-- Partition key (shard_id) allows:
--   Periodic:                      SELECT * WHERE shard_id = ?
--   Domain failover:               SELECT * WHERE shard_id = ? AND domain_id = ? 
--                                  AND cluster_attribute_scope = '' AND cluster_attribute_name = ''
--   Cluster Attribute failover:    SELECT * WHERE shard_id = ? AND domain_id = ?
--                                  AND cluster_attribute_scope = ? AND cluster_attribute_name = ?
--
-- ack_level_visibility_ts + ack_level_task_id together form the composite ack key,
-- matching the (visibility_ts, task_id) pair used in HistoryTaskKey. For immediate
-- tasks, ack_level_visibility_ts is epoch.
CREATE TABLE history_task_dlq_ack_level (
    shard_id                  int,
    domain_id                 text,
    cluster_attribute_scope   text,
    cluster_attribute_name    text,
    task_type                 int,
    ack_level_visibility_ts   timestamp,
    ack_level_task_id         bigint,
    last_updated_at           timestamp,
    PRIMARY KEY ((shard_id), domain_id, cluster_attribute_scope, cluster_attribute_name, task_type)
) WITH compaction = {
    'class': 'org.apache.cassandra.db.compaction.LeveledCompactionStrategy'
};
