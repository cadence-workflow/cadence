-- Cassandra/history-backed async workflow queue.
-- async_workflow_queue holds async workflow requests (start / signal-with-start) awaiting execution,
-- sharded by history shard id. message_id is a monotonic per-shard sequence written by the single
-- shard owner, giving FIFO ordering without conditional writes. Domain isolation is virtual: the
-- history-side consumer schedules messages through a weighted per-domain scheduler, so there is no
-- per-queue partitioning at the storage layer.
CREATE TABLE async_workflow_queue (
  shard_id        int,
  message_id      bigint,
  message_payload blob,
  encoding        text,
  partition_key   text,
  created_time    timestamp,
  PRIMARY KEY ((shard_id), message_id)
) WITH CLUSTERING ORDER BY (message_id ASC)
  AND COMPACTION = {
    'class': 'org.apache.cassandra.db.compaction.LeveledCompactionStrategy'
  };

-- Per-shard consumption cursor (ack_level) with a version for CAS updates.
CREATE TABLE async_workflow_queue_metadata (
  shard_id          int,
  ack_level         bigint,
  version           bigint,
  created_time      timestamp,
  last_updated_time timestamp,
  PRIMARY KEY ((shard_id))
) WITH COMPACTION = {
    'class': 'org.apache.cassandra.db.compaction.LeveledCompactionStrategy'
  };

-- Dead-letter queue for poison / corrupt async workflow requests. Same shape as async_workflow_queue.
CREATE TABLE async_workflow_queue_dlq (
  shard_id        int,
  message_id      bigint,
  message_payload blob,
  encoding        text,
  partition_key   text,
  created_time    timestamp,
  PRIMARY KEY ((shard_id), message_id)
) WITH CLUSTERING ORDER BY (message_id ASC)
  AND COMPACTION = {
    'class': 'org.apache.cassandra.db.compaction.LeveledCompactionStrategy'
  };
