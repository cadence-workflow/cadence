package cassandra

// TODO(c-warren): Move this to history
const templateInsertHistoryDLQTaskRowQuery = `INSERT INTO history_task_dlq (` +
	`shard_id, domain_id, cluster_attribute_scope, cluster_attribute_name, ` +
	`task_category, visibility_ts, task_id, workflow_id, run_id, version, ` +
	`task_payload, encoding_type, created_at) ` +
	`VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// templateSelectHistoryDLQTaskRowsQuery uses CQL multi-column slice syntax (tuple comparison),
// which is valid for consecutive clustering key columns in Cassandra 3.x+.
// Bounds are [inclusive min, exclusive max).
const templateSelectHistoryDLQTaskRowsQuery = `SELECT ` +
	`shard_id, domain_id, cluster_attribute_scope, cluster_attribute_name, ` +
	`task_category, visibility_ts, task_id, task_payload, encoding_type, created_at ` +
	`FROM history_task_dlq ` +
	`WHERE shard_id = ? AND domain_id = ? AND cluster_attribute_scope = ? AND cluster_attribute_name = ? ` +
	`AND task_category = ? ` +
	`AND (visibility_ts, task_id) >= (?, ?) ` +
	`AND (visibility_ts, task_id) < (?, ?)`

const templateRangeDeleteHistoryDLQTaskRowsQuery = `DELETE FROM history_task_dlq ` +
	`WHERE shard_id = ? AND domain_id = ? AND cluster_attribute_scope = ? AND cluster_attribute_name = ? ` +
	`AND task_category = ? ` +
	`AND (visibility_ts, task_id) < (?, ?)`

const templateSelectHistoryDLQAckLevelRowsQuery = `SELECT ` +
	`domain_id, cluster_attribute_scope, cluster_attribute_name, ` +
	`task_category, ack_level_visibility_ts, ack_level_task_id, last_updated_at ` +
	`FROM history_task_dlq_ack_level ` +
	`WHERE shard_id = ?`

const templateSelectHistoryDLQAckLevelRowsByDomainQuery = `SELECT ` +
	`domain_id, cluster_attribute_scope, cluster_attribute_name, ` +
	`task_category, ack_level_visibility_ts, ack_level_task_id, last_updated_at ` +
	`FROM history_task_dlq_ack_level ` +
	`WHERE shard_id = ? AND domain_id = ?`

const templateSelectHistoryDLQAckLevelRowsByClusterAttributeQuery = `SELECT ` +
	`domain_id, cluster_attribute_scope, cluster_attribute_name, ` +
	`task_category, ack_level_visibility_ts, ack_level_task_id, last_updated_at ` +
	`FROM history_task_dlq_ack_level ` +
	`WHERE shard_id = ? AND domain_id = ? AND cluster_attribute_scope = ? AND cluster_attribute_name = ?`

const templateUpsertHistoryDLQAckLevelRowQuery = `INSERT INTO history_task_dlq_ack_level (` +
	`shard_id, domain_id, cluster_attribute_scope, cluster_attribute_name, ` +
	`task_category, ack_level_visibility_ts, ack_level_task_id, last_updated_at) ` +
	`VALUES(?, ?, ?, ?, ?, ?, ?, ?)`

// templateInitHistoryDLQAckLevelRowQuery initializes the ack-level row for a DLQ partition
// the first time a task is written to it. Uses IF NOT EXISTS so it never overwrites real progress.
const templateInitHistoryDLQAckLevelRowQuery = templateUpsertHistoryDLQAckLevelRowQuery + ` IF NOT EXISTS`
