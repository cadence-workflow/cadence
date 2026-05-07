package cassandra

// TODO(c-warren): Move this to history
const templateInsertHistoryDLQTaskRowQuery = `INSERT INTO history_task_dlq (` +
	`shard_id, domain_id, cluster_attribute_scope, cluster_attribute_name, ` +
	`task_type, visibility_ts, task_id, workflow_id, run_id, version, ` +
	`task_payload, encoding_type, created_at) ` +
	`VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
