package cassandra

// TODO(c-warren): Move this to history
const templateInsertHistoryDLQTaskRowQuery = `INSERT INTO history_task_dlq (` +
	`shard_id, domain_id, cluster_attribute_scope, cluster_attribute_name, ` +
	`task_type, visibility_ts, task_id, ` +
	`task_payload, encoding_type, created_at) ` +
	`VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const templateSelectHistoryDLQTaskRowsQuery = `SELECT ` +
	`shard_id, domain_id, cluster_attribute_scope, cluster_attribute_name, ` +
	`task_type, visibility_ts, task_id, task_payload, encoding_type, created_at ` +
	`FROM history_task_dlq ` +
	`WHERE shard_id = ? AND domain_id = ? AND cluster_attribute_scope = ? AND cluster_attribute_name = ? ` +
	`AND task_type = ? ` +
	`AND (visibility_ts, task_id) > (?, ?) ` +
	`AND (visibility_ts, task_id) <= (?, ?)`

const templateRangeDeleteHistoryDLQTaskRowsQuery = `DELETE FROM history_task_dlq ` +
	`WHERE shard_id = ? AND domain_id = ? AND cluster_attribute_scope = ? AND cluster_attribute_name = ? ` +
	`AND task_type = ? ` +
	`AND (visibility_ts, task_id) <= (?, ?)`

const templateSelectHistoryDLQAckLevelRowsQuery = `SELECT ` +
	`domain_id, cluster_attribute_scope, cluster_attribute_name, ` +
	`task_type, ack_level_visibility_ts, ack_level_task_id, last_updated_at ` +
	`FROM history_task_dlq_ack_level ` +
	`WHERE shard_id = ?`

const templateSelectHistoryDLQAckLevelRowsByDomainQuery = `SELECT ` +
	`domain_id, cluster_attribute_scope, cluster_attribute_name, ` +
	`task_type, ack_level_visibility_ts, ack_level_task_id, last_updated_at ` +
	`FROM history_task_dlq_ack_level ` +
	`WHERE shard_id = ? AND domain_id = ?`

const templateSelectHistoryDLQAckLevelRowsByClusterAttributeQuery = `SELECT ` +
	`domain_id, cluster_attribute_scope, cluster_attribute_name, ` +
	`task_type, ack_level_visibility_ts, ack_level_task_id, last_updated_at ` +
	`FROM history_task_dlq_ack_level ` +
	`WHERE shard_id = ? AND domain_id = ? AND cluster_attribute_scope = ? AND cluster_attribute_name = ?`

const templateUpsertHistoryDLQAckLevelRowQuery = `INSERT INTO history_task_dlq_ack_level (` +
	`shard_id, domain_id, cluster_attribute_scope, cluster_attribute_name, ` +
	`task_type, ack_level_visibility_ts, ack_level_task_id, last_updated_at) ` +
	`VALUES(?, ?, ?, ?, ?, ?, ?, ?)`
