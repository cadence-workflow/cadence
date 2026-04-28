-- Add native map column to track timer tasks associated with a workflow execution
-- for cleanup when the workflow is deleted at retention time.
-- Key: taskID (bigint), Value: visibilityTimestamp (timestamp)
ALTER TABLE executions ADD workflow_timer_tasks map<bigint, timestamp>;
